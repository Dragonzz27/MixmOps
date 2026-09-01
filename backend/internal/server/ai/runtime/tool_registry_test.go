package runtime

import (
	"context"
	"testing"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

type namedTool struct{ name string }

func (t namedTool) Info(context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{Name: t.name}, nil
}

func TestToolRegistryPermissionBoundary(t *testing.T) {
	r := NewToolRegistry()
	read := namedTool{name: "query"}
	proposal := namedTool{name: "propose"}
	mutation := namedTool{name: "delete"}
	for _, item := range []ToolDefinition{{Name: "query", Class: ToolReadOnly, Handler: read}, {Name: "propose", Class: ToolProposal, Handler: proposal}, {Name: "delete", Class: ToolMutation, Handler: mutation}} {
		if err := r.Register(item); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := r.Resolve(AgentDefinition{Name: "read-only", Tools: []string{"query", "propose"}, PermissionMode: ReadOnly}); err == nil {
		t.Fatal("read-only agent must not resolve proposal tools")
	}
	if _, err := r.Resolve(AgentDefinition{Name: "proposal", Tools: []string{"query", "propose"}, PermissionMode: Proposal}); err != nil {
		t.Fatalf("proposal agent should resolve read/proposal tools: %v", err)
	}
	if _, err := r.Resolve(AgentDefinition{Name: "mutation", Tools: []string{"delete"}, PermissionMode: Proposal}); err == nil {
		t.Fatal("mutation tool must never be exposed")
	}
	var _ tool.BaseTool = read
}
