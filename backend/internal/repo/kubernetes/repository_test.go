package kubernetes

import (
	"context"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestListPodsAndDeployments(t *testing.T) {
	client := fake.NewSimpleClientset(
		&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: "autoops-test"}, Status: corev1.PodStatus{Phase: corev1.PodRunning, ContainerStatuses: []corev1.ContainerStatus{{RestartCount: 2, Ready: true}}}},
		&appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: "autoops-test"}, Spec: appsv1.DeploymentSpec{Replicas: int32ptr(1)}, Status: appsv1.DeploymentStatus{Replicas: 1, AvailableReplicas: 1, UpdatedReplicas: 1}},
	)
	r := &repository{client: client}
	pods, err := r.ListPods(context.Background(), "autoops-test")
	if err != nil || len(pods) != 1 || pods[0].Restarts != 2 || !pods[0].Ready {
		t.Fatalf("unexpected pods: %#v, %v", pods, err)
	}
	deployments, err := r.ListDeployments(context.Background(), "autoops-test")
	if err != nil || len(deployments) != 1 || !deployments[0].Ready {
		t.Fatalf("unexpected deployments: %#v, %v", deployments, err)
	}
}

func TestListEvents(t *testing.T) {
	client := fake.NewSimpleClientset(&corev1.Event{ObjectMeta: metav1.ObjectMeta{Name: "failed", Namespace: "autoops-test"}, Type: "Warning", Reason: "Failed", Message: "image pull failed", InvolvedObject: corev1.ObjectReference{Kind: "Pod", Name: "api"}})
	r := &repository{client: client}
	events, err := r.ListEvents(context.Background(), "autoops-test")
	if err != nil || len(events) != 1 || events[0].Reason != "Failed" {
		t.Fatalf("unexpected events: %#v, %v", events, err)
	}
}

func int32ptr(v int32) *int32 { return &v }
