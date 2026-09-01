package supervisor

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"AutoOps/internal/server/ai/tools"
	"AutoOps/internal/server/background"
	"AutoOps/pkg/config"
)

// Processor is deliberately small: the supervisor only schedules work and
// never performs classification, LLM reasoning, repair, or incident handoff.
type Processor interface {
	Process(context.Context, string) error
}

// FailureHandler is optional. Remediation implementations can use it to
// hand a failed workflow to Incident instead of leaving a task as a generic
// failed record. Keeping this optional preserves the small Supervisor
// dependency and makes it straightforward to test with a fake Processor.
type FailureHandler interface {
	Handoff(context.Context, string, string) error
}
type Supervisor struct {
	tasks         *background.Service
	processor     Processor
	cfg           config.BackgroundConfig
	prometheusURL string
	cancel        context.CancelFunc
	mu            sync.RWMutex
	lastPoll      time.Time
	lastSuccess   time.Time
	lastError     string
}

func New(tasks *background.Service, processor Processor, cfg config.BackgroundConfig, prometheusURL string) *Supervisor {
	return &Supervisor{tasks: tasks, processor: processor, cfg: cfg, prometheusURL: prometheusURL}
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
	// Recover tasks that were in-flight before a process restart before the
	// first Prometheus poll. Reconcile is intentionally idempotent; the
	// remediation agent owns the per-task execution lock.
	_ = s.Reconcile(child)
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
func (s *Supervisor) HandleAlert(ctx context.Context, alert background.Alert) (background.Task, bool, error) {
	if alert.Namespace == "" {
		alert.Namespace = s.cfg.Namespace
	}
	if alert.Namespace == "" {
		alert.Namespace = "autoops-test"
	}
	if strings.TrimSpace(alert.Fingerprint) == "" {
		alert.Fingerprint = alert.Name + "|" + alert.Namespace
	}
	task, created, err := s.tasks.CreateFromAlert(ctx, alert)
	if err != nil || !created {
		return task, created, err
	}
	_ = s.tasks.Timeline(ctx, task.ID, "alert_received", "success", "task created by background supervisor")
	s.Schedule(task.ID)
	return task, true, nil
}
func (s *Supervisor) Schedule(taskID string) {
	if s.processor == nil {
		return
	}
	go func() {
		workCtx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
		defer cancel()
		if e := s.processor.Process(workCtx, taskID); e != nil {
			bgctx := context.Background()
			if h, ok := s.processor.(FailureHandler); ok {
				if handoffErr := h.Handoff(bgctx, taskID, e.Error()); handoffErr == nil {
					return
				}
			}
			_ = s.tasks.SetStatus(bgctx, taskID, background.StatusFailed)
			_ = s.tasks.Timeline(bgctx, taskID, "remediation_failed", "failed", e.Error())
		}
	}()
}

// CreateIncident exposes an idempotent manual handoff for operators when an
// automatic handoff failed or a task needs escalation from the task page.
func (s *Supervisor) CreateIncident(ctx context.Context, taskID, reason string) error {
	h, ok := s.processor.(FailureHandler)
	if !ok {
		return fmt.Errorf("remediation processor does not support incident handoff")
	}
	return h.Handoff(ctx, taskID, reason)
}

// Kind and Reconcile implement operations.Reconciler. The Supervisor only
// discovers unfinished tasks and schedules them; it never performs workflow
// work itself.
func (s *Supervisor) Kind() string { return "background_supervisor" }

func (s *Supervisor) Reconcile(ctx context.Context) error {
	if s.tasks == nil || s.processor == nil {
		return nil
	}
	tasks, err := s.tasks.List(ctx)
	if err != nil {
		return err
	}
	for _, task := range tasks {
		switch task.Status {
		case background.StatusReceived, background.StatusCollecting,
			background.StatusAnalyzing, background.StatusClassifying,
			"policy_evaluating", background.StatusAutoRepairPending,
			background.StatusAutoRepairing, background.StatusWaitingVerification:
			s.Schedule(task.ID)
		}
	}
	return nil
}
func (s *Supervisor) poll(ctx context.Context) {
	s.mu.Lock()
	s.lastPoll = time.Now().UTC()
	s.mu.Unlock()
	if strings.TrimSpace(s.prometheusURL) == "" {
		return
	}
	result, err := tools.QueryPrometheusAlerts(s.prometheusURL)
	if err != nil {
		s.mu.Lock()
		s.lastError = err.Error()
		s.mu.Unlock()
		log.Printf("background prometheus poll failed: %v", err)
		return
	}
	s.mu.Lock()
	s.lastSuccess = time.Now().UTC()
	s.lastError = ""
	s.mu.Unlock()
	for _, a := range result.Alerts {
		_, _, _ = s.HandleAlert(ctx, background.Alert{Fingerprint: a.Fingerprint, Name: a.AlertName, Namespace: s.cfg.Namespace, Labels: map[string]string{"alertname": a.AlertName, "severity": a.Severity}, Annotations: map[string]string{"description": a.Description}})
	}
}
func (s *Supervisor) Status() map[string]any {
	s.mu.RLock()
	lastPoll, lastSuccess, lastError := s.lastPoll, s.lastSuccess, s.lastError
	s.mu.RUnlock()
	active := 0
	if s.tasks != nil {
		if tasks, err := s.tasks.List(context.Background()); err == nil {
			for _, task := range tasks {
				switch task.Status {
				case background.StatusReceived, background.StatusCollecting, background.StatusAnalyzing,
					background.StatusClassifying, "policy_evaluating", background.StatusAutoRepairPending,
					background.StatusAutoRepairing, background.StatusWaitingVerification:
					active++
				}
			}
		}
	}
	return map[string]any{"enabled": s.cfg.Enabled, "last_poll_at": lastPoll, "last_success_at": lastSuccess, "last_error": lastError, "active_tasks": active}
}
