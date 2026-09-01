package incident

import (
	"context"
	"fmt"

	shared "AutoOps/internal/server/ai/agent/shared"
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
	out, err := s.Runner.Invoke(ctx, &shared.UserMessage{ID: "worker:" + in.IncidentID + ":" + s.NameValue, Query: in.Task})
	result := SpecialistResult{AgentName: s.NameValue}
	if err != nil {
		return result, err
	}
	if out != nil {
		result.Summary = out.Content
	}
	return result, nil
}
