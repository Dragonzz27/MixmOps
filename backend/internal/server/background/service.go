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
	StatusReceived        = "received"
	StatusCollecting      = "collecting_evidence"
	StatusAnalyzing       = "analyzing"
	StatusPendingApproval = "action_pending_approval"
	StatusApproved        = "action_approved"
	StatusIsolating       = "isolating"
	StatusWaitingHuman    = "waiting_human"
	StatusResolved        = "resolved"
	StatusFailed          = "failed"
	StatusCancelled       = "cancelled"
)

type Task struct {
	ID             string          `json:"id"`
	Fingerprint    string          `json:"alert_fingerprint"`
	AlertName      string          `json:"alert_name"`
	Namespace      string          `json:"namespace"`
	Status         string          `json:"status"`
	TargetPod      string          `json:"target_pod,omitempty"`
	Analysis       string          `json:"analysis,omitempty"`
	ProposedAction json.RawMessage `json:"proposed_action,omitempty"`
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
	_, err := s.db.ExecContext(ctx, `INSERT INTO background_tasks(id,alert_fingerprint,alert_name,namespace,status,created_at,updated_at) VALUES(?,?,?,?,?,?,?)`, id, alert.Fingerprint, alert.Name, ns, StatusReceived, now, now)
	if err != nil {
		if isUnique(err) {
			return s.getByFingerprint(ctx, alert.Fingerprint)
		}
		return Task{}, false, err
	}
	if s.kube != nil {
		pods, listErr := s.kube.ListPods(ctx, ns)
		if listErr == nil {
			for _, pod := range pods {
				if !pod.Ready || pod.Restarts > 0 {
					action, _ := json.Marshal(map[string]any{"type": "delete_pod", "pod": pod.Name, "reason": "Pod is unhealthy or restarting; approval required"})
					_, _ = s.db.ExecContext(ctx, `UPDATE background_tasks SET status=?,target_pod=?,analysis=?,proposed_action=?,updated_at=? WHERE id=?`, StatusPendingApproval, pod.Name, "Kubernetes evidence collected; a conservative Pod isolation action is proposed.", string(action), time.Now().UTC().Format(time.RFC3339Nano), id)
					break
				}
			}
		}
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
	rows, err := s.db.QueryContext(ctx, `SELECT id,alert_fingerprint,alert_name,namespace,status,target_pod,analysis,proposed_action,created_at,updated_at FROM background_tasks ORDER BY created_at DESC`)
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
	row := s.db.QueryRowContext(ctx, `SELECT id,alert_fingerprint,alert_name,namespace,status,target_pod,analysis,proposed_action,created_at,updated_at FROM background_tasks WHERE id=?`, id)
	t, err := scanTask(row)
	return t, true, err
}

type scanner interface{ Scan(...any) error }

func scanTask(row scanner) (Task, error) {
	var t Task
	var created, updated string
	var action sql.NullString
	err := row.Scan(&t.ID, &t.Fingerprint, &t.AlertName, &t.Namespace, &t.Status, &t.TargetPod, &t.Analysis, &action, &created, &updated)
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
