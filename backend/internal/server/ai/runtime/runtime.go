package runtime

import (
	"AutoOps/internal/server/ai/agent/shared"
	"context"
	"errors"
	"fmt"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"io"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Runtime is the common lifecycle boundary for business agents. Business
// packages own their prompts and tool policy; Runtime owns lookup, deadlines
// and streaming event conversion.
type Runtime struct {
	registry *Registry
	runners  map[string]compose.Runnable[*shared.UserMessage, *schema.Message]
	timeout  time.Duration
	mu       sync.RWMutex
}

// Wrap adapts an Eino runnable to the common Runtime boundary. This lets
// legacy graph construction continue to evolve independently while all
// business agents receive the same timeout, cancellation and persona checks.
func Wrap(runner compose.Runnable[*shared.UserMessage, *schema.Message], agentName string, registry *Registry, timeout time.Duration) (compose.Runnable[*shared.UserMessage, *schema.Message], error) {
	rt := New(registry, timeout)
	if err := rt.Register(agentName, runner); err != nil {
		return nil, err
	}
	invoke := func(ctx context.Context, in *shared.UserMessage, _ ...compose.Option) (*schema.Message, error) {
		out, err := rt.Run(ctx, RunInput{AgentName: agentName, SessionID: in.ID, UserPrompt: in.Query, History: in.History})
		return out.Message, err
	}
	stream := func(ctx context.Context, in *shared.UserMessage, _ ...compose.Option) (*schema.StreamReader[*schema.Message], error) {
		reader, writer := schema.Pipe[*schema.Message](32)
		go func() {
			defer writer.Close()
			forward := func(event Event) {
				// Propagate lifecycle/tool events to the outer conversation sink.
				// Assistant tokens still flow through the Eino stream below.
				if event.Type != "assistant" {
					EmitEvent(ctx, event)
				}
				if content, ok := event.Data.(string); ok {
					writer.Send(schema.AssistantMessage(content, nil), nil)
				}
			}
			err := rt.Stream(ctx, RunInput{AgentName: agentName, SessionID: in.ID, UserPrompt: in.Query, History: in.History}, forward)
			if err != nil {
				writer.Send(nil, err)
			}
		}()
		return reader, nil
	}
	lambda, err := compose.AnyLambda[*shared.UserMessage, *schema.Message, compose.Option](invoke, stream, nil, nil)
	if err != nil {
		return nil, err
	}
	graph := compose.NewGraph[*shared.UserMessage, *schema.Message]()
	if err = graph.AddLambdaNode("runtime", lambda); err != nil {
		return nil, err
	}
	if err = graph.AddEdge(compose.START, "runtime"); err != nil {
		return nil, err
	}
	if err = graph.AddEdge("runtime", compose.END); err != nil {
		return nil, err
	}
	return graph.Compile(context.Background(), compose.WithGraphName(agentName+"Runtime"))
}

func New(reg *Registry, timeout time.Duration) *Runtime {
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	return &Runtime{registry: reg, runners: map[string]compose.Runnable[*shared.UserMessage, *schema.Message]{}, timeout: timeout}
}
func (r *Runtime) Register(name string, runner compose.Runnable[*shared.UserMessage, *schema.Message]) error {
	if name == "" || runner == nil {
		return fmt.Errorf("agent runtime: name and runner are required")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.runners[name]; ok {
		return fmt.Errorf("agent runtime: duplicate runner %q", name)
	}
	if r.registry != nil {
		if _, ok := r.registry.Get(name); !ok {
			return fmt.Errorf("agent persona %q is not registered", name)
		}
	}
	r.runners[name] = runner
	return nil
}
func (r *Runtime) runner(name string) (compose.Runnable[*shared.UserMessage, *schema.Message], bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	runner, ok := r.runners[name]
	return runner, ok
}
func (r *Runtime) Run(ctx context.Context, in RunInput) (RunOutput, error) {
	runner, ok := r.runner(in.AgentName)
	if !ok {
		return RunOutput{}, fmt.Errorf("agent %q is not registered", in.AgentName)
	}
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()
	started := time.Now().UTC()
	runID := uuid.NewString()
	ownerType, ownerID := in.OwnerType, in.OwnerID
	if existing, ok := CurrentRun(ctx); ok && existing.RunID != "" {
		runID = existing.RunID
		if ownerType == "" {
			ownerType = existing.OwnerType
		}
		if ownerID == "" {
			ownerID = existing.OwnerID
		}
	}
	ctx = WithRunContext(ctx, RunContext{RunID: runID, OwnerType: ownerType, OwnerID: ownerID})
	EmitEvent(ctx, Event{Type: "run_started", Data: map[string]any{"run_id": runID, "agent": in.AgentName, "owner_type": ownerType, "owner_id": ownerID}})
	out, err := runner.Invoke(ctx, &shared.UserMessage{ID: in.SessionID, Query: in.UserPrompt, History: in.History})
	if err != nil {
		EmitEvent(ctx, Event{Type: "run_failed", Data: map[string]any{"run_id": runID, "agent": in.AgentName, "error": boundedString(err.Error(), 4000)}})
		return RunOutput{AgentName: in.AgentName, StartedAt: started, EndedAt: time.Now().UTC()}, err
	}
	EmitEvent(ctx, Event{Type: "run_completed", Data: map[string]any{"run_id": runID, "agent": in.AgentName}})
	return RunOutput{AgentName: in.AgentName, Message: out, StartedAt: started, EndedAt: time.Now().UTC()}, nil
}
func (r *Runtime) Stream(ctx context.Context, in RunInput, emit Emit) error {
	runner, ok := r.runner(in.AgentName)
	if !ok {
		return fmt.Errorf("agent %q is not registered", in.AgentName)
	}
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()
	runID := uuid.NewString()
	ownerType, ownerID := in.OwnerType, in.OwnerID
	if existing, ok := CurrentRun(ctx); ok {
		if existing.RunID != "" {
			runID = existing.RunID
		}
		if ownerType == "" {
			ownerType = existing.OwnerType
		}
		if ownerID == "" {
			ownerID = existing.OwnerID
		}
	}
	ctx = WithRunContext(ctx, RunContext{RunID: runID, OwnerType: ownerType, OwnerID: ownerID})
	if emit != nil {
		ctx = WithEventSink(ctx, emit)
	}
	EmitEvent(ctx, Event{Type: "run_started", Data: map[string]any{"run_id": runID, "agent": in.AgentName, "owner_type": ownerType, "owner_id": ownerID}})
	stream, err := runner.Stream(ctx, &shared.UserMessage{ID: in.SessionID, Query: in.UserPrompt, History: in.History})
	if err != nil {
		EmitEvent(ctx, Event{Type: "run_failed", Data: map[string]any{"run_id": runID, "agent": in.AgentName, "error": boundedString(err.Error(), 4000)}})
		return err
	}
	for {
		part, e := stream.Recv()
		if errors.Is(e, io.EOF) {
			EmitEvent(ctx, Event{Type: "run_completed", Data: map[string]any{"run_id": runID, "agent": in.AgentName}})
			return nil
		}
		if e != nil {
			if errors.Is(e, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
				err := fmt.Errorf("agent %s timed out after %s: %w", in.AgentName, r.timeout, context.DeadlineExceeded)
				EmitEvent(ctx, Event{Type: "run_failed", Data: map[string]any{"run_id": runID, "agent": in.AgentName, "error": boundedString(err.Error(), 4000)}})
				return err
			}
			EmitEvent(ctx, Event{Type: "run_failed", Data: map[string]any{"run_id": runID, "agent": in.AgentName, "error": boundedString(e.Error(), 4000)}})
			return e
		}
		if emit != nil {
			emit(Event{Type: "assistant", Data: part.Content})
		}
	}
}
