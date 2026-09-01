package cases

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type Incident struct {
	ID          string          `json:"id"`
	Fingerprint string          `json:"alert_fingerprint"`
	AlertName   string          `json:"alert_name"`
	Status      string          `json:"status"`
	Context     json.RawMessage `json:"context"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
}
type Message struct {
	ID        string    `json:"id"`
	OwnerType string    `json:"owner_type"`
	OwnerID   string    `json:"owner_id"`
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}
type IncidentAction struct {
	ID              string          `json:"id"`
	IncidentID      string          `json:"incident_id"`
	ActionType      string          `json:"action_type"`
	Namespace       string          `json:"namespace"`
	Target          string          `json:"target"`
	ResourceVersion string          `json:"resource_version,omitempty"`
	Payload         json.RawMessage `json:"payload"`
	Reason          string          `json:"reason"`
	Risk            string          `json:"risk"`
	Impact          string          `json:"impact"`
	Rollback        string          `json:"rollback"`
	Verification    string          `json:"verification"`
	Status          string          `json:"status"`
	ConfirmedBy     string          `json:"confirmed_by,omitempty"`
	Result          string          `json:"result,omitempty"`
	Error           string          `json:"error,omitempty"`
	CreatedAt       time.Time       `json:"created_at"`
}
type Service struct{ db *sql.DB }

func NewService(db *sql.DB) *Service { return &Service{db: db} }
func (s *Service) CreateIncident(ctx context.Context, fp, name string, snapshot any) (Incident, error) {
	id := uuid.New().String()
	now := time.Now().UTC()
	b, _ := json.Marshal(snapshot)
	_, err := s.db.ExecContext(ctx, `INSERT INTO incidents(id,alert_fingerprint,alert_name,status,context_snapshot,created_at,updated_at) VALUES(?,?,?,?,?,?,?)`, id, fp, name, "open", string(b), now.Format(time.RFC3339Nano), now.Format(time.RFC3339Nano))
	return Incident{ID: id, Fingerprint: fp, AlertName: name, Status: "open", Context: b, CreatedAt: now, UpdatedAt: now}, err
}
func (s *Service) ListIncidents(ctx context.Context) ([]Incident, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,alert_fingerprint,alert_name,status,context_snapshot,created_at,updated_at FROM incidents ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Incident{}
	for rows.Next() {
		var i Incident
		var raw, c, u string
		if err := rows.Scan(&i.ID, &i.Fingerprint, &i.AlertName, &i.Status, &raw, &c, &u); err != nil {
			return nil, err
		}
		i.Context = json.RawMessage(raw)
		i.CreatedAt, _ = time.Parse(time.RFC3339Nano, c)
		i.UpdatedAt, _ = time.Parse(time.RFC3339Nano, u)
		out = append(out, i)
	}
	return out, rows.Err()
}
func (s *Service) GetIncident(ctx context.Context, id string) (Incident, error) {
	var i Incident
	var raw, c, u string
	err := s.db.QueryRowContext(ctx, `SELECT id,alert_fingerprint,alert_name,status,context_snapshot,created_at,updated_at FROM incidents WHERE id=?`, id).Scan(&i.ID, &i.Fingerprint, &i.AlertName, &i.Status, &raw, &c, &u)
	i.Context = json.RawMessage(raw)
	i.CreatedAt, _ = time.Parse(time.RFC3339Nano, c)
	i.UpdatedAt, _ = time.Parse(time.RFC3339Nano, u)
	return i, err
}

func (s *Service) AddMessage(ctx context.Context, owner, id, role, content string) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO agent_messages(id,owner_type,owner_id,role,content,created_at) VALUES(?,?,?,?,?,?)`, uuid.New().String(), owner, id, role, content, time.Now().UTC().Format(time.RFC3339Nano))
	return err
}
func (s *Service) Messages(ctx context.Context, owner, id string) ([]Message, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,owner_type,owner_id,role,content,created_at FROM agent_messages WHERE owner_type=? AND owner_id=? ORDER BY created_at,id`, owner, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Message{}
	for rows.Next() {
		var m Message
		var at string
		if err := rows.Scan(&m.ID, &m.OwnerType, &m.OwnerID, &m.Role, &m.Content, &at); err != nil {
			return nil, err
		}
		m.CreatedAt, _ = time.Parse(time.RFC3339Nano, at)
		out = append(out, m)
	}
	return out, rows.Err()
}
func (s *Service) ResolveIncident(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `UPDATE incidents SET status='resolved',resolved_at=?,updated_at=? WHERE id=?`, time.Now().UTC().Format(time.RFC3339Nano), time.Now().UTC().Format(time.RFC3339Nano), id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

var validIncidentActions = map[string]bool{"delete_managed_pod": true, "rollout_restart_deployment": true, "rollback_deployment": true, "scale_deployment": true}

func (s *Service) ProposeIncidentAction(ctx context.Context, incidentID string, a IncidentAction) (IncidentAction, error) {
	if !validIncidentActions[a.ActionType] {
		return a, fmt.Errorf("unsupported incident action %q", a.ActionType)
	}
	if a.Namespace == "" {
		return a, fmt.Errorf("namespace is required")
	}
	a.ID = uuid.New().String()
	a.IncidentID = incidentID
	a.Status = "pending_confirmation"
	a.CreatedAt = time.Now().UTC()
	if len(a.Payload) == 0 {
		a.Payload = json.RawMessage(`{}`)
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO incident_actions(id,incident_id,action_type,namespace,target,resource_version,payload,reason,impact,risk,rollback,verification,status,created_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, a.ID, a.IncidentID, a.ActionType, a.Namespace, a.Target, a.ResourceVersion, string(a.Payload), a.Reason, a.Impact, a.Risk, a.Rollback, a.Verification, a.Status, a.CreatedAt.Format(time.RFC3339Nano))
	return a, err
}
func (s *Service) GetIncidentAction(ctx context.Context, id string) (IncidentAction, error) {
	var a IncidentAction
	var payload, created string
	err := s.db.QueryRowContext(ctx, `SELECT id,incident_id,action_type,namespace,target,resource_version,payload,reason,impact,risk,rollback,verification,status,confirmed_by,result,error,created_at FROM incident_actions WHERE id=?`, id).Scan(&a.ID, &a.IncidentID, &a.ActionType, &a.Namespace, &a.Target, &a.ResourceVersion, &payload, &a.Reason, &a.Impact, &a.Risk, &a.Rollback, &a.Verification, &a.Status, &a.ConfirmedBy, &a.Result, &a.Error, &created)
	a.Payload = json.RawMessage(payload)
	a.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
	return a, err
}
func (s *Service) ConfirmIncidentAction(ctx context.Context, id, by string) (IncidentAction, error) {
	res, err := s.db.ExecContext(ctx, `UPDATE incident_actions SET status='confirmed',confirmed_by=?,confirmed_at=? WHERE id=? AND status='pending_confirmation'`, by, time.Now().UTC().Format(time.RFC3339Nano), id)
	if err != nil {
		return IncidentAction{}, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return IncidentAction{}, fmt.Errorf("action is not pending confirmation")
	}
	return s.GetIncidentAction(ctx, id)
}
func (s *Service) RejectIncidentAction(ctx context.Context, id, by string) (IncidentAction, error) {
	res, err := s.db.ExecContext(ctx, `UPDATE incident_actions SET status='rejected',confirmed_by=?,confirmed_at=? WHERE id=? AND status='pending_confirmation'`, by, time.Now().UTC().Format(time.RFC3339Nano), id)
	if err != nil {
		return IncidentAction{}, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return IncidentAction{}, fmt.Errorf("action is not pending confirmation")
	}
	return s.GetIncidentAction(ctx, id)
}

// AddTimeline records coordinator, worker and action-broker events in the
// same durable audit stream used by background tasks.
func (s *Service) AddTimeline(ctx context.Context, ownerType, ownerID, eventName, status, output, eventErr string) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO agent_timeline(id,owner_type,owner_id,event_type,event_name,status,output,error,created_at) VALUES(?,?,?,?,?,?,?,?,?)`, uuid.New().String(), ownerType, ownerID, "agent", eventName, status, output, eventErr, time.Now().UTC().Format(time.RFC3339Nano))
	return err
}
func (s *Service) SetIncidentActionResult(ctx context.Context, id, status, result, actionErr string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE incident_actions SET status=?,result=?,error=?,executed_at=? WHERE id=? AND status IN ('confirmed','executing')`, status, result, actionErr, time.Now().UTC().Format(time.RFC3339Nano), id)
	return err
}
func (s *Service) ListIncidentActions(ctx context.Context, incidentID string) ([]IncidentAction, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,incident_id,action_type,namespace,target,resource_version,payload,reason,impact,risk,rollback,verification,status,confirmed_by,result,error,created_at FROM incident_actions WHERE incident_id=? ORDER BY created_at`, incidentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []IncidentAction{}
	for rows.Next() {
		var a IncidentAction
		var payload, created string
		if err := rows.Scan(&a.ID, &a.IncidentID, &a.ActionType, &a.Namespace, &a.Target, &a.ResourceVersion, &payload, &a.Reason, &a.Impact, &a.Risk, &a.Rollback, &a.Verification, &a.Status, &a.ConfirmedBy, &a.Result, &a.Error, &created); err != nil {
			return nil, err
		}
		a.Payload = json.RawMessage(payload)
		a.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
		out = append(out, a)
	}
	return out, rows.Err()
}
