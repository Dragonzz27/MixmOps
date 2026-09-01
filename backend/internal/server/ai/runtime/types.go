package runtime

import (
	"context"
	"github.com/cloudwego/eino/schema"
	"time"
)

type PermissionMode string

const (
	ReadOnly PermissionMode = "read-only"
	Proposal PermissionMode = "proposal"
)

type AgentDefinition struct {
	Name            string         `yaml:"name" json:"name"`
	Description     string         `yaml:"description" json:"description"`
	Capabilities    []Capability   `yaml:"capabilities" json:"capabilities"`
	Tools           []string       `yaml:"tools" json:"tools"`
	DisallowedTools []string       `yaml:"disallowed_tools" json:"disallowed_tools"`
	PermissionMode  PermissionMode `yaml:"permission_mode" json:"permission_mode"`
	MaxTurns        int            `yaml:"max_turns" json:"max_turns"`
	Prompt          string         `yaml:"-" json:"-"`
}

type Capability struct {
	ID    string   `yaml:"id" json:"id"`
	Tools []string `yaml:"tools" json:"tools"`
}
type RunInput struct {
	AgentName    string
	SessionID    string
	SystemPrompt string
	UserPrompt   string
	History      []*schema.Message
	Context      any
	OwnerType    string
	OwnerID      string
}
type RunOutput struct {
	AgentName string
	Message   *schema.Message
	ToolCalls []ToolCallRecord `json:"tool_calls,omitempty"`
	Workers   []WorkerRecord
	Usage     Usage
	StartedAt time.Time
	EndedAt   time.Time
}
type ToolCallRecord struct {
	ToolName  string    `json:"tool_name"`
	Arguments string    `json:"arguments,omitempty"`
	Status    string    `json:"status"`
	Error     string    `json:"error,omitempty"`
	StartedAt time.Time `json:"started_at"`
	EndedAt   time.Time `json:"ended_at"`
}
type Usage struct {
	PromptTokens     int `json:"prompt_tokens,omitempty"`
	CompletionTokens int `json:"completion_tokens,omitempty"`
	TotalTokens      int `json:"total_tokens,omitempty"`
}
type WorkerRecord struct {
	ID        string `json:"id"`
	AgentName string `json:"agent_name"`
	Status    string `json:"status"`
	Result    string `json:"result,omitempty"`
	Error     string `json:"error,omitempty"`
}
type Emit func(Event)
type Event struct {
	Type string `json:"type"`
	Data any    `json:"data"`
}
type Runner interface {
	Run(context.Context, RunInput) (RunOutput, error)
	Stream(context.Context, RunInput, Emit) error
}
