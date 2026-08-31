package tools

import (
	kuberepo "AutoOps/internal/repo/kubernetes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
)

type ResourceInput struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
}

func NewIncidentObservationTools(repo kuberepo.KubernetesRepository, ns string) ([]tool.BaseTool, error) {
	if repo == nil {
		return nil, fmt.Errorf("kubernetes integration is disabled")
	}
	resolve := func(in ResourceInput) (string, error) {
		if in.Namespace == "" {
			in.Namespace = ns
		}
		if in.Namespace != ns {
			return "", fmt.Errorf("namespace outside policy")
		}
		if in.Name == "" {
			return "", fmt.Errorf("name is required")
		}
		return in.Namespace, nil
	}
	depY, e := utils.InferTool("query_kubernetes_resource_yaml", "Read a Deployment YAML snapshot for incident diagnosis.", func(ctx context.Context, in ResourceInput) (string, error) {
		n, e := resolve(in)
		if e != nil {
			return "", e
		}
		b, e := repo.(interface {
			GetDeploymentYAML(context.Context, string, string) ([]byte, error)
		}).GetDeploymentYAML(ctx, n, in.Name)
		if e != nil {
			return "", e
		}
		return string(b), nil
	})
	if e != nil {
		return nil, e
	}
	podY, e := utils.InferTool("query_kubernetes_pod_owner", "Read the controlling owner of a Pod.", func(ctx context.Context, in ResourceInput) (string, error) {
		n, e := resolve(in)
		if e != nil {
			return "", e
		}
		o, e := repo.(interface {
			GetPodOwner(context.Context, string, string) (kuberepo.OwnerInfo, error)
		}).GetPodOwner(ctx, n, in.Name)
		if e != nil {
			return "", e
		}
		b, _ := json.Marshal(o)
		return string(b), nil
	})
	if e != nil {
		return nil, e
	}
	cond, e := utils.InferTool("query_kubernetes_resource_conditions", "Read Deployment conditions.", func(ctx context.Context, in ResourceInput) (string, error) {
		n, e := resolve(in)
		if e != nil {
			return "", e
		}
		v, e := repo.(interface {
			GetDeploymentConditions(context.Context, string, string) ([]kuberepo.ConditionInfo, error)
		}).GetDeploymentConditions(ctx, n, in.Name)
		if e != nil {
			return "", e
		}
		b, _ := json.Marshal(v)
		return string(b), nil
	})
	if e != nil {
		return nil, e
	}
	rev, e := utils.InferTool("query_kubernetes_rollout_history", "Read Deployment rollout revisions.", func(ctx context.Context, in ResourceInput) (string, error) {
		n, e := resolve(in)
		if e != nil {
			return "", e
		}
		v, e := repo.(interface {
			ListRolloutRevisions(context.Context, string, string) ([]kuberepo.RevisionInfo, error)
		}).ListRolloutRevisions(ctx, n, in.Name)
		if e != nil {
			return "", e
		}
		b, _ := json.Marshal(v)
		return string(b), nil
	})
	if e != nil {
		return nil, e
	}
	return []tool.BaseTool{depY, podY, cond, rev}, nil
}
