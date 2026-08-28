package background

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	kuberepo "AutoOps/internal/repo/kubernetes"
	"github.com/google/uuid"
)

const (
	StatusReceived            = "received"
	StatusCollecting          = "collecting_evidence"
	StatusAnalyzing           = "analyzing"
	StatusPendingApproval     = "action_pending_approval"
	StatusApproved            = "action_approved"
	StatusIsolating           = "isolating"
	StatusWaitingHuman        = "waiting_human"
	StatusResolved            = "resolved"
	StatusFailed              = "failed"
	StatusCancelled           = "cancelled"
	StatusClassifying         = "classifying"
	StatusAutoRepairPending   = "auto_repair_pending"
	StatusAutoRepairing       = "auto_repairing"
	StatusWaitingVerification = "waiting_verification"
	StatusIncidentCreated     = "incident_created"
)

type Severity string

const (
	SeveritySimple  Severity = "simple"
	SeveritySevere  Severity = "severe"
	SeverityUnknown Severity = "unknown"
)

type Classification struct {
	Severity   Severity `json:"severity"`
	Confidence string   `json:"confidence"`
	Reason     string   `json:"reason"`
	TargetPod  string   `json:"target_pod,omitempty"`
}

type Task struct {
	ID             string          `json:"id"`
	Fingerprint    string          `json:"alert_fingerprint"`
	AlertName      string          `json:"alert_name"`
	Namespace      string          `json:"namespace"`
	Status         string          `json:"status"`
	TargetPod      string          `json:"target_pod,omitempty"`
	Analysis       string          `json:"analysis,omitempty"`
	ProposedAction json.RawMessage `json:"proposed_action,omitempty"`
	Severity       Severity        `json:"severity"`
	SeverityReason string          `json:"severity_reason,omitempty"`
	RepairStatus   string          `json:"repair_status,omitempty"`
	RepairResult   string          `json:"repair_result,omitempty"`
	IncidentID     string          `json:"incident_id,omitempty"`
	CreatedAt      time.Time       `json:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at"`
}
type Alert struct {
	Fingerprint string            `json:"fingerprint"`
	Name        string            `json:"alertname"`
	Namespace   string            `json:"namespace"`
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
}
type Service struct {
	db   *sql.DB
	kube kuberepo.KubernetesRepository
}

