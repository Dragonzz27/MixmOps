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

type Task struct {
	ID                  string          `json:"id"`
	Fingerprint         string          `json:"alert_fingerprint"`
	AlertName           string          `json:"alert_name"`
	AlertSeverity       string          `json:"alert_severity,omitempty"`
	Namespace           string          `json:"namespace"`
	Status              string          `json:"status"`
	TargetPod           string          `json:"target_pod,omitempty"`
	Analysis            string          `json:"analysis,omitempty"`
	ProposedAction      json.RawMessage `json:"proposed_action,omitempty"`
	Severity            Severity        `json:"severity"`
	SeverityReason      string          `json:"severity_reason,omitempty"`
	RepairStatus        string          `json:"repair_status,omitempty"`
	RepairResult        string          `json:"repair_result,omitempty"`
	IncidentID          string          `json:"incident_id,omitempty"`
	CurrentRound        int             `json:"current_round"`
	MaxRounds           int             `json:"max_rounds"`
	DecisionSnapshot    json.RawMessage `json:"decision,omitempty"`
	PlanSnapshot        json.RawMessage `json:"plan,omitempty"`
	ObservationSnapshot json.RawMessage `json:"observation,omitempty"`
	CreatedAt           time.Time       `json:"created_at"`
	UpdatedAt           time.Time       `json:"updated_at"`
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
	_, err := s.db.ExecContext(ctx, `INSERT INTO background_tasks(id,alert_fingerprint,alert_name,namespace,alert_severity,status,severity,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?)`, id, alert.Fingerprint, alert.Name, ns, alert.Labels["severity"], StatusReceived, string(SeverityUnknown), now, now)
	if err != nil {
		if isUnique(err) {
			return s.getByFingerprint(ctx, alert.Fingerprint)
		}
		return Task{}, false, err
	}
	task, _, err := s.get(ctx, id)
	return task, true, err
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
	rows, err := s.db.QueryContext(ctx, `SELECT id,alert_fingerprint,alert_name,namespace,alert_severity,status,target_pod,analysis,proposed_action,severity,severity_reason,repair_status,repair_result,incident_id,current_round,max_rounds,decision_snapshot,plan_snapshot,observation_snapshot,created_at,updated_at FROM background_tasks ORDER BY created_at DESC`)
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
	row := s.db.QueryRowContext(ctx, `SELECT id,alert_fingerprint,alert_name,namespace,alert_severity,status,target_pod,analysis,proposed_action,severity,severity_reason,repair_status,repair_result,incident_id,current_round,max_rounds,decision_snapshot,plan_snapshot,observation_snapshot,created_at,updated_at FROM background_tasks WHERE id=?`, id)
	t, err := scanTask(row)
	return t, true, err
}

type scanner interface{ Scan(...any) error }

func scanTask(row scanner) (Task, error) {
	var t Task
	var created, updated sql.NullString
	var fingerprint, alertName, namespace, status sql.NullString
	var alertSeverity, targetPod, analysis, sev, severityReason, repairStatus, repairResult, incidentID sql.NullString
	var currentRound, maxRounds sql.NullInt64
	var decision, plan, observation sql.NullString
	var action sql.NullString
	err := row.Scan(&t.ID, &fingerprint, &alertName, &namespace, &alertSeverity, &status, &targetPod, &analysis, &action, &sev, &severityReason, &repairStatus, &repairResult, &incidentID, &currentRound, &maxRounds, &decision, &plan, &observation, &created, &updated)
	t.Fingerprint = fingerprint.String
	t.AlertName = alertName.String
	t.Namespace = namespace.String
	t.Status = status.String
	t.AlertSeverity = alertSeverity.String
	t.TargetPod = targetPod.String
	t.Analysis = analysis.String
	t.Severity = Severity(sev.String)
	t.SeverityReason = severityReason.String
	t.RepairStatus = repairStatus.String
	t.RepairResult = repairResult.String
	t.IncidentID = incidentID.String
	t.CurrentRound = int(currentRound.Int64)
	t.MaxRounds = int(maxRounds.Int64)
	if t.MaxRounds == 0 {
		t.MaxRounds = 3
	}
	t.DecisionSnapshot = json.RawMessage(decision.String)
	t.PlanSnapshot = json.RawMessage(plan.String)
	t.ObservationSnapshot = json.RawMessage(observation.String)
	t.ProposedAction = json.RawMessage(action.String)
	t.CreatedAt, _ = time.Parse(time.RFC3339Nano, created.String)
	t.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updated.String)
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
func (s *Service) SetEvidence(ctx context.Context, id string, evidence []byte) error {
	_, err := s.db.ExecContext(ctx, `UPDATE background_tasks SET evidence_snapshot=?,updated_at=? WHERE id=?`, string(evidence), time.Now().UTC().Format(time.RFC3339Nano), id)
	return err
}
func (s *Service) SetDecision(ctx context.Context, id, severity, reason, pod, decision string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE background_tasks SET severity=?,severity_reason=?,target_pod=?,analysis=?,proposed_action=?,decision_snapshot=?,updated_at=? WHERE id=?`, severity, reason, pod, decision, decision, decision, time.Now().UTC().Format(time.RFC3339Nano), id)
	return err
}
func (s *Service) SetRound(ctx context.Context, id string, round int) error {
	_, err := s.db.ExecContext(ctx, `UPDATE background_tasks SET current_round=?,updated_at=? WHERE id=?`, round, time.Now().UTC().Format(time.RFC3339Nano), id)
	return err
}
func (s *Service) SetPlan(ctx context.Context, id string, plan []byte) error {
	_, err := s.db.ExecContext(ctx, `UPDATE background_tasks SET plan_snapshot=?,updated_at=? WHERE id=?`, string(plan), time.Now().UTC().Format(time.RFC3339Nano), id)
	return err
}
func (s *Service) SetObservation(ctx context.Context, id string, obs []byte) error {
	_, err := s.db.ExecContext(ctx, `UPDATE background_tasks SET observation_snapshot=?,updated_at=? WHERE id=?`, string(obs), time.Now().UTC().Format(time.RFC3339Nano), id)
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
func (s *Service) TimelineList(ctx context.Context, id string) ([]map[string]any, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT event_type,event_name,status,output,error,created_at FROM agent_timeline WHERE owner_type='background_task' AND owner_id=? ORDER BY created_at`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var typ, name, status, output, e, created string
		if err := rows.Scan(&typ, &name, &status, &output, &e, &created); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{"event_type": typ, "event_name": name, "status": status, "output": output, "error": e, "created_at": created})
	}
	return out, rows.Err()
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
