package kubernetes

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type PodInfo struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	Status    string `json:"status"`
	Ready     bool   `json:"ready"`
	Restarts  int32  `json:"restarts"`
	Reason    string `json:"reason,omitempty"`
	Message   string `json:"message,omitempty"`
}

func (r *repository) ListPods(ctx context.Context, namespace string) ([]PodInfo, error) {
	items, err := r.client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	result := make([]PodInfo, 0, len(items.Items))
	for _, pod := range items.Items {
		ready := true
		var restarts int32
		for _, status := range pod.Status.ContainerStatuses {
			restarts += status.RestartCount
			for _, condition := range pod.Status.Conditions {
				if condition.Type == "Ready" && condition.Status != "True" {
					ready = false
				}
			}
		}
		result = append(result, PodInfo{Namespace: pod.Namespace, Name: pod.Name, Status: string(pod.Status.Phase), Ready: ready, Restarts: restarts, Reason: pod.Status.Reason, Message: pod.Status.Message})
	}
	return result, nil
}

func (r *repository) DeletePod(ctx context.Context, namespace, name string) error {
	if err := r.DeleteManagedPod(ctx, namespace, name); err != nil {
		return err
	}
	return nil
}

func (r *repository) IsManagedPod(ctx context.Context, namespace, name string) (bool, error) {
	pod, err := r.client.CoreV1().Pods(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return false, err
	}
	managed := false
	for _, owner := range pod.OwnerReferences {
		if owner.Kind == "ReplicaSet" && owner.Controller != nil && *owner.Controller {
			managed = true
			break
		}
	}
	return managed, nil
}

func (r *repository) DeleteManagedPod(ctx context.Context, namespace, name string) error {
	managed, err := r.IsManagedPod(ctx, namespace, name)
	if err != nil {
		return err
	}
	if !managed {
		return fmt.Errorf("pod %s is not controlled by a ReplicaSet", name)
	}
	return r.client.CoreV1().Pods(namespace).Delete(ctx, name, metav1.DeleteOptions{})
}
