package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	kuberepo "AutoOps/internal/repo/kubernetes"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
)

const maxKubernetesOutput = 12000

type KubernetesResourcesInput struct {
	Namespace string `json:"namespace,omitempty" jsonschema:"description=Namespace to inspect; defaults to the configured test namespace"`
	Resource  string `json:"resource,omitempty" jsonschema:"description=Resource type: pods, deployments, or all"`
}
type KubernetesEventsInput struct {
	Namespace string `json:"namespace,omitempty"`
}
type KubernetesLogsInput struct {
	Namespace    string `json:"namespace,omitempty"`
	Pod          string `json:"pod" jsonschema:"description=Pod name"`
	Container    string `json:"container,omitempty"`
	TailLines    int64  `json:"tail_lines,omitempty"`
	SinceSeconds int64  `json:"since_seconds,omitempty"`
}

func NewKubernetesTools(repo kuberepo.KubernetesRepository, defaultNamespace string) ([]tool.BaseTool, error) {
	if repo == nil {
		return nil, fmt.Errorf("kubernetes integration is disabled")
	}
	resources, err := utils.InferTool("query_kubernetes_resources", "Read Pod and Deployment status from the local Minikube cluster.", func(ctx context.Context, in KubernetesResourcesInput) (string, error) {
		ns, err := resolveNamespace(in.Namespace, defaultNamespace)
		if err != nil {
			return "", err
		}
		ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		resource := strings.ToLower(in.Resource)
		if resource == "" {
			resource = "all"
		}
		out := map[string]any{"resource_type": resource, "namespace": ns}
		if resource == "pods" || resource == "all" {
			v, err := repo.ListPods(ctx, ns)
			if err != nil {
				return "", err
			}
			out["pods"] = v
		}
		if resource == "deployments" || resource == "all" {
			v, err := repo.ListDeployments(ctx, ns)
			if err != nil {
				return "", err
			}
			out["deployments"] = v
		}
		if resource != "pods" && resource != "deployments" && resource != "all" {
			return "", fmt.Errorf("unsupported Kubernetes resource %q", in.Resource)
		}
		return marshalLimited(out)
	})
	if err != nil {
		return nil, err
	}
	events, err := utils.InferTool("query_kubernetes_events", "Read recent Kubernetes Events from the local Minikube cluster.", func(ctx context.Context, in KubernetesEventsInput) (string, error) {
		ns, err := resolveNamespace(in.Namespace, defaultNamespace)
		if err != nil {
			return "", err
		}
		ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		v, err := repo.ListEvents(ctx, ns)
		if err != nil {
			return "", err
		}
		return marshalLimited(map[string]any{"resource_type": "events", "namespace": ns, "events": v})
	})
	if err != nil {
		return nil, err
	}
	logs, err := utils.InferTool("query_kubernetes_logs", "Read bounded logs from a Pod in the local Minikube cluster.", func(ctx context.Context, in KubernetesLogsInput) (string, error) {
		ns, err := resolveNamespace(in.Namespace, defaultNamespace)
		if err != nil {
			return "", err
		}
		if strings.TrimSpace(in.Pod) == "" {
			return "", fmt.Errorf("pod is required")
		}
		tail := in.TailLines
		if tail <= 0 {
			tail = 200
		}
		if tail > 1000 {
			tail = 1000
		}
		since := in.SinceSeconds
		var sincePtr *int64
		if since > 0 {
			if since > 86400 {
				since = 86400
			}
			sincePtr = &since
		}
		ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
		defer cancel()
		logs, err := repo.GetPodLogs(ctx, ns, in.Pod, in.Container, kuberepo.LogOptions{TailLines: &tail, SinceSeconds: sincePtr, LimitBytes: 100000})
		if err != nil {
			return "", err
		}
		return marshalLimited(map[string]any{"resource_type": "pod_logs", "namespace": ns, "pod": in.Pod, "container": in.Container, "logs": logs})
	})
	if err != nil {
		return nil, err
	}
	return []tool.BaseTool{resources, events, logs}, nil
}

func resolveNamespace(value, fallback string) (string, error) {
	if strings.TrimSpace(value) == "" {
		if fallback == "" {
			return "", fmt.Errorf("Kubernetes namespace is not configured")
		}
		return fallback, nil
	}
	if fallback != "" && value != fallback {
		return "", fmt.Errorf("namespace %q is outside configured namespace %q", value, fallback)
	}
	return value, nil
}
func marshalLimited(value any) (string, error) {
	b, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	if len(b) > maxKubernetesOutput {
		b = b[:maxKubernetesOutput]
		return string(b) + "...<truncated>", nil
	}
	return string(b), nil
}
