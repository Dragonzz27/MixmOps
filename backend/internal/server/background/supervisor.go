package background

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	kuberepo "AutoOps/internal/repo/kubernetes"
	sharedchat "AutoOps/internal/server/ai/agent/sharedchat"
	"AutoOps/internal/server/ai/tools"
	"AutoOps/internal/server/cases"
	"AutoOps/pkg/config"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
)

// Supervisor owns the event-driven background workflow. Policy is deterministic;
// the LLM is never granted an unrestricted Kubernetes write capability.
type Supervisor struct {
	service       *Service
	cases         *cases.Service
	kube          kuberepo.KubernetesRepository
	cfg           config.BackgroundConfig
	prometheusURL string
	cancel        context.CancelFunc
	runner        compose.Runnable[*sharedchat.UserMessage, *schema.Message]
}

func NewSupervisor(s *Service, cs *cases.Service, kube kuberepo.KubernetesRepository, cfg config.BackgroundConfig, prometheusURL string, runners ...compose.Runnable[*sharedchat.UserMessage, *schema.Message]) *Supervisor {
	var runner compose.Runnable[*sharedchat.UserMessage, *schema.Message]
	if len(runners) > 0 {
		runner = runners[0]
	}
	return &Supervisor{service: s, cases: cs, kube: kube, cfg: cfg, prometheusURL: prometheusURL, runner: runner}
}

