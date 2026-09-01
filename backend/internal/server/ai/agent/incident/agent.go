package incident

import (
	kuberepo "AutoOps/internal/repo/kubernetes"
	shared "AutoOps/internal/server/ai/agent/shared"
	airuntime "AutoOps/internal/server/ai/runtime"
	"AutoOps/pkg/config"
	"context"
	"errors"
	"fmt"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"io"
	"strings"
	"time"
)

type Input struct {
	IncidentID string
	Message    string
	History    []*schema.Message
	Context    any
}
type Agent struct {
	runner      compose.Runnable[*shared.UserMessage, *schema.Message]
	coordinator *Coordinator
}

func (a *Agent) Runner() compose.Runnable[*shared.UserMessage, *schema.Message] { return a.runner }

func NewAgent(ctx context.Context, cfg *config.Config, kube kuberepo.KubernetesRepository) (*Agent, error) {
	agentTimeout, err := parseTimeout(cfg.Incident.AgentTimeout, 5*time.Minute, "Incident Agent")
	if err != nil {
		return nil, err
	}
	workerTimeout, err := parseTimeout(cfg.Incident.WorkerTimeout, 75*time.Second, "Incident Specialist")
	if err != nil {
		return nil, err
	}
	registry, prompt := loadPersona("incident-coordinator", systemPrompt)
	r, err := airuntime.BuildScopedAgent(ctx, cfg, kube, "incident", prompt)
	if err != nil {
		return nil, err
	}
	// Incident diagnosis may include several bounded tool calls before the
	// final model response. Keep the overall conversation deadline above the
	// sum of those tool budgets; individual tools still enforce 10-15s limits.
	r, err = airuntime.Wrap(r, "incident-coordinator", registry, agentTimeout)
	if err != nil {
		return nil, err
	}
	coordinator := NewCoordinator()
	workers := []struct {
		name   string
		prompt string
	}{
		{"specialist-kubernetes", "你是 Kubernetes 专家。仅查询 Pod、Deployment、Service、Owner、Conditions 和 rollout，返回事实、证据、假设和下一步检查。"},
		{"specialist-observability", "你是 Observability 专家。仅查询 Prometheus 告警和指标，返回影响范围、时间线、恢复状态和证据。"},
		{"specialist-logs", "你是日志与事件专家。仅查询 Kubernetes Events 和 Pod 日志，定位 CrashLoop、OOM、探针、镜像和依赖错误。"},
		{"specialist-runbook", "你是维护文档专家。检索维护文档，优先 incident-runbook 和 change-standard，只返回与实时证据匹配的操作依据。"},
	}
	for _, spec := range workers {
		reg, p := loadPersona(spec.name, spec.prompt)
		runner, buildErr := airuntime.BuildScopedAgent(ctx, cfg, kube, spec.name, p)
		if buildErr != nil {
			return nil, fmt.Errorf("initialize %s: %w", spec.name, buildErr)
		}
		runner, buildErr = airuntime.Wrap(runner, spec.name, reg, workerTimeout)
		if buildErr != nil {
			return nil, fmt.Errorf("wrap %s runtime: %w", spec.name, buildErr)
		}
		if buildErr = coordinator.Register(RuntimeSpecialist{NameValue: spec.name, Runner: runner}); buildErr != nil {
			return nil, buildErr
		}
	}
	coordinated, err := withCoordinator(r, coordinator, workerTimeout+5*time.Second)
	if err != nil {
		return nil, err
	}
	return &Agent{runner: coordinated, coordinator: coordinator}, nil
}

func (a *Agent) Coordinator() *Coordinator { return a.coordinator }

func (a *Agent) Investigate(ctx context.Context, in SpecialistInput, names []string) []SpecialistResult {
	if a.coordinator == nil {
		return nil
	}
	return a.coordinator.Investigate(ctx, in, names)
}

