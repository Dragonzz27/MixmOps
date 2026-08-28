package kubernetes

import (
	"context"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"strconv"
)

type ServiceInfo struct {
	Namespace string            `json:"namespace"`
	Name      string            `json:"name"`
	Type      string            `json:"type"`
	ClusterIP string            `json:"cluster_ip"`
	Ports     []string          `json:"ports"`
	Selector  map[string]string `json:"selector"`
}

func (r *repository) ListServices(ctx context.Context, namespace string) ([]ServiceInfo, error) {
	items, err := r.client.CoreV1().Services(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	out := make([]ServiceInfo, 0, len(items.Items))
	for _, svc := range items.Items {
		ports := make([]string, 0, len(svc.Spec.Ports))
		for _, p := range svc.Spec.Ports {
			ports = append(ports, p.Name+":"+strconv.FormatInt(int64(p.Port), 10))
		}
		out = append(out, ServiceInfo{Namespace: svc.Namespace, Name: svc.Name, Type: string(svc.Spec.Type), ClusterIP: svc.Spec.ClusterIP, Ports: ports, Selector: svc.Spec.Selector})
	}
	return out, nil
}
