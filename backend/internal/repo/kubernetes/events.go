package kubernetes

import (
	"context"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type EventInfo struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	Type      string `json:"type"`
	Reason    string `json:"reason"`
	Message   string `json:"message"`
	Object    string `json:"object"`
	Timestamp string `json:"timestamp"`
}

func (r *repository) ListEvents(ctx context.Context, namespace string) ([]EventInfo, error) {
	items, err := r.client.CoreV1().Events(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	result := make([]EventInfo, 0, len(items.Items))
	for _, event := range items.Items {
		ts := event.LastTimestamp.Time
		if ts.IsZero() {
			ts = event.EventTime.Time
		}
		result = append(result, EventInfo{Namespace: event.Namespace, Name: event.Name, Type: event.Type, Reason: event.Reason, Message: event.Message, Object: event.InvolvedObject.Kind + "/" + event.InvolvedObject.Name, Timestamp: ts.Format("2006-01-02T15:04:05Z07:00")})
	}
	return result, nil
}
