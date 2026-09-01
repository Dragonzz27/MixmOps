package runtime

import (
	"fmt"
	"github.com/cloudwego/eino/components/tool"
	"sync"
	"time"
)

type ToolClass string

const (
	ToolReadOnly  ToolClass = "read-only"
	ToolProposal  ToolClass = "proposal"
	ToolGenerated ToolClass = "generated"
	ToolMutation  ToolClass = "mutation"
)

type ToolDefinition struct {
	Name      string
	Class     ToolClass
	Timeout   time.Duration
	MaxOutput int
	Handler   tool.BaseTool
}
type ToolRegistry struct {
	mu    sync.RWMutex
	items map[string]ToolDefinition
}

func NewToolRegistry() *ToolRegistry { return &ToolRegistry{items: map[string]ToolDefinition{}} }
func (r *ToolRegistry) Register(d ToolDefinition) error {
	if d.Name == "" || d.Handler == nil {
		return fmt.Errorf("invalid tool")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.items[d.Name]; ok {
		return fmt.Errorf("duplicate tool %q", d.Name)
	}
	r.items[d.Name] = d
	return nil
}
func (r *ToolRegistry) Resolve(agent AgentDefinition) ([]tool.BaseTool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	blocked := map[string]bool{}
	for _, x := range agent.DisallowedTools {
		blocked[x] = true
	}
	out := []tool.BaseTool{}
	for _, name := range agent.Tools {
		d, ok := r.items[name]
		if !ok {
			return nil, fmt.Errorf("agent %s tool %s not registered", agent.Name, name)
		}
		if blocked[name] || d.Class == ToolMutation {
			return nil, fmt.Errorf("agent %s cannot use tool %s", agent.Name, name)
		}
		if d.Class == ToolProposal && agent.PermissionMode != Proposal {
			return nil, fmt.Errorf("agent %s requires proposal permission for tool %s", agent.Name, name)
		}
		out = append(out, d.Handler)
	}
	return out, nil
}
