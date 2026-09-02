package cases

import (
	"context"
	"testing"

	sqliterepo "AutoOps/internal/repo/sqlite"
)

func TestRecordRuntimeWorkerAndToolEvents(t *testing.T) {
	db, err := sqliterepo.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	service := NewService(db.DB)
	runID, err := service.StartAgentRun(context.Background(), "incident", "incident-1", "incident-coordinator", "inspect")
	if err != nil {
		t.Fatal(err)
	}
	if err := service.RecordRuntimeEvent(context.Background(), "worker_started", map[string]any{"id": "worker-1", "run_id": runID, "agent_name": "specialist-logs", "prompt": "read logs", "status": "running"}); err != nil {
		t.Fatal(err)
	}
	if err := service.RecordRuntimeEvent(context.Background(), "worker_completed", map[string]any{"id": "worker-1", "run_id": runID, "agent_name": "specialist-logs", "status": "completed", "result": "found probe failure"}); err != nil {
		t.Fatal(err)
	}
	if err := service.RecordRuntimeEvent(context.Background(), "tool_started", map[string]any{"id": "tool-1", "run_id": runID, "tool_name": "query_kubernetes_logs", "tool_class": "read-only", "status": "running"}); err != nil {
		t.Fatal(err)
	}
	if err := service.RecordRuntimeEvent(context.Background(), "tool_completed", map[string]any{"id": "tool-1", "run_id": runID, "tool_name": "query_kubernetes_logs", "tool_class": "read-only", "status": "completed", "result": "logs"}); err != nil {
		t.Fatal(err)
	}
	workers, err := service.AgentWorkers(context.Background(), "incident", "incident-1")
	if err != nil || len(workers) != 1 || workers[0]["status"] != "completed" {
		t.Fatalf("unexpected workers=%#v err=%v", workers, err)
	}
	tools, err := service.AgentToolCalls(context.Background(), "incident", "incident-1")
	if err != nil || len(tools) != 1 || tools[0]["status"] != "completed" {
		t.Fatalf("unexpected tools=%#v err=%v", tools, err)
	}
	if err := service.AddTimeline(context.Background(), "incident", "incident-1", "tool_completed", "completed", `{"tool_name":"query_kubernetes_logs"}`, ""); err != nil {
		t.Fatal(err)
	}
	timeline, err := service.Timeline(context.Background(), "incident", "incident-1")
	if err != nil || len(timeline) != 1 || timeline[0]["event_name"] != "tool_completed" {
		t.Fatalf("unexpected timeline=%#v err=%v", timeline, err)
	}
}
