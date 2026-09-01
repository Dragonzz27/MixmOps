package incident

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// EvidenceFact is a single observation returned by a specialist. Specialists
// never produce the user-facing answer; the coordinator owns synthesis.
type EvidenceFact struct {
	Source  string `json:"source"`
	Content string `json:"content"`
}

type Hypothesis struct {
	Summary    string   `json:"summary"`
	Confidence string   `json:"confidence"`
	Evidence   []string `json:"evidence,omitempty"`
}

type SpecialistResult struct {
	AgentName  string         `json:"agent_name"`
	Summary    string         `json:"summary"`
	Facts      []EvidenceFact `json:"facts,omitempty"`
	Hypotheses []Hypothesis   `json:"hypotheses,omitempty"`
	Confidence string         `json:"confidence"`
	NextChecks []string       `json:"next_checks,omitempty"`
	Errors     []string       `json:"errors,omitempty"`
	StartedAt  time.Time      `json:"started_at"`
	EndedAt    time.Time      `json:"ended_at"`
}

type SpecialistInput struct {
	IncidentID string
	Task       string
	Context    any
}

// Specialist is intentionally smaller than the business Agent interface. A
// worker receives a self-contained task and cannot access coordinator state.
type Specialist interface {
	Name() string
	Run(context.Context, SpecialistInput) (SpecialistResult, error)
}

// Coordinator dispatches read-only workers and returns all evidence. Worker
// failures are isolated and represented in the result so one unavailable
// subsystem does not hide the remaining evidence.
type Coordinator struct {
	mu         sync.RWMutex
	workers    map[string]Specialist
	maxWorkers int
}

func NewCoordinator(workers ...Specialist) *Coordinator {
	c := &Coordinator{workers: map[string]Specialist{}, maxWorkers: 4}
	for _, worker := range workers {
		if worker != nil && worker.Name() != "" {
			c.workers[worker.Name()] = worker
		}
	}
	return c
}

func (c *Coordinator) Register(worker Specialist) error {
	if worker == nil || worker.Name() == "" {
		return fmt.Errorf("specialist is required")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, exists := c.workers[worker.Name()]; exists {
		return fmt.Errorf("specialist %q already registered", worker.Name())
	}
	c.workers[worker.Name()] = worker
	return nil
}

func (c *Coordinator) Investigate(ctx context.Context, input SpecialistInput, names []string) []SpecialistResult {
	c.mu.RLock()
	selected := make([]Specialist, 0, len(names))
	for _, name := range names {
		if worker, ok := c.workers[name]; ok {
			selected = append(selected, worker)
		}
	}
	c.mu.RUnlock()
	if len(selected) > c.maxWorkers {
		selected = selected[:c.maxWorkers]
	}
	results := make([]SpecialistResult, len(selected))
	var wg sync.WaitGroup
	for idx, worker := range selected {
		wg.Add(1)
		go func(i int, w Specialist) {
			defer wg.Done()
			started := time.Now().UTC()
			result, err := w.Run(ctx, input)
			if result.AgentName == "" {
				result.AgentName = w.Name()
			}
			if result.StartedAt.IsZero() {
				result.StartedAt = started
			}
			if result.EndedAt.IsZero() {
				result.EndedAt = time.Now().UTC()
			}
			if err != nil {
				result.Errors = append(result.Errors, err.Error())
			}
			results[i] = result
		}(idx, worker)
	}
	wg.Wait()
	return results
}
