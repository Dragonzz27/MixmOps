package kubernetes

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"AutoOps/pkg/config"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// KubernetesRepository is the read-only view of the test cluster used by agents.
type KubernetesRepository interface {
	ListPods(context.Context, string) ([]PodInfo, error)
	ListDeployments(context.Context, string) ([]DeploymentInfo, error)
	ListServices(context.Context, string) ([]ServiceInfo, error)
	ListEvents(context.Context, string) ([]EventInfo, error)
	GetPodLogs(context.Context, string, string, string, LogOptions) (string, error)
}

type PodActions interface {
	DeletePod(context.Context, string, string) error
	DeleteManagedPod(context.Context, string, string) error
	IsManagedPod(context.Context, string, string) (bool, error)
}

type ConditionInfo struct {
	Type    string `json:"type"`
	Status  string `json:"status"`
	Reason  string `json:"reason,omitempty"`
	Message string `json:"message,omitempty"`
}
type RevisionInfo struct {
	Revision  int64  `json:"revision"`
	Image     string `json:"image,omitempty"`
	CreatedAt string `json:"created_at,omitempty"`
}
type OwnerInfo struct {
	Kind string `json:"kind"`
	Name string `json:"name"`
	UID  string `json:"uid,omitempty"`
}
type KubernetesObservationRepository interface {
	GetDeploymentYAML(context.Context, string, string) ([]byte, error)
	GetPodYAML(context.Context, string, string) ([]byte, error)
	GetDeploymentConditions(context.Context, string, string) ([]ConditionInfo, error)
	ListRolloutRevisions(context.Context, string, string) ([]RevisionInfo, error)
	GetPodOwner(context.Context, string, string) (OwnerInfo, error)
}
type IncidentActions interface {
	DeleteManagedPod(context.Context, string, string) error
	RolloutRestartDeployment(context.Context, string, string) error
	RollbackDeployment(context.Context, string, string, int64) error
	ScaleDeployment(context.Context, string, string, int32) error
}

type repository struct{ client kubernetes.Interface }

func NewRepository(cfg config.KubernetesConfig) (KubernetesRepository, error) {
	if !cfg.Enabled {
		return nil, nil
	}
	var restCfg *rest.Config
	var err error
	if cfg.InCluster {
		restCfg, err = rest.InClusterConfig()
	} else {
		path := cfg.Kubeconfig
		if path == "" {
			path = os.Getenv("KUBECONFIG")
		}
		if path == "" {
			home, homeErr := os.UserHomeDir()
			if homeErr != nil {
				return nil, fmt.Errorf("resolve kubeconfig home: %w", homeErr)
			}
			path = filepath.Join(home, ".kube", "config")
		}
		path = expandHome(path)
		loading := clientcmd.NewDefaultClientConfigLoadingRules()
		loading.ExplicitPath = path
		overrides := &clientcmd.ConfigOverrides{}
		if cfg.Context != "" {
			overrides.CurrentContext = cfg.Context
		}
		restCfg, err = clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loading, overrides).ClientConfig()
	}
	if err != nil {
		return nil, fmt.Errorf("build Kubernetes client config: %w", err)
	}
	client, err := kubernetes.NewForConfig(restCfg)
	if err != nil {
		return nil, fmt.Errorf("create Kubernetes client: %w", err)
	}
	return &repository{client: client}, nil
}

func expandHome(path string) string {
	if strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, strings.TrimPrefix(path, "~/"))
		}
	}
	return path
}

var _ KubernetesRepository = (*repository)(nil)
