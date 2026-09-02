package runtime

import "context"

type eventContextKey struct{}
type runContextKey struct{}
type workerContextKey struct{}

type RunContext struct {
	RunID     string `json:"run_id"`
	OwnerType string `json:"owner_type"`
	OwnerID   string `json:"owner_id"`
}

type WorkerEvent struct {
	ID             string `json:"id"`
	RunID          string `json:"run_id,omitempty"`
	ParentWorkerID string `json:"parent_worker_id,omitempty"`
	AgentName      string `json:"agent_name"`
	Prompt         string `json:"prompt,omitempty"`
	Status         string `json:"status"`
	Result         string `json:"result,omitempty"`
	Error          string `json:"error,omitempty"`
}

type ToolCallEvent struct {
	ID        string `json:"id"`
	RunID     string `json:"run_id,omitempty"`
	WorkerID  string `json:"worker_id,omitempty"`
	ToolName  string `json:"tool_name"`
	ToolClass string `json:"tool_class"`
	Arguments string `json:"arguments,omitempty"`
	Status    string `json:"status"`
	Result    string `json:"result,omitempty"`
	Error     string `json:"error,omitempty"`
}

func WithEventSink(ctx context.Context, emit Emit) context.Context {
	if emit == nil {
		return ctx
	}
	// A Runtime wrapper receives an explicit callback and is responsible for
	// forwarding events to its parent. Replacing the sink here avoids duplicate
	// events when nested runtimes are used by Coordinator workers.
	return context.WithValue(ctx, eventContextKey{}, emit)
}

func WithRunContext(ctx context.Context, run RunContext) context.Context {
	return context.WithValue(ctx, runContextKey{}, run)
}

func WithWorkerID(ctx context.Context, workerID string) context.Context {
	return context.WithValue(ctx, workerContextKey{}, workerID)
}

func CurrentRun(ctx context.Context) (RunContext, bool) {
	run, ok := ctx.Value(runContextKey{}).(RunContext)
	return run, ok
}

func CurrentWorkerID(ctx context.Context) string {
	workerID, _ := ctx.Value(workerContextKey{}).(string)
	return workerID
}

func EmitEvent(ctx context.Context, event Event) {
	if emit, ok := ctx.Value(eventContextKey{}).(Emit); ok && emit != nil {
		emit(event)
	}
}
