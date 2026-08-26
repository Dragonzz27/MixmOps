package plan

import (
	kuberepo "AutoOps/internal/repo/kubernetes"
	planexecutereplan "AutoOps/internal/server/ai/agent/plan_execute_replan"
	"AutoOps/pkg/config"
	"context"

	"github.com/cloudwego/eino-ext/components/model/openai"
	qdrant_retriever "github.com/cloudwego/eino-ext/components/retriever/qdrant"
	"github.com/sirupsen/logrus"
)

// 运维server
type PlanServer interface {
	Plan(context.Context) (string, []string, error)
}

type planServer struct {
	cfg        config.Config
	model      *openai.ChatModel
	logger     *logrus.Logger
	retriever  *qdrant_retriever.Retriever
	kubernetes kuberepo.KubernetesRepository
}

func NewPlanServer(cfg config.Config, model *openai.ChatModel, logger *logrus.Logger, retriever *qdrant_retriever.Retriever, repositories ...kuberepo.KubernetesRepository) PlanServer {
	var repository kuberepo.KubernetesRepository
	if len(repositories) > 0 {
		repository = repositories[0]
	}
	return &planServer{
		cfg:        cfg,
		model:      model,
		logger:     logger,
		retriever:  retriever,
		kubernetes: repository,
	}
}
func (p *planServer) Plan(ctx context.Context) (string, []string, error) {
	//初始化运维agent
	query := `你是本地 Minikube 集群的智能运维分析助手。
1. 首先调用 query_prometheus_alerts 获取所有活跃告警。
2. 根据告警名称调用 query_internal_docs 获取对应处理方案，并严格遵循内部文档。
3. 查询相关 Kubernetes Pod、Deployment 状态和 Events，必要时读取 Pod 日志。
4. 涉及时间参数时先调用 get_current_time。
5. 最后分别总结每个告警的现象、证据、可能原因和建议，并汇总所有告警。`
	lastmsg, msgs, err := planexecutereplan.BuildPlanExecuteReplanAgent(ctx, query, p.cfg, p.model, p.retriever, p.kubernetes)
	if err != nil {
		p.logger.Errorf("运维agent初始化失败, err: %v", err)
		return "", nil, err
	}
	return lastmsg, msgs, nil
}