func withCoordinator(base compose.Runnable[*shared.UserMessage, *schema.Message], coordinator *Coordinator, workerTimeout time.Duration) (compose.Runnable[*shared.UserMessage, *schema.Message], error) {
	prepare := func(ctx context.Context, in *shared.UserMessage) *shared.UserMessage {
		// Worker fan-out is performed for the first turn only. Later turns use
		// the persisted conversation and can request targeted tools themselves.
		if coordinator == nil || len(in.History) > 0 {
			return in
		}
		workerCtx, cancel := context.WithTimeout(ctx, workerTimeout)
		defer cancel()
		results := coordinator.Investigate(workerCtx, SpecialistInput{IncidentID: in.ID, Task: truncateText(in.Query, 30000)}, []string{"specialist-kubernetes", "specialist-observability", "specialist-logs", "specialist-runbook"})
		var evidence strings.Builder
		for _, result := range results {
			evidence.WriteString("\n[专家 " + result.AgentName + "]\n")
			evidence.WriteString(truncateText(result.Summary, 8000))
			if len(result.Errors) > 0 {
				evidence.WriteString("\n错误：" + strings.Join(result.Errors, "; "))
			}
		}
		copy := *in
		copy.Query = in.Query + "\n\n以下是本轮只读专家证据，请核对后再下结论：" + evidence.String()
		return &copy
	}
	invoke := func(ctx context.Context, in *shared.UserMessage, _ ...compose.Option) (*schema.Message, error) {
		return base.Invoke(ctx, prepare(ctx, in))
	}
	stream := func(ctx context.Context, in *shared.UserMessage, _ ...compose.Option) (*schema.StreamReader[*schema.Message], error) {
		upstream, err := base.Stream(ctx, prepare(ctx, in))
		if err != nil {
			return nil, err
		}
		reader, writer := schema.Pipe[*schema.Message](32)
		go func() {
			defer writer.Close()
			for {
				part, recvErr := upstream.Recv()
				if errors.Is(recvErr, io.EOF) {
					return
				}
				if recvErr != nil {
					writer.Send(nil, recvErr)
					return
				}
				writer.Send(part, nil)
			}
		}()
		return reader, nil
	}
	lambda, err := compose.AnyLambda[*shared.UserMessage, *schema.Message, compose.Option](invoke, stream, nil, nil)
	if err != nil {
		return nil, err
	}
	graph := compose.NewGraph[*shared.UserMessage, *schema.Message]()
	if err := graph.AddLambdaNode("coordinator", lambda); err != nil {
		return nil, err
	}
	if err := graph.AddEdge(compose.START, "coordinator"); err != nil {
		return nil, err
	}
	if err := graph.AddEdge("coordinator", compose.END); err != nil {
		return nil, err
	}
	return graph.Compile(context.Background(), compose.WithGraphName("incidentCoordinator"))
}

func parseTimeout(value string, fallback time.Duration, label string) (time.Duration, error) {
	if strings.TrimSpace(value) == "" {
		return fallback, nil
	}
	duration, err := time.ParseDuration(value)
	if err != nil || duration <= 0 {
		return 0, fmt.Errorf("invalid %s timeout %q", label, value)
	}
	return duration, nil
}

func truncateText(value string, maxRunes int) string {
	runes := []rune(value)
	if len(runes) <= maxRunes {
		return value
	}
	return string(runes[:maxRunes]) + "\n...<context truncated>"
}

func loadPersona(name, fallback string) (*airuntime.Registry, string) {
	for _, root := range []string{"agents", "backend/agents", "../backend/agents", "../agents"} {
		if reg, err := airuntime.Load(root); err == nil {
			if def, ok := reg.Get(name); ok && def.Prompt != "" {
				return reg, def.Prompt + "\n\n" + fallback
			}
		}
	}
	return nil, fallback
}

const systemPrompt = `你是 Incident 故障排查 Agent，负责与人工协作彻底恢复本地 Minikube 集群故障。先查询 Prometheus、Pod、Deployment、Events、日志、资源 YAML、Owner 和 rollout history，再形成假设。严格区分观测事实、推断和未知；故障预案参考 incident-runbook，变更回滚参考 change-standard。不得伪造结果，不得直接执行 Kubernetes 写操作。需要修复时必须先输出动作、目标、影响、风险、回滚和验证方式，并明确等待用户确认；确认后由后端受控执行。支持的动作仅为 delete_managed_pod、rollout_restart_deployment、rollback_deployment、scale_deployment，禁止修改其他资源、exec 或脚本执行。动作完成后必须重新观测，只有 Pod、Deployment 和告警均恢复才可建议解决。输出使用纯文本。`

func (a *Agent) Chat(ctx context.Context, in Input) (*schema.Message, error) {
	return a.runner.Invoke(ctx, &shared.UserMessage{ID: "incident:" + in.IncidentID, Query: in.Message, History: in.History})
}
func (a *Agent) Stream(ctx context.Context, in Input) (*schema.StreamReader[*schema.Message], error) {
	return a.runner.Stream(ctx, &shared.UserMessage{ID: "incident:" + in.IncidentID, Query: in.Message, History: in.History})
}
