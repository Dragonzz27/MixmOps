package supervisor

import (
	"context"
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
type Supervisor struct {
	tasks         *background.Service
	processor     Processor
	cfg           config.BackgroundConfig
	prometheusURL string
	cancel        context.CancelFunc
	mu            sync.RWMutex
	lastPoll      time.Time
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
		workCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		if e := s.processor.Process(workCtx, taskID); e != nil {
			_ = s.tasks.SetStatus(context.Background(), taskID, background.StatusFailed)
			_ = s.tasks.Timeline(context.Background(), taskID, "remediation_failed", "failed", e.Error())
		}
	}()
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
	for _, a := range result.Alerts {
		_, _, _ = s.HandleAlert(ctx, background.Alert{Fingerprint: a.Fingerprint, Name: a.AlertName, Namespace: s.cfg.Namespace, Labels: map[string]string{"alertname": a.AlertName, "severity": a.Severity}, Annotations: map[string]string{"description": a.Description}})
	}
}
func (s *Supervisor) Status() map[string]any {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return map[string]any{"enabled": s.cfg.Enabled, "last_poll_at": s.lastPoll, "last_error": s.lastError}
}
