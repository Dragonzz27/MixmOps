package remediation

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	kuberepo "AutoOps/internal/repo/kubernetes"
	backgroundagent "AutoOps/internal/server/ai/agent/background"
	"AutoOps/internal/server/background"
	"AutoOps/internal/server/cases"
	"AutoOps/pkg/config"
)

type Decision struct {
	Severity       string   `json:"severity"`
	Confidence     string   `json:"confidence"`
	Summary        string   `json:"summary"`
	Evidence       []string `json:"evidence"`
	ProposedAction string   `json:"proposed_action"`
	TargetPod      string   `json:"target_pod,omitempty"`
	Reason         string   `json:"reason"`
	RequiresHuman  bool     `json:"requires_human"`
}
type Evidence struct {
	Alert       background.Alert          `json:"alert"`
	Pods        []kuberepo.PodInfo        `json:"pods,omitempty"`
	Deployments []kuberepo.DeploymentInfo `json:"deployments,omitempty"`
	Events      []kuberepo.EventInfo      `json:"events,omitempty"`
	Logs        map[string]string         `json:"logs,omitempty"`
	Errors      []string                  `json:"errors,omitempty"`
}
type ActionPlan struct {
	Allowed bool   `json:"allowed"`
	Action  string `json:"action"`
	Pod     string `json:"pod,omitempty"`
	Reason  string `json:"reason"`
}

type Agent struct {
	tasks   *background.Service
	cases   *cases.Service
	kube    kuberepo.KubernetesRepository
	cfg     config.BackgroundConfig
	llm     *backgroundagent.Agent
	running sync.Map
}

func New(tasks *background.Service, cs *cases.Service, kube kuberepo.KubernetesRepository, cfg config.BackgroundConfig, llm *backgroundagent.Agent) *Agent {
	return &Agent{tasks: tasks, cases: cs, kube: kube, cfg: cfg, llm: llm}
}

