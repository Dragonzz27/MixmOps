package incident

import (
	"context"
	"fmt"

	shared "AutoOps/internal/server/ai/agent/shared"
	airuntime "AutoOps/internal/server/ai/runtime"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
)

// RuntimeSpecialist adapts a persona registered in the shared runtime to the
// coordinator worker contract. The prompt is fully self-contained for every
// invocation, preventing accidental sharing of hidden worker context.
type RuntimeSpecialist struct {
	NameValue string
	Runner    compose.Runnable[*shared.UserMessage, *schema.Message]
}

func (s RuntimeSpecialist) Name() string { return s.NameValue }

func (s RuntimeSpecialist) Run(ctx context.Context, in SpecialistInput) (SpecialistResult, error) {
	if s.Runner == nil {
		return SpecialistResult{AgentName: s.NameValue}, fmt.Errorf("specialist runtime unavailable")
	}
	workerID := "worker:" + in.IncidentID + ":" + s.NameValue
	run, _ := airuntime.CurrentRun(ctx)
	ctx = airuntime.WithWorkerID(ctx, workerID)
	airuntime.EmitEvent(ctx, airuntime.Event{Type: "worker_started", Data: airuntime.WorkerEvent{ID: workerID, RunID: run.RunID, AgentName: s.NameValue, Prompt: in.Task, Status: "running"}})
	out, err := s.Runner.Invoke(ctx, &shared.UserMessage{ID: workerID, Query: in.Task})
	result := SpecialistResult{AgentName: s.NameValue}
	if err != nil {
		airuntime.EmitEvent(ctx, airuntime.Event{Type: "worker_failed", Data: airuntime.WorkerEvent{ID: workerID, RunID: run.RunID, AgentName: s.NameValue, Status: "failed", Error: err.Error()}})
		return result, err
	}
	if out != nil {
		result.Summary = out.Content
	}
	airuntime.EmitEvent(ctx, airuntime.Event{Type: "worker_completed", Data: airuntime.WorkerEvent{ID: workerID, RunID: run.RunID, AgentName: s.NameValue, Status: "completed", Result: boundedSummary(result.Summary)}})
	return result, nil
}

func boundedSummary(value string) string {
	runes := []rune(value)
	if len(runes) > 8000 {
		return string(runes[:8000]) + "\n...<truncated>"
	}
	return value
}
