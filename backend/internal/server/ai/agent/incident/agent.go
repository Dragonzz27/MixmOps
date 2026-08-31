package incident

import (
	kuberepo "AutoOps/internal/repo/kubernetes"
	shared "AutoOps/internal/server/ai/agent/shared"
	"AutoOps/pkg/config"
	"context"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
)

type Input struct {
	IncidentID string
	Message    string
	History    []*schema.Message
	Context    any
}
type Agent struct {
	runner compose.Runnable[*shared.UserMessage, *schema.Message]
}

func (a *Agent) Runner() compose.Runnable[*shared.UserMessage, *schema.Message] { return a.runner }

func NewAgent(ctx context.Context, cfg *config.Config, kube kuberepo.KubernetesRepository) (*Agent, error) {
	r, err := shared.BuildScopedAgent(ctx, cfg, kube, "incident", systemPrompt)
	if err != nil {
		return nil, err
	}
	return &Agent{runner: r}, nil
}

const systemPrompt = `你是 Incident 故障排查 Agent，负责与人工协作彻底恢复本地 Minikube 集群故障。先查询 Prometheus、Pod、Deployment、Events、日志、资源 YAML、Owner 和 rollout history，再形成假设。严格区分观测事实、推断和未知；故障预案参考 incident-runbook，变更回滚参考 change-standard。不得伪造结果，不得直接执行 Kubernetes 写操作。需要修复时必须先输出动作、目标、影响、风险、回滚和验证方式，并明确等待用户确认；确认后由后端受控执行。支持的动作仅为 delete_managed_pod、rollout_restart_deployment、rollback_deployment、scale_deployment，禁止修改其他资源、exec 或脚本执行。动作完成后必须重新观测，只有 Pod、Deployment 和告警均恢复才可建议解决。输出使用纯文本。`

func (a *Agent) Chat(ctx context.Context, in Input) (*schema.Message, error) {
	return a.runner.Invoke(ctx, &shared.UserMessage{ID: "incident:" + in.IncidentID, Query: in.Message, History: in.History})
}
func (a *Agent) Stream(ctx context.Context, in Input) (*schema.StreamReader[*schema.Message], error) {
	return a.runner.Stream(ctx, &shared.UserMessage{ID: "incident:" + in.IncidentID, Query: in.Message, History: in.History})
}