func (a *Agent) Process(parent context.Context, taskID string) error {
	if _, loaded := a.running.LoadOrStore(taskID, struct{}{}); loaded {
		return nil
	}
	defer a.running.Delete(taskID)
	ctx, cancel := context.WithTimeout(parent, 60*time.Second)
	defer cancel()
	task, err := a.tasks.Get(ctx, taskID)
	if err != nil {
		return err
	}
	_ = a.tasks.SetStatus(ctx, taskID, background.StatusCollecting)
	evidence := a.collect(ctx, task)
	b, _ := json.Marshal(evidence)
	_ = a.tasks.SetEvidence(ctx, taskID, b)
	_ = a.tasks.Timeline(ctx, taskID, "evidence_collected", "success", string(b))
	_ = a.tasks.SetStatus(ctx, taskID, background.StatusAnalyzing)
	decision := a.analyze(ctx, task, string(b))
	db, _ := json.Marshal(decision)
	_ = a.tasks.SetDecision(ctx, taskID, decision.Severity, decision.Reason, decision.TargetPod, string(db))
	_ = a.tasks.SetStatus(ctx, taskID, "policy_evaluating")
	plan := a.evaluate(ctx, task, decision, evidence)
	if !plan.Allowed {
		return a.createIncident(ctx, task, evidence, decision, plan, "policy rejected automatic repair")
	}
	return a.executeAndVerify(ctx, task, plan, evidence, decision)
}
func (a *Agent) collect(ctx context.Context, t background.Task) Evidence {
	e := Evidence{Alert: background.Alert{Fingerprint: t.Fingerprint, Name: t.AlertName, Namespace: t.Namespace}, Logs: map[string]string{}}
	if a.kube == nil {
		e.Errors = append(e.Errors, "Kubernetes integration unavailable")
		return e
	}
	var err error
	e.Pods, err = a.kube.ListPods(ctx, t.Namespace)
	if err != nil {
		e.Errors = append(e.Errors, err.Error())
		return e
	}
	e.Deployments, err = a.kube.ListDeployments(ctx, t.Namespace)
	if err != nil {
		e.Errors = append(e.Errors, err.Error())
	}
	e.Events, err = a.kube.ListEvents(ctx, t.Namespace)
	if err != nil {
		e.Errors = append(e.Errors, err.Error())
	}
	for _, p := range e.Pods {
		if !p.Ready || p.Restarts >= int32(a.cfg.MaxRestarts) {
			tail := int64(200)
			if logs, x := a.kube.GetPodLogs(ctx, t.Namespace, p.Name, "", kuberepo.LogOptions{TailLines: &tail, LimitBytes: 100000}); x == nil {
				e.Logs[p.Name] = logs
			}
		}
	}
	return e
}
func (a *Agent) analyze(ctx context.Context, t background.Task, evidence string) Decision {
	fallback := Decision{Severity: "unknown", Confidence: "low", Summary: "automatic analysis unavailable", ProposedAction: "create_incident", Reason: "unable to produce a reliable remediation decision", RequiresHuman: true}
	if a.llm == nil {
		return fallback
	}
	raw, err := a.llm.Analyze(ctx, t.ID, evidence)
	if err != nil {
		return fallback
	}
	raw = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(raw, "```json"), "```"))
	var d Decision
	if json.Unmarshal([]byte(raw), &d) != nil {
		return fallback
	}
	if d.Severity != "simple" && d.Severity != "severe" && d.Severity != "unknown" {
		return fallback
	}
	if d.ProposedAction != "delete_managed_pod" && d.ProposedAction != "create_incident" {
		return fallback
	}
	return d
}
func (a *Agent) evaluate(ctx context.Context, t background.Task, d Decision, e Evidence) ActionPlan {
	deny := func(r string) ActionPlan { return ActionPlan{Action: "create_incident", Reason: r} }
	if !a.cfg.AutoRepairEnabled {
		return deny("automatic repair disabled")
	}
	if d.Severity != "simple" || d.ProposedAction != "delete_managed_pod" || d.RequiresHuman {
		return deny("agent decision requires human handling")
	}
	if contains(parseSet(a.cfg.SevereAlerts), t.AlertName) || !contains(parseSet(a.cfg.AutoRepairAlerts), t.AlertName) {
		return deny("alert is outside automatic repair policy")
	}
	if a.kube == nil || t.Namespace != a.cfg.Namespace {
		return deny("cluster evidence or namespace policy unavailable")
	}
	var bad []kuberepo.PodInfo
	for _, p := range e.Pods {
		if !p.Ready || p.Restarts >= int32(a.cfg.MaxRestarts) {
			bad = append(bad, p)
		}
	}
	if len(bad) != 1 || bad[0].Name != d.TargetPod {
		return deny("target is not the single observed abnormal pod")
	}
	actions, ok := a.kube.(kuberepo.PodActions)
	if !ok {
		return deny("pod action capability unavailable")
	}
	managed, err := actions.IsManagedPod(ctx, t.Namespace, d.TargetPod)
	if err != nil || !managed {
		return deny("target is not a managed ReplicaSet pod")
	}
	return ActionPlan{Allowed: true, Action: "delete_managed_pod", Pod: d.TargetPod, Reason: d.Reason}
}
func (a *Agent) executeAndVerify(ctx context.Context, t background.Task, p ActionPlan, e Evidence, d Decision) error {
	_ = a.tasks.SetStatus(ctx, t.ID, background.StatusAutoRepairPending)
	_ = a.tasks.SetRepair(ctx, t.ID, "pending", "")
	actions := a.kube.(kuberepo.PodActions)
	_ = a.tasks.SetStatus(ctx, t.ID, background.StatusAutoRepairing)
	if err := actions.DeleteManagedPod(ctx, t.Namespace, p.Pod); err != nil {
		_ = a.tasks.SetRepair(ctx, t.ID, "failed", err.Error())
		return a.createIncident(ctx, t, e, d, p, err.Error())
	}
	_ = a.tasks.SetRepair(ctx, t.ID, "completed", "system deleted managed Pod "+p.Pod)
	_ = a.tasks.SetStatus(ctx, t.ID, background.StatusWaitingVerification)
	_ = a.tasks.Timeline(ctx, t.ID, "repair_completed", "success", p.Pod)
	for i := 0; i < 3; i++ {
		select {
		case <-ctx.Done():
			break
		case <-time.After(5 * time.Second):
		}
		pods, err := a.kube.ListPods(ctx, t.Namespace)
		if err == nil {
			healthy := true
			for _, pod := range pods {
				if !pod.Ready {
					healthy = false
				}
			}
			if healthy {
				_ = a.tasks.SetStatus(ctx, t.ID, background.StatusResolved)
				_ = a.tasks.SetRepair(ctx, t.ID, "verified", "cluster recovered")
				_ = a.tasks.Timeline(ctx, t.ID, "verification_completed", "success", "cluster recovered")
				return nil
			}
		}
	}
	return a.createIncident(ctx, t, e, d, p, "recovery verification failed")
}
func (a *Agent) createIncident(ctx context.Context, t background.Task, e Evidence, d Decision, p ActionPlan, reason string) error {
	snapshot := map[string]any{"background_task_id": t.ID, "alert": e.Alert, "evidence": e, "remediation_decision": d, "policy_result": p, "repair_result": reason}
	i, err := a.cases.CreateIncident(ctx, t.Fingerprint, t.AlertName, snapshot)
	if err != nil {
		_ = a.tasks.SetStatus(ctx, t.ID, background.StatusFailed)
		return err
	}
	_ = a.tasks.SetIncident(ctx, t.ID, i.ID)
	_, _ = a.tasks.MarkWaitingHuman(ctx, t.ID)
	_ = a.tasks.Timeline(ctx, t.ID, "incident_created", "success", fmt.Sprintf("%s: %s", i.ID, reason))
	return nil
}
func parseSet(v string) map[string]bool {
	out := map[string]bool{}
	for _, x := range strings.Split(v, ",") {
		if x = strings.TrimSpace(x); x != "" {
			out[x] = true
		}
	}
	return out
}
func contains(m map[string]bool, v string) bool { return m[v] }
