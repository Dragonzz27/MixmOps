package shared

import (
	"context"
	"fmt"

	kuberepo "AutoOps/internal/repo/kubernetes"
	"AutoOps/internal/server/ai/tools"
	"AutoOps/internal/server/model"
	"AutoOps/pkg/config"
	"github.com/cloudwego/eino/components/prompt"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/flow/agent/react"
	"github.com/cloudwego/eino/schema"
)

// BuildScopedAgent creates an isolated ReAct graph. The mode controls its tool
// registry; each caller receives a distinct graph and prompt boundary.
func BuildScopedAgent(ctx context.Context, cfg *config.Config, kube kuberepo.KubernetesRepository, mode, systemPrompt string) (compose.Runnable[*UserMessage, *schema.Message], error) {
	model, err := model.NewOpenaiModel(ctx, cfg)
	if err != nil {
		return nil, err
	}
	timeTool, err := tools.TimeTool(ctx)
	if err != nil {
		return nil, err
	}
	ragTool, err := tools.RetrieveTool()
	if err != nil {
		return nil, err
	}
	all := []tool.BaseTool{timeTool, ragTool}
	if mode == "incident" {
		alerts, e := tools.NewPrometheusAlertsTool(cfg.GetPrometheusURL())
		if e != nil {
			return nil, e
		}
		metrics, e := tools.NewPrometheusQueryTool(cfg.GetPrometheusURL())
		if e != nil {
			return nil, e
		}
		all = append(all, alerts, metrics)
	} else if mode == "workorder" {
		gens, e := tools.NewGenerationTools()
		if e != nil {
			return nil, e
		}
		all = append(all, gens...)
	} else {
		return nil, fmt.Errorf("unknown agent mode %q", mode)
	}
	if kube != nil {
		kt, e := tools.NewKubernetesTools(kube, cfg.Kubernetes.Namespace)
		if e != nil {
			return nil, e
		}
		all = append(all, kt...)
	}
	agent, err := react.NewAgent(ctx, &react.AgentConfig{ToolCallingModel: model, ToolsConfig: compose.ToolsNodeConfig{Tools: all}})
	if err != nil {
		return nil, err
	}
	// A graph wrapper gives callers the full Runnable contract (including stream
	// and collect methods) while keeping this agent's prompt isolated.
	g := compose.NewGraph[*UserMessage, *schema.Message]()
	_ = g.AddLambdaNode("input", compose.InvokableLambda(func(ctx context.Context, in *UserMessage) (map[string]any, error) {
		return map[string]any{"content": in.Query, "history": in.History}, nil
	}))
	_ = g.AddChatTemplateNode("prompt", prompt.FromMessages(schema.FString, schema.SystemMessage(systemPrompt), schema.MessagesPlaceholder("history", false), schema.UserMessage("{content}")))
	lambda, err := compose.AnyLambda(agent.Generate, agent.Stream, nil, nil)
	if err != nil {
		return nil, err
	}
	_ = g.AddLambdaNode("agent", lambda)
	_ = g.AddEdge(compose.START, "input")
	_ = g.AddEdge("input", "prompt")
	_ = g.AddEdge("prompt", "agent")
	_ = g.AddEdge("agent", compose.END)
	return g.Compile(ctx, compose.WithGraphName(mode+"Agent"))
}