func (s *Supervisor) Start(ctx context.Context) {
	if !s.cfg.Enabled {
		return
	}
	interval, err := time.ParseDuration(s.cfg.PollInterval)
	if err != nil || interval <= 0 {
		interval = 30 * time.Second
	}
	child, cancel := context.WithCancel(ctx)
	s.cancel = cancel
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		s.poll(child)
		for {
			select {
			case <-ticker.C:
				s.poll(child)
			case <-child.Done():
				return
			}
		}
	}()
}
func (s *Supervisor) Stop() {
	if s.cancel != nil {
		s.cancel()
	}
}
func (s *Supervisor) poll(ctx context.Context) {
	if strings.TrimSpace(s.prometheusURL) == "" {
		return
	}
	alerts, err := tools.QueryPrometheusAlerts(s.prometheusURL)
	if err != nil {
		log.Printf("background prometheus poll failed: %v", err)
		return
	}
	for _, a := range alerts.Alerts {
		name := a.AlertName
		ns := s.cfg.Namespace
		if ns == "" {
			ns = "autoops-test"
		}
		_, _, _ = s.HandleAlert(ctx, Alert{Fingerprint: a.Fingerprint, Name: name, Namespace: ns, Labels: map[string]string{"alertname": name, "severity": a.Severity}, Annotations: map[string]string{"description": a.Description}})
	}
}
func (s *Supervisor) HandleAlert(ctx context.Context, alert Alert) (Task, bool, error) {
	if alert.Namespace == "" {
		alert.Namespace = s.cfg.Namespace
	}
	if alert.Namespace == "" {
		alert.Namespace = "autoops-test"
	}
	if strings.TrimSpace(alert.Fingerprint) == "" {
		alert.Fingerprint = alert.Name + "|" + alert.Namespace
	}
	task, created, err := s.service.CreateFromAlert(ctx, alert)
	if err != nil || !created {
		return task, created, err
	}
	_ = s.service.SetStatus(ctx, task.ID, StatusCollecting)
	_ = s.service.Timeline(ctx, task.ID, "evidence_collected", "success", "Prometheus and Kubernetes evidence collection started")
	auto, severe := parseSet(s.cfg.AutoRepairAlerts), parseSet(s.cfg.SevereAlerts)
	max := s.cfg.MaxRestarts
	class, err := s.service.Classify(ctx, task.ID, alert, auto, severe, max)
	if err != nil {
		return task, true, err
	}
	_ = s.service.Timeline(ctx, task.ID, "severity_classified", "success", class.Reason)
	if class.Severity == SeveritySimple && s.cfg.AutoRepairEnabled {
		if s.kube == nil {
			return s.toIncident(ctx, task, class.Reason)
		}
		actions, ok := s.kube.(kuberepo.PodActions)
		if !ok {
			return s.toIncident(ctx, task, "Pod action capability unavailable")
		}
		_ = s.service.SetStatus(ctx, task.ID, StatusAutoRepairPending)
		_ = s.service.SetRepair(ctx, task.ID, "pending", "")
		if err := actions.DeleteManagedPod(ctx, alert.Namespace, class.TargetPod); err != nil {
			_ = s.service.SetRepair(ctx, task.ID, "failed", err.Error())
			return s.toIncident(ctx, task, err.Error())
		}
		_ = s.service.SetStatus(ctx, task.ID, StatusWaitingVerification)
		_ = s.service.SetRepair(ctx, task.ID, "completed", "managed Pod deleted; verification pending")
		_ = s.service.Timeline(ctx, task.ID, "repair_completed", "success", fmt.Sprintf("deleted Pod %s", class.TargetPod))
		return s.verify(ctx, task.ID, alert.Namespace), true, nil
	}
	_ = s.service.SetStatus(ctx, task.ID, StatusAnalyzing)
	_ = s.service.Timeline(ctx, task.ID, "analysis_completed", "success", class.Reason)
	return s.toIncident(ctx, task, class.Reason)
}
func (s *Supervisor) verify(ctx context.Context, id, ns string) Task {
	for i := 0; i < 3; i++ {
		time.Sleep(5 * time.Second)
		if s.kube != nil {
			if pods, err := s.kube.ListPods(ctx, ns); err == nil {
				healthy := true
				for _, p := range pods {
					if !p.Ready {
						healthy = false
					}
				}
				if healthy {
					_ = s.service.SetStatus(ctx, id, StatusResolved)
					_ = s.service.SetRepair(ctx, id, "verified", "cluster recovered")
					break
				}
			}
		}
	}
	t, _ := s.service.Get(ctx, id)
	return t
}
func (s *Supervisor) toIncident(ctx context.Context, task Task, reason string) (Task, bool, error) {
	if s.cases == nil {
		_ = s.service.SetStatus(ctx, task.ID, StatusWaitingHuman)
		t, err := s.service.Get(ctx, task.ID)
		return t, true, err
	}
	snapshot := map[string]any{"alert_name": task.AlertName, "namespace": task.Namespace, "severity": task.Severity, "reason": reason, "background_task_id": task.ID}
	if s.kube != nil {
		// Capture a bounded, read-only evidence snapshot before handing off to a human.
		if pods, e := s.kube.ListPods(ctx, task.Namespace); e == nil {
			snapshot["pods"] = pods
		}
		if deps, e := s.kube.ListDeployments(ctx, task.Namespace); e == nil {
			snapshot["deployments"] = deps
		}
		if events, e := s.kube.ListEvents(ctx, task.Namespace); e == nil {
			snapshot["events"] = events
		}
	}
	if s.runner != nil {
		if out, e := s.runner.Invoke(ctx, &sharedchat.UserMessage{ID: "background:" + task.ID, Query: "请基于以下后台任务证据给出保守的故障分析，禁止执行写操作：" + fmt.Sprint(snapshot)}); e == nil {
			snapshot["agent_analysis"] = out.Content
		}
	}
	i, err := s.cases.CreateIncident(ctx, task.Fingerprint, task.AlertName, snapshot)
	if err != nil {
		_ = s.service.SetStatus(ctx, task.ID, StatusFailed)
		return task, true, err
	}
	_ = s.service.SetIncident(ctx, task.ID, i.ID)
	_ = s.service.Timeline(ctx, task.ID, "incident_created", "success", i.ID)
	t, err := s.service.Get(ctx, task.ID)
	return t, true, err
}
func parseSet(value string) map[string]bool {
	out := map[string]bool{}
	for _, v := range strings.Split(value, ",") {
		if v = strings.TrimSpace(v); v != "" {
			out[v] = true
		}
	}
	return out
}
