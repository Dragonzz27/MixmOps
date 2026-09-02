package runtime

import (
	"context"
	"time"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"github.com/google/uuid"
)

type observedInvokableTool struct {
	base      tool.InvokableTool
	name      string
	class     ToolClass
	timeout   time.Duration
	maxOutput int
}

func observeTool(ctx context.Context, candidate tool.BaseTool, class ToolClass, timeout time.Duration, maxOutput int) tool.BaseTool {
	invokable, ok := candidate.(tool.InvokableTool)
	if !ok {
		return candidate
	}
	info, err := candidate.Info(ctx)
	if err != nil || info == nil {
		return candidate
	}
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	if maxOutput <= 0 {
		maxOutput = 12000
	}
	return &observedInvokableTool{base: invokable, name: info.Name, class: class, timeout: timeout, maxOutput: maxOutput}
}

func (t *observedInvokableTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return t.base.Info(ctx)
}

func (t *observedInvokableTool) InvokableRun(ctx context.Context, arguments string, opts ...tool.Option) (string, error) {
	callID := uuid.NewString()
	workerID := CurrentWorkerID(ctx)
	run, _ := CurrentRun(ctx)
	EmitEvent(ctx, Event{Type: "tool_started", Data: ToolCallEvent{ID: callID, RunID: run.RunID, WorkerID: workerID, ToolName: t.name, ToolClass: string(t.class), Arguments: boundedString(arguments, 4000), Status: "running"}})

	toolCtx, cancel := context.WithTimeout(ctx, t.timeout)
	defer cancel()
	result, err := t.base.InvokableRun(toolCtx, arguments, opts...)
	if err != nil {
		EmitEvent(ctx, Event{Type: "tool_failed", Data: ToolCallEvent{ID: callID, RunID: run.RunID, WorkerID: workerID, ToolName: t.name, ToolClass: string(t.class), Status: "failed", Error: boundedString(err.Error(), 4000)}})
		return "", err
	}
	bounded := boundedString(result, t.maxOutput)
	EmitEvent(ctx, Event{Type: "tool_completed", Data: ToolCallEvent{ID: callID, RunID: run.RunID, WorkerID: workerID, ToolName: t.name, ToolClass: string(t.class), Status: "completed", Result: bounded}})
	return result, nil
}

func boundedString(value string, limit int) string {
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit]) + "\n...<truncated>"
}
