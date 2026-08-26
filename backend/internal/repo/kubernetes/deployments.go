package kubernetes

import (
	"context"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type DeploymentInfo struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	Replicas  int32  `json:"replicas"`
	Available int32  `json:"available"`
	Ready     bool   `json:"ready"`
	Updated   int32  `json:"updated"`
}

func (r *repository) ListDeployments(ctx context.Context, namespace string) ([]DeploymentInfo, error) {
	items, err := r.client.AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	result := make([]DeploymentInfo, 0, len(items.Items))
	for _, d := range items.Items {
		expected := int32(1)
		if d.Spec.Replicas != nil {
			expected = *d.Spec.Replicas
		}
		result = append(result, DeploymentInfo{Namespace: d.Namespace, Name: d.Name, Replicas: d.Status.Replicas, Available: d.Status.AvailableReplicas, Ready: d.Status.AvailableReplicas == expected, Updated: d.Status.UpdatedReplicas})
	}
	return result, nil
}
