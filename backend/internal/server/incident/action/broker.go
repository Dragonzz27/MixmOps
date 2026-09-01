package action

import (
	kuberepo "AutoOps/internal/repo/kubernetes"
	aitools "AutoOps/internal/server/ai/tools"
	"AutoOps/internal/server/cases"
	"AutoOps/pkg/config"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type ProposedAction = cases.IncidentAction
type ActionPlan struct {
	ActionType              string
	Namespace               string
	Target                  string
	Payload                 json.RawMessage
	ExpectedResourceVersion string
}
type ActionResult struct {
	Action cases.IncidentAction
	Status string
	Result string
	Error  string
}
type Broker struct {
	cases *cases.Service
	kube  kuberepo.KubernetesRepository
	cfg   *config.Config
}

func NewBroker(cs *cases.Service, k kuberepo.KubernetesRepository, cfg *config.Config) *Broker {
	return &Broker{cases: cs, kube: k, cfg: cfg}
}
func (b *Broker) Propose(ctx context.Context, incidentID string, in cases.IncidentAction) (cases.IncidentAction, error) {
	incident, e := b.cases.GetIncident(ctx, incidentID)
	if e != nil {
		return cases.IncidentAction{}, fmt.Errorf("incident not found: %w", e)
	}
	if incident.Status != "open" {
		return cases.IncidentAction{}, fmt.Errorf("incident is not open")
	}
	in.IncidentID = incidentID
	if in.Namespace == "" {
		in.Namespace = b.cfg.Kubernetes.Namespace
	}
	if in.Namespace != b.cfg.Kubernetes.Namespace {
		return cases.IncidentAction{}, fmt.Errorf("namespace outside policy")
	}
	if strings.TrimSpace(in.Target) == "" {
		return cases.IncidentAction{}, fmt.Errorf("action target is required")
	}
	if b.kube != nil {
		if obs, ok := b.kube.(kuberepo.KubernetesObservationRepository); ok {
			kind := "deployment"
			if in.ActionType == "delete_managed_pod" {
				kind = "pod"
			}
			rv, err := obs.GetResourceVersion(ctx, in.Namespace, kind, in.Target)
			if err != nil {
				return cases.IncidentAction{}, fmt.Errorf("preflight target lookup: %w", err)
			}
			in.ResourceVersion = rv
		}
	}
	if hasEvidence(inTargetContext(incident.Context)) && !targetInContext(incident.Context, in.Target) {
		return cases.IncidentAction{}, fmt.Errorf("action target is not present in incident evidence")
	}
	a, err := b.cases.ProposeIncidentAction(ctx, incidentID, in)
	if err == nil {
		_ = b.cases.AddTimeline(ctx, "incident", incidentID, "action_proposed", "success", a.ActionType+" "+a.Target, "")
	}
	return a, err
}
func (b *Broker) Confirm(ctx context.Context, incidentID, actionID, operator string) (ActionResult, error) {
	incident, e := b.cases.GetIncident(ctx, incidentID)
	if e != nil || incident.Status != "open" {
		return ActionResult{}, fmt.Errorf("incident is not open")
	}
	a, e := b.cases.GetIncidentAction(ctx, actionID)
	if e != nil {
		return ActionResult{}, e
	}
	if a.IncidentID != incidentID {
		return ActionResult{}, fmt.Errorf("action does not belong to incident")
	}
	if a.Status != "pending_confirmation" {
		return ActionResult{}, fmt.Errorf("action is not pending confirmation")
	}
	if !b.cfg.Incident.ActionsEnabled || !allowed(b.cfg.Incident.AllowedActions, a.ActionType) {
		return ActionResult{}, fmt.Errorf("action disabled by policy")
	}
	if b.kube == nil {
		return ActionResult{}, fmt.Errorf("Kubernetes unavailable")
	}
	if obs, ok := b.kube.(kuberepo.KubernetesObservationRepository); ok && a.ResourceVersion != "" {
		kind := "deployment"
		if a.ActionType == "delete_managed_pod" {
			kind = "pod"
		}
		current, err := obs.GetResourceVersion(ctx, a.Namespace, kind, a.Target)
		if err != nil {
			return ActionResult{}, fmt.Errorf("preflight target lookup: %w", err)
		}
		if current != a.ResourceVersion {
			return ActionResult{}, fmt.Errorf("resource version conflict: expected %s, current %s", a.ResourceVersion, current)
		}
	}
	a, e = b.cases.ConfirmIncidentAction(ctx, actionID, operator)
	if e != nil {
		return ActionResult{}, e
	}
	_ = b.cases.AddTimeline(ctx, "incident", incidentID, "action_confirmed", "success", "operator="+operator, "")
	if e = b.cases.SetIncidentActionResult(ctx, a.ID, "executing", "", ""); e != nil {
		return ActionResult{}, e
	}
	_ = b.cases.AddTimeline(ctx, "incident", incidentID, "action_started", "success", a.ActionType+" "+a.Target, "")
	runCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	e = execute(runCtx, b.kube, a, b.cfg.Incident.MaxReplicas)
	if e != nil {
		_ = b.cases.SetIncidentActionResult(ctx, a.ID, "failed", "", e.Error())
		_ = b.cases.AddTimeline(ctx, "incident", incidentID, "action_failed", "failed", "", e.Error())
		return ActionResult{Action: a, Status: "failed", Error: e.Error()}, e
	}
	_ = b.cases.AddTimeline(ctx, "incident", incidentID, "verification_started", "success", a.Target, "")
	if e = verify(runCtx, b.kube, a, incident.AlertName, b.cfg.GetPrometheusURL()); e != nil {
		_ = b.cases.SetIncidentActionResult(ctx, a.ID, "failed", "action executed but verification failed", e.Error())
		_ = b.cases.AddTimeline(ctx, "incident", incidentID, "verification_failed", "failed", "", e.Error())
		return ActionResult{Action: a, Status: "failed", Error: e.Error()}, e
	}
	_ = b.cases.SetIncidentActionResult(ctx, a.ID, "succeeded", "action executed", "")
	_ = b.cases.AddTimeline(ctx, "incident", incidentID, "action_succeeded", "success", "action executed", "")
	_ = b.cases.AddTimeline(ctx, "incident", incidentID, "verification_succeeded", "success", a.Target, "")
	a.Status = "succeeded"
	return ActionResult{Action: a, Status: "succeeded", Result: "action executed"}, nil
}
func (b *Broker) Reject(ctx context.Context, incidentID, actionID, operator string) error {
	a, e := b.cases.GetIncidentAction(ctx, actionID)
	if e != nil {
		return e
	}
	if a.IncidentID != incidentID {
		return fmt.Errorf("action does not belong to incident")
	}
	if _, e = b.cases.RejectIncidentAction(ctx, actionID, operator); e != nil {
		return e
	}
	_ = b.cases.AddTimeline(ctx, "incident", incidentID, "action_rejected", "success", a.ActionType+" "+a.Target, "")
	return nil
}

func inTargetContext(raw json.RawMessage) any { var v any; _ = json.Unmarshal(raw, &v); return v }
func hasEvidence(v any) bool {
	m, ok := v.(map[string]any)
	if !ok {
		return false
	}
	for _, key := range []string{"evidence", "pods", "deployments", "services", "logs"} {
		if _, exists := m[key]; exists {
			return true
		}
	}
	return false
}
func targetInContext(v any, target string) bool {
	switch x := v.(type) {
	case string:
		return x == target
	case []any:
		for _, item := range x {
			if targetInContext(item, target) {
				return true
			}
		}
	case map[string]any:
		for _, item := range x {
			if targetInContext(item, target) {
				return true
			}
		}
	}
	return false
}
func execute(ctx context.Context, k kuberepo.KubernetesRepository, a cases.IncidentAction, maxReplicas int32) error {
	switch a.ActionType {
	case "delete_managed_pod":
		actions, ok := k.(kuberepo.PodActions)
		if !ok {
			return fmt.Errorf("managed pod action capability unavailable")
		}
		managed, err := actions.IsManagedPod(ctx, a.Namespace, a.Target)
		if err != nil {
			return err
		}
		if !managed {
			return fmt.Errorf("target Pod is not managed by a ReplicaSet")
		}
		return actions.DeleteManagedPod(ctx, a.Namespace, a.Target)
	case "rollout_restart_deployment":
		actions, ok := k.(interface {
			RolloutRestartDeployment(context.Context, string, string) error
		})
		if !ok {
			return fmt.Errorf("deployment rollout capability unavailable")
		}
		return actions.RolloutRestartDeployment(ctx, a.Namespace, a.Target)
	case "rollback_deployment":
		var p struct {
			Revision int64 `json:"revision"`
		}
		if e := json.Unmarshal(a.Payload, &p); e != nil {
			return e
		}
		actions, ok := k.(interface {
			RollbackDeployment(context.Context, string, string, int64) error
		})
		if !ok {
			return fmt.Errorf("deployment rollback capability unavailable")
		}
		return actions.RollbackDeployment(ctx, a.Namespace, a.Target, p.Revision)
	case "scale_deployment":
		var p struct {
			Replicas int32 `json:"replicas"`
		}
		if e := json.Unmarshal(a.Payload, &p); e != nil {
			return e
		}
		if p.Replicas < 1 || p.Replicas > maxReplicas {
			return fmt.Errorf("replicas outside policy")
		}
		actions, ok := k.(interface {
			ScaleDeployment(context.Context, string, string, int32) error
		})
		if !ok {
			return fmt.Errorf("deployment scale capability unavailable")
		}
		return actions.ScaleDeployment(ctx, a.Namespace, a.Target, p.Replicas)
	}
	return fmt.Errorf("unsupported action %q", a.ActionType)
}

func verify(ctx context.Context, k kuberepo.KubernetesRepository, a cases.IncidentAction, alertName, prometheusURL string) error {
	for attempt := 0; attempt < 3; attempt++ {
		clusterHealthy := false
		switch a.ActionType {
		case "delete_managed_pod":
			pods, err := k.ListPods(ctx, a.Namespace)
			if err != nil {
				return err
			}
			present := false
			replacementReady := false
			for _, pod := range pods {
				if pod.Name == a.Target {
					present = true
					if pod.Ready {
						return fmt.Errorf("target Pod %s still exists after deletion", a.Target)
					}
				} else if pod.Ready {
					replacementReady = true
				}
			}
			clusterHealthy = !present && replacementReady
		case "rollout_restart_deployment", "rollback_deployment", "scale_deployment":
			deps, err := k.ListDeployments(ctx, a.Namespace)
			if err != nil {
				return err
			}
			for _, dep := range deps {
				if dep.Name == a.Target {
					clusterHealthy = dep.Ready
					break
				}
			}
		}
		if clusterHealthy && strings.TrimSpace(prometheusURL) != "" && strings.TrimSpace(alertName) != "" {
			alerts, err := aitools.QueryPrometheusAlerts(prometheusURL)
			if err == nil {
				stillFiring := false
				for _, alert := range alerts.Alerts {
					if alert.AlertName == alertName && alert.State == "firing" {
						stillFiring = true
						break
					}
				}
				if !stillFiring {
					return nil
				}
			}
		} else if clusterHealthy && (strings.TrimSpace(prometheusURL) == "" || strings.TrimSpace(alertName) == "") {
			return nil
		}
		if attempt < 2 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(5 * time.Second):
			}
		}
	}
	return fmt.Errorf("resource did not recover after action")
}
func allowed(list, action string) bool {
	for _, v := range strings.Split(list, ",") {
		if strings.TrimSpace(v) == action {
			return true
		}
	}
	return false
}
