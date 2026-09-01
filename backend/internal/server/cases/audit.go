package cases

import (
	"context"
	"database/sql"
)

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
