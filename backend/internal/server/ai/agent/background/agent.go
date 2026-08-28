package backgroundagent

import (
	"context"

	kuberepo "AutoOps/internal/repo/kubernetes"
	shared "AutoOps/internal/server/ai/agent/shared"
	"AutoOps/pkg/config"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
)

// Agent is the read-only LLM reasoning component used by background remediation.
// It intentionally has no Kubernetes mutation tools.
type Agent struct {
	runner compose.Runnable[*shared.UserMessage, *schema.Message]
}

func NewAgent(ctx context.Context, cfg *config.Config, kube kuberepo.KubernetesRepository) (*Agent, error) {
	r, err := shared.BuildScopedAgent(ctx, cfg, kube, "background", systemPrompt)
	if err != nil {
		return nil, err
	}
	return &Agent{runner: r}, nil
}
func (a *Agent) Runner() compose.Runnable[*shared.UserMessage, *schema.Message] { return a.runner }
func (a *Agent) Analyze(ctx context.Context, taskID, evidence string) (string, error) {
	out, err := a.runner.Invoke(ctx, &shared.UserMessage{ID: "background:" + taskID, Query: "请只输出 JSON，不要 Markdown。证据：" + evidence})
	if err != nil {
		return "", err
	}
	return out.Content, nil
}

const systemPrompt = `你是 Background Remediation Agent。当前环境为本地 Minikube。只读采集和分析告警、Kubernetes 资源、Events、日志和维护文档。不得执行任何 Kubernetes 写操作。基于实际证据只输出 JSON：{"severity":"simple|severe|unknown","confidence":"low|medium|high","summary":"...","evidence":["..."],"proposed_action":"delete_managed_pod|create_incident","target_pod":"...","reason":"...","requires_human":true}。只有单个明确异常的受控 Pod 才能建议 delete_managed_pod；Critical、多 Pod、Deployment 不可用、证据不足或未知情况必须建议 create_incident。`