func NewService(db *sql.DB, kube kuberepo.KubernetesRepository) *Service {
	return &Service{db: db, kube: kube}
}
func (s *Service) CreateFromAlert(ctx context.Context, alert Alert) (Task, bool, error) {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	id := uuid.New().String()
	ns := alert.Namespace
	if ns == "" {
		ns = "autoops-test"
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO background_tasks(id,alert_fingerprint,alert_name,namespace,status,severity,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?)`, id, alert.Fingerprint, alert.Name, ns, StatusReceived, string(SeverityUnknown), now, now)
	if err != nil {
		if isUnique(err) {
			return s.getByFingerprint(ctx, alert.Fingerprint)
		}
		return Task{}, false, err
	}
	task, _, err := s.get(ctx, id)
	return task, true, err
}

// Classify deterministically decides whether an alert is eligible for automatic repair.
func (s *Service) Classify(ctx context.Context, taskID string, alert Alert, autoRepair, severe map[string]bool, maxRestarts int) (Classification, error) {
	if alert.Labels["severity"] == "critical" || severe[alert.Name] {
		return Classification{Severity: SeveritySevere, Confidence: "high", Reason: "critical or explicitly severe alert"}, s.updateClassification(ctx, taskID, SeveritySevere, "critical or explicitly severe alert", "")
	}
	if !autoRepair[alert.Name] {
		return Classification{Severity: SeverityUnknown, Confidence: "high", Reason: "alert is not in automatic repair allowlist"}, s.updateClassification(ctx, taskID, SeverityUnknown, "alert is not in automatic repair allowlist", "")
	}
	if s.kube == nil {
		return Classification{Severity: SeverityUnknown, Confidence: "high", Reason: "Kubernetes evidence unavailable"}, s.updateClassification(ctx, taskID, SeverityUnknown, "Kubernetes evidence unavailable", "")
	}
	pods, err := s.kube.ListPods(ctx, alert.Namespace)
	if err != nil {
		return Classification{Severity: SeverityUnknown, Reason: err.Error()}, s.updateClassification(ctx, taskID, SeverityUnknown, err.Error(), "")
	}
	var target string
	abnormal := 0
	for _, p := range pods {
		if !p.Ready || (maxRestarts > 0 && int(p.Restarts) >= maxRestarts) {
			abnormal++
			if target == "" {
				target = p.Name
			}
		}
	}
	if abnormal != 1 {
		reason := fmt.Sprintf("%d abnormal pods detected", abnormal)
		return Classification{Severity: SeveritySevere, Confidence: "high", Reason: reason}, s.updateClassification(ctx, taskID, SeveritySevere, reason, "")
	}
	return Classification{Severity: SeveritySimple, Confidence: "medium", Reason: "single abnormal pod matches automatic repair allowlist", TargetPod: target}, s.updateClassification(ctx, taskID, SeveritySimple, "single abnormal pod matches automatic repair allowlist", target)
}
func (s *Service) updateClassification(ctx context.Context, id string, sev Severity, reason, pod string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE background_tasks SET status=?,severity=?,severity_reason=?,target_pod=CASE WHEN ?='' THEN target_pod ELSE ? END,updated_at=? WHERE id=?`, StatusClassifying, string(sev), reason, pod, pod, time.Now().UTC().Format(time.RFC3339Nano), id)
	return err
}
func isUnique(err error) bool {
	return err != nil && (len(err.Error()) > 0 && (contains(err.Error(), "UNIQUE") || contains(err.Error(), "constraint")))
}
func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
func (s *Service) getByFingerprint(ctx context.Context, fp string) (Task, bool, error) {
	var id string
	err := s.db.QueryRowContext(ctx, `SELECT id FROM background_tasks WHERE alert_fingerprint=? AND status NOT IN ('resolved','failed','cancelled') LIMIT 1`, fp).Scan(&id)
	if err != nil {
		return Task{}, false, err
	}
	t, _, err := s.get(ctx, id)
	return t, false, err
}
func (s *Service) List(ctx context.Context) ([]Task, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,alert_fingerprint,alert_name,namespace,status,target_pod,analysis,proposed_action,severity,severity_reason,repair_status,repair_result,incident_id,created_at,updated_at FROM background_tasks ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Task{}
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}
func (s *Service) Get(ctx context.Context, id string) (Task, error) {
	t, _, err := s.get(ctx, id)
	return t, err
}
func (s *Service) get(ctx context.Context, id string) (Task, bool, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id,alert_fingerprint,alert_name,namespace,status,target_pod,analysis,proposed_action,severity,severity_reason,repair_status,repair_result,incident_id,created_at,updated_at FROM background_tasks WHERE id=?`, id)
	t, err := scanTask(row)
	return t, true, err
}

type scanner interface{ Scan(...any) error }

func scanTask(row scanner) (Task, error) {
	var t Task
	var created, updated, sev string
	var action sql.NullString
	err := row.Scan(&t.ID, &t.Fingerprint, &t.AlertName, &t.Namespace, &t.Status, &t.TargetPod, &t.Analysis, &action, &sev, &t.SeverityReason, &t.RepairStatus, &t.RepairResult, &t.IncidentID, &created, &updated)
	t.Severity = Severity(sev)
	t.ProposedAction = json.RawMessage(action.String)
	t.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
	t.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updated)
	return t, err
}
func (s *Service) Approve(ctx context.Context, id, by string) (Task, error) {
	return s.transition(ctx, id, StatusApproved, by)
}
func (s *Service) Reject(ctx context.Context, id, by string) (Task, error) {
	return s.transition(ctx, id, StatusCancelled, by)
}
func (s *Service) MarkWaitingHuman(ctx context.Context, id string) (Task, error) {
	_, err := s.db.ExecContext(ctx, `UPDATE background_tasks SET status=?,updated_at=? WHERE id=?`, StatusWaitingHuman, time.Now().UTC().Format(time.RFC3339Nano), id)
	if err != nil {
		return Task{}, err
	}
	return s.Get(ctx, id)
}

func (s *Service) SetRepair(ctx context.Context, id, status, result string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE background_tasks SET repair_status=?,repair_result=?,updated_at=? WHERE id=?`, status, result, time.Now().UTC().Format(time.RFC3339Nano), id)
	return err
}
func (s *Service) SetStatus(ctx context.Context, id, status string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE background_tasks SET status=?,updated_at=? WHERE id=?`, status, time.Now().UTC().Format(time.RFC3339Nano), id)
	return err
}
func (s *Service) SetIncident(ctx context.Context, id, incidentID string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE background_tasks SET incident_id=?,status=?,updated_at=? WHERE id=?`, incidentID, StatusIncidentCreated, time.Now().UTC().Format(time.RFC3339Nano), id)
	return err
}
func (s *Service) Timeline(ctx context.Context, id, event, status, output string) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO agent_timeline(id,owner_type,owner_id,event_type,event_name,status,output,created_at) VALUES(?,?,?,?,?,?,?,?)`, uuid.New().String(), "background_task", id, "background", event, status, output, time.Now().UTC().Format(time.RFC3339Nano))
	return err
}
func (s *Service) transition(ctx context.Context, id, status, by string) (Task, error) {
	res, err := s.db.ExecContext(ctx, `UPDATE background_tasks SET status=?,analysis=CASE WHEN ?='' THEN analysis ELSE COALESCE(analysis,'')||? END,updated_at=? WHERE id=? AND status=?`, status, by, "\nDecision by "+by, time.Now().UTC().Format(time.RFC3339Nano), id, StatusPendingApproval)
	if err != nil {
		return Task{}, err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return Task{}, fmt.Errorf("task is not awaiting approval")
	}
	return s.Get(ctx, id)
}
