package incident

import (
	"context"
	"errors"
	"testing"
)

type fakeSpecialist struct {
	name string
	err  error
}

func (f fakeSpecialist) Name() string { return f.name }
func (f fakeSpecialist) Run(_ context.Context, _ SpecialistInput) (SpecialistResult, error) {
	return SpecialistResult{AgentName: f.name, Summary: "evidence"}, f.err
}

func TestCoordinatorIsolatesWorkerFailure(t *testing.T) {
	c := NewCoordinator(
		fakeSpecialist{name: "specialist-kubernetes"},
		fakeSpecialist{name: "specialist-logs", err: errors.New("logs unavailable")},
	)
	results := c.Investigate(context.Background(), SpecialistInput{IncidentID: "i-1", Task: "inspect"}, []string{"specialist-kubernetes", "specialist-logs"})
	if len(results) != 2 {
		t.Fatalf("got %d results", len(results))
	}
	if results[0].AgentName != "specialist-kubernetes" || results[0].Summary != "evidence" {
		t.Fatalf("successful worker result lost: %#v", results[0])
	}
	if len(results[1].Errors) != 1 || results[1].Errors[0] != "logs unavailable" {
		t.Fatalf("worker failure should be represented, got %#v", results[1])
	}
}
