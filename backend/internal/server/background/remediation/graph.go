package remediation

import "time"

// The workflow is intentionally represented as explicit Plan/Execute/Observe
// phases. Keeping the state in this value makes the transition contract clear
// and allows the runner to be resumed from SQLite snapshots after a restart.
type WorkflowState struct {
	TaskID        string      `json:"task_id"`
	Round         int         `json:"round"`
	Evidence      Evidence    `json:"evidence"`
	Decision      Decision    `json:"decision"`
	Plan          ActionPlan  `json:"plan"`
	Observation   Observation `json:"observation"`
	Handoff       bool        `json:"handoff"`
	HandoffReason string      `json:"handoff_reason,omitempty"`
}

const (
	MaxRounds      = 3
	ReplanInterval = 30 * time.Second
	TaskTimeout    = 3 * time.Minute
)
