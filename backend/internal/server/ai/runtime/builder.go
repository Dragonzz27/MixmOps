package runtime

import (
	"context"
	"fmt"
	"strings"

	kuberepo "AutoOps/internal/repo/kubernetes"
	"AutoOps/internal/server/ai/agent/shared"
	"AutoOps/internal/server/ai/tools"
	"AutoOps/internal/server/model"
	"AutoOps/pkg/config"
	"github.com/cloudwego/eino/components/prompt"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/flow/agent/react"
	"github.com/cloudwego/eino/schema"
)

// BuildScopedAgent is the single Eino ReAct construction boundary. Business
// agents select a scope and prompt, while Runtime owns which tool families may
// be attached to that scope. Mutation tools are never attached here.
func BuildScopedAgent(ctx context.Context, cfg *config.Config, kube kuberepo.KubernetesRepository, mode, systemPrompt string) (compose.Runnable[*shared.UserMessage, *schema.Message], error) {
	chatModel, err := model.NewOpenaiModel(ctx, cfg)
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
	registered := []tool.BaseTool{}
	addBase := func(withRAG bool) {
		registered = append(registered, timeTool)
		if withRAG {
			registered = append(registered, ragTool)
		}
	}
	switch mode {
	case "incident":
		addBase(true)
		alerts, e := tools.NewPrometheusAlertsTool(cfg.GetPrometheusURL())
		if e != nil {
			return nil, e
		}
		metrics, e := tools.NewPrometheusQueryTool(cfg.GetPrometheusURL())
		if e != nil {
			return nil, e
		}
		registered = append(registered, alerts, metrics)
		if kube != nil {
			extra, e := tools.NewIncidentObservationTools(kube, cfg.Kubernetes.Namespace)
			if e != nil {
				return nil, e
			}
			registered = append(registered, extra...)
		}
		proposal, e := tools.NewIncidentActionProposalTool(cfg.Kubernetes.Namespace)
		if e != nil {
			return nil, e
		}
		registered = append(registered, proposal)
	case "background":
		addBase(true)
		alerts, e := tools.NewPrometheusAlertsTool(cfg.GetPrometheusURL())
		if e != nil {
			return nil, e
		}
		metrics, e := tools.NewPrometheusQueryTool(cfg.GetPrometheusURL())
		if e != nil {
			return nil, e
		}
		registered = append(registered, alerts, metrics)
	case "specialist-observability":
		alerts, e := tools.NewPrometheusAlertsTool(cfg.GetPrometheusURL())
		if e != nil {
			return nil, e
		}
		metrics, e := tools.NewPrometheusQueryTool(cfg.GetPrometheusURL())
		if e != nil {
			return nil, e
		}
		registered = append(registered, alerts, metrics)
	case "specialist-runbook":
		registered = append(registered, ragTool)
	case "specialist-kubernetes", "specialist-logs":
		// Kubernetes tools are attached below. The specialist persona controls
		// which of the returned read-only tools it is expected to call.
	default:
		return nil, fmt.Errorf("unknown agent scope %q", mode)
	}
	if kube != nil && (mode == "incident" || mode == "background" || mode == "specialist-kubernetes" || mode == "specialist-logs") {
		clusterTools, e := tools.NewKubernetesTools(kube, cfg.Kubernetes.Namespace)
		if e != nil {
			return nil, e
		}
		if mode == "specialist-kubernetes" {
			// The Kubernetes specialist receives resources plus the advanced
			// read-only inspection tools; it must not inherit Events/logs tools.
			registered = append(registered, clusterTools[0])
		} else if mode == "specialist-logs" {
			// Logs specialist is deliberately limited to Events and bounded logs.
			registered = append(registered, clusterTools[1:]...)
		} else {
			registered = append(registered, clusterTools...)
		}
		if mode == "specialist-kubernetes" {
			extra, e := tools.NewIncidentObservationTools(kube, cfg.Kubernetes.Namespace)
			if e != nil {
				return nil, e
			}
			registered = append(registered, extra...)
		}
	}
	registered, err = applyPersonaToolPolicy(ctx, mode, registered)
	if err != nil {
		return nil, err
	}

	agent, err := react.NewAgent(ctx, &react.AgentConfig{ToolCallingModel: chatModel, ToolsConfig: compose.ToolsNodeConfig{Tools: registered}})
	if err != nil {
		return nil, err
	}
	graph := compose.NewGraph[*shared.UserMessage, *schema.Message]()
	_ = graph.AddLambdaNode("input", compose.InvokableLambda(func(_ context.Context, in *shared.UserMessage) (map[string]any, error) {
		return map[string]any{"content": in.Query, "history": in.History}, nil
	}))
	_ = graph.AddChatTemplateNode("prompt", prompt.FromMessages(schema.FString, schema.SystemMessage(systemPrompt), schema.MessagesPlaceholder("history", false), schema.UserMessage("{content}")))
	lambda, err := compose.AnyLambda(agent.Generate, agent.Stream, nil, nil)
	if err != nil {
		return nil, err
	}
	_ = graph.AddLambdaNode("agent", lambda)
	_ = graph.AddEdge(compose.START, "input")
	_ = graph.AddEdge("input", "prompt")
	_ = graph.AddEdge("prompt", "agent")
	_ = graph.AddEdge("agent", compose.END)
	return graph.Compile(ctx, compose.WithGraphName(mode+"Agent"))
}

func applyPersonaToolPolicy(ctx context.Context, mode string, candidates []tool.BaseTool) ([]tool.BaseTool, error) {
	persona := map[string]string{
		"incident": "incident-coordinator", "background": "background-remediation",
		"specialist-kubernetes": "specialist-kubernetes", "specialist-observability": "specialist-observability",
		"specialist-logs": "specialist-logs", "specialist-runbook": "specialist-runbook",
	}[mode]
	if persona == "" {
		return candidates, nil
	}
	var definition AgentDefinition
	var found bool
	for _, root := range []string{"agents", "backend/agents", "../backend/agents", "../agents"} {
		if reg, err := Load(root); err == nil {
			if definition, found = reg.Get(persona); found {
				break
			}
		}
	}
	if !found {
		// A binary may be started from a directory without embedded personas;
		// retain the backwards-compatible scope list in that case.
		return candidates, nil
	}
	registry := NewToolRegistry()
	for _, candidate := range candidates {
		info, err := candidate.Info(ctx)
		if err != nil || info == nil || info.Name == "" {
			continue
		}
		class := ToolReadOnly
		if strings.HasPrefix(info.Name, "generate_") {
			class = ToolGenerated
		} else if strings.HasPrefix(info.Name, "propose_") {
			class = ToolProposal
		}
		if err := registry.Register(ToolDefinition{Name: info.Name, Class: class, Handler: candidate}); err != nil {
			return nil, err
		}
	}
	return registry.Resolve(definition)
}
