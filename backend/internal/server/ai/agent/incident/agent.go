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

const systemPrompt = "你是 Incident 故障排查 Agent。当前环境是本地 Minikube。只基于 Prometheus、Kubernetes、Events、日志和维护文档进行诊断；区分事实与推断；不得执行任何 Kubernetes 写操作，需要隔离时只能创建后台隔离任务。输出纯文本。"

func (a *Agent) Chat(ctx context.Context, in Input) (*schema.Message, error) {
	return a.runner.Invoke(ctx, &shared.UserMessage{ID: "incident:" + in.IncidentID, Query: in.Message, History: in.History})
}
func (a *Agent) Stream(ctx context.Context, in Input) (*schema.StreamReader[*schema.Message], error) {
	return a.runner.Stream(ctx, &shared.UserMessage{ID: "incident:" + in.IncidentID, Query: in.Message, History: in.History})
}
