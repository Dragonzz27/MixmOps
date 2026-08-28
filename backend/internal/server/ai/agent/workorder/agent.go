package workorder

import (
	kuberepo "AutoOps/internal/repo/kubernetes"
	shared "AutoOps/internal/server/ai/agent/shared"
	"AutoOps/pkg/config"
	"context"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
)

type Input struct {
	WorkOrderID string
	Message     string
	History     []*schema.Message
	Context     any
}
type Agent struct {
	runner compose.Runnable[*shared.UserMessage, *schema.Message]
}

func (a *Agent) Runner() compose.Runnable[*shared.UserMessage, *schema.Message] { return a.runner }

func NewAgent(ctx context.Context, cfg *config.Config, kube kuberepo.KubernetesRepository) (*Agent, error) {
	r, err := shared.BuildScopedAgent(ctx, cfg, kube, "workorder", systemPrompt)
	if err != nil {
		return nil, err
	}
	return &Agent{runner: r}, nil
}

const systemPrompt = "你是 Work Order 日常运维 Agent。只处理用户主动创建的运维工单，不加载告警或 Incident 上下文。缺少参数时先追问，生成方案时包含风险、回滚和验证步骤；YAML、Shell 和配置仅供审核，不得执行任何写操作。输出纯文本。"

func (a *Agent) Chat(ctx context.Context, in Input) (*schema.Message, error) {
	return a.runner.Invoke(ctx, &shared.UserMessage{ID: "workorder:" + in.WorkOrderID, Query: in.Message, History: in.History})
}
func (a *Agent) Stream(ctx context.Context, in Input) (*schema.StreamReader[*schema.Message], error) {
	return a.runner.Stream(ctx, &shared.UserMessage{ID: "workorder:" + in.WorkOrderID, Query: in.Message, History: in.History})
}
