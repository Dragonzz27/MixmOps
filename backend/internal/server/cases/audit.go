package cases

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

func (s *Service) StartAgentRun(ctx context.Context, ownerType, ownerID, agentName, input string) (string, error) {
	id := uuid.NewString()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := s.db.ExecContext(ctx, `INSERT INTO agent_runs(id,owner_type,owner_id,agent_name,status,input_snapshot,started_at) VALUES(?,?,?,?,?,?,?)`, id, ownerType, ownerID, agentName, "running", input, now)
	return id, err
}

func (s *Service) FinishAgentRun(ctx context.Context, runID, status, output string, runErr error) error {
	errText := ""
	if runErr != nil {
		errText = runErr.Error()
	}
	_, err := s.db.ExecContext(ctx, `UPDATE agent_runs SET status=?,output_snapshot=?,error=?,ended_at=? WHERE id=?`, status, output, errText, time.Now().UTC().Format(time.RFC3339Nano), runID)
	return err
}

func (s *Service) UpsertWorkerEvent(ctx context.Context, runID string, event runtimeWorkerEvent) error {
	if runID == "" || event.ID == "" {
		return fmt.Errorf("run id and worker id are required")
	}
	prompt := event.Prompt
	result := event.Result
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if event.Status == "running" {
		_, err := s.db.ExecContext(ctx, `INSERT OR IGNORE INTO agent_workers(id,run_id,agent_name,prompt,status,started_at) VALUES(?,?,?,?,?,?)`, event.ID, runID, event.AgentName, prompt, event.Status, now)
		return err
	}
	_, err := s.db.ExecContext(ctx, `UPDATE agent_workers SET status=?,result=?,error=?,ended_at=? WHERE id=? AND run_id=?`, event.Status, result, event.Error, now, event.ID, runID)
	return err
}

func (s *Service) UpsertToolEvent(ctx context.Context, runID string, event runtimeToolEvent) error {
	if runID == "" || event.ID == "" {
		return fmt.Errorf("run id and tool call id are required")
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if event.Status == "running" {
		_, err := s.db.ExecContext(ctx, `INSERT OR IGNORE INTO agent_tool_calls(id,run_id,worker_id,tool_name,tool_class,arguments,status,started_at) VALUES(?,?,?,?,?,?,?,?)`, event.ID, runID, event.WorkerID, event.ToolName, event.ToolClass, event.Arguments, event.Status, now)
		return err
	}
	_, err := s.db.ExecContext(ctx, `UPDATE agent_tool_calls SET status=?,result=?,error=?,ended_at=? WHERE id=? AND run_id=?`, event.Status, event.Result, event.Error, now, event.ID, runID)
	return err
}

type runtimeWorkerEvent struct{ ID, Prompt, AgentName, Status, Result, Error string }
type runtimeToolEvent struct{ ID, WorkerID, ToolName, ToolClass, Arguments, Status, Result, Error string }

func (s *Service) RecordRuntimeEvent(ctx context.Context, eventName string, data any) error {
	run, ok := runtimeRunFromEvent(data)
	if !ok {
		return nil
	}
	switch eventName {
	case "worker_started", "worker_completed", "worker_failed":
		return s.UpsertWorkerEvent(ctx, run.RunID, runtimeWorkerEvent{ID: run.EventID, Prompt: run.Prompt, AgentName: run.AgentName, Status: run.Status, Result: run.Result, Error: run.Error})
	case "tool_started", "tool_completed", "tool_failed":
		return s.UpsertToolEvent(ctx, run.RunID, runtimeToolEvent{ID: run.EventID, WorkerID: run.WorkerID, ToolName: run.ToolName, ToolClass: run.ToolClass, Arguments: run.Arguments, Status: run.Status, Result: run.Result, Error: run.Error})
	}
	return nil
}

type runtimeEvent struct {
	RunID     string `json:"run_id"`
	OwnerType string `json:"owner_type"`
	OwnerID   string `json:"owner_id"`
	WorkerID  string `json:"worker_id"`
	EventID   string `json:"id"`
	Prompt    string `json:"prompt"`
	AgentName string `json:"agent_name"`
	Status    string `json:"status"`
	Result    string `json:"result"`
	Error     string `json:"error"`
	ToolName  string `json:"tool_name"`
	ToolClass string `json:"tool_class"`
	Arguments string `json:"arguments"`
}

func runtimeRunFromEvent(data any) (runtimeEvent, bool) {
	b, err := json.Marshal(data)
	if err != nil {
		return runtimeEvent{}, false
	}
	var e runtimeEvent
	if json.Unmarshal(b, &e) != nil {
		return runtimeEvent{}, false
	}
	return e, true
}

// AuditRows are intentionally JSON-friendly maps. The schema is expected to
// evolve as runtime callbacks gain more fields, while API consumers retain a
// stable object envelope.
func (s *Service) Timeline(ctx context.Context, ownerType, ownerID string) ([]map[string]any, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT event_type,event_name,status,input,output,error,created_at FROM agent_timeline WHERE owner_type=? AND owner_id=? ORDER BY created_at,id`, ownerType, ownerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var typ, name, status, input, output, eventErr, at sql.NullString
		if err := rows.Scan(&typ, &name, &status, &input, &output, &eventErr, &at); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{"event_type": typ.String, "event_name": name.String, "status": status.String, "input": input.String, "output": output.String, "error": eventErr.String, "created_at": at.String})
	}
	return out, rows.Err()
}

func (s *Service) AgentRuns(ctx context.Context, ownerType, ownerID string) ([]map[string]any, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,agent_name,status,input_snapshot,output_snapshot,error,started_at,ended_at FROM agent_runs WHERE owner_type=? AND owner_id=? ORDER BY started_at`, ownerType, ownerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanAuditRows(rows, []string{"id", "agent_name", "status", "input_snapshot", "output_snapshot", "error", "started_at", "ended_at"})
}

func (s *Service) AgentWorkers(ctx context.Context, ownerType, ownerID string) ([]map[string]any, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT w.id,w.agent_name,w.status,w.prompt,w.result,w.error,w.started_at,w.ended_at FROM agent_workers w JOIN agent_runs r ON r.id=w.run_id WHERE r.owner_type=? AND r.owner_id=? ORDER BY w.started_at`, ownerType, ownerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanAuditRows(rows, []string{"id", "agent_name", "status", "prompt", "result", "error", "started_at", "ended_at"})
}

func (s *Service) AgentToolCalls(ctx context.Context, ownerType, ownerID string) ([]map[string]any, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT t.id,t.tool_name,t.tool_class,t.arguments,t.status,t.result,t.error,t.started_at,t.ended_at FROM agent_tool_calls t JOIN agent_runs r ON r.id=t.run_id WHERE r.owner_type=? AND r.owner_id=? ORDER BY t.started_at`, ownerType, ownerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanAuditRows(rows, []string{"id", "tool_name", "tool_class", "arguments", "status", "result", "error", "started_at", "ended_at"})
}

type rowsScanner interface {
	Next() bool
	Scan(...any) error
}

func scanAuditRows(rows rowsScanner, keys []string) ([]map[string]any, error) {
	out := []map[string]any{}
	for rows.Next() {
		values := make([]sql.NullString, len(keys))
		args := make([]any, len(values))
		for i := range values {
			args[i] = &values[i]
		}
		if err := rows.Scan(args...); err != nil {
			return nil, err
		}
		item := map[string]any{}
		for i, key := range keys {
			item[key] = values[i].String
		}
		out = append(out, item)
	}
	return out, nil
}
