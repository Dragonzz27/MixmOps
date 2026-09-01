package kubernetes

import (
	"context"
	"encoding/json"
	"fmt"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func (r *repository) GetResourceVersion(ctx context.Context, ns, kind, name string) (string, error) {
	switch kind {
	case "pod":
		p, err := r.client.CoreV1().Pods(ns).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return "", err
		}
		return p.ResourceVersion, nil
	case "deployment":
		d, err := r.client.AppsV1().Deployments(ns).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return "", err
		}
		return d.ResourceVersion, nil
	default:
		return "", fmt.Errorf("unsupported resource kind %q", kind)
	}
}

func (r *repository) GetDeploymentYAML(ctx context.Context, ns, name string) ([]byte, error) {
	d, e := r.client.AppsV1().Deployments(ns).Get(ctx, name, metav1.GetOptions{})
	if e != nil {
		return nil, e
	}
	return json.Marshal(d)
}
func (r *repository) GetPodYAML(ctx context.Context, ns, name string) ([]byte, error) {
	p, e := r.client.CoreV1().Pods(ns).Get(ctx, name, metav1.GetOptions{})
	if e != nil {
		return nil, e
	}
	return json.Marshal(p)
}
func (r *repository) GetDeploymentConditions(ctx context.Context, ns, name string) ([]ConditionInfo, error) {
	d, e := r.client.AppsV1().Deployments(ns).Get(ctx, name, metav1.GetOptions{})
	if e != nil {
		return nil, e
	}
	out := make([]ConditionInfo, 0, len(d.Status.Conditions))
	for _, c := range d.Status.Conditions {
		out = append(out, ConditionInfo{Type: string(c.Type), Status: string(c.Status), Reason: c.Reason, Message: c.Message})
	}
	return out, nil
}
func (r *repository) ListRolloutRevisions(ctx context.Context, ns, name string) ([]RevisionInfo, error) {
	rs, e := r.client.AppsV1().ReplicaSets(ns).List(ctx, metav1.ListOptions{})
	if e != nil {
		return nil, e
	}
	out := []RevisionInfo{}
	for _, x := range rs.Items {
		for _, o := range x.OwnerReferences {
			if o.Kind == "Deployment" && o.Name == name {
				rev := int64(0)
				if v := x.Annotations["deployment.kubernetes.io/revision"]; v != "" {
					fmt.Sscan(v, &rev)
				}
				out = append(out, RevisionInfo{Revision: rev, CreatedAt: x.CreationTimestamp.UTC().Format("2006-01-02T15:04:05Z")})
			}
		}
	}
	return out, nil
}
func (r *repository) GetPodOwner(ctx context.Context, ns, name string) (OwnerInfo, error) {
	p, e := r.client.CoreV1().Pods(ns).Get(ctx, name, metav1.GetOptions{})
	if e != nil {
		return OwnerInfo{}, e
	}
	for _, o := range p.OwnerReferences {
		if o.Controller != nil && *o.Controller {
			return OwnerInfo{Kind: o.Kind, Name: o.Name, UID: string(o.UID)}, nil
		}
	}
	return OwnerInfo{}, nil
}
func (r *repository) RolloutRestartDeployment(ctx context.Context, ns, name string) error {
	patch := []byte(`{"spec":{"template":{"metadata":{"annotations":{"autoops.io/restartedAt":"now"}}}}}`)
	_, e := r.client.AppsV1().Deployments(ns).Patch(ctx, name, types.StrategicMergePatchType, patch, metav1.PatchOptions{})
	return e
}
func (r *repository) RollbackDeployment(ctx context.Context, ns, name string, revision int64) error {
	rs, e := r.client.AppsV1().ReplicaSets(ns).List(ctx, metav1.ListOptions{})
	if e != nil {
		return e
	}
	for _, x := range rs.Items {
		if x.Annotations["deployment.kubernetes.io/revision"] == fmt.Sprint(revision) {
			d, e := r.client.AppsV1().Deployments(ns).Get(ctx, name, metav1.GetOptions{})
			if e != nil {
				return e
			}
			d.Spec.Template = x.Spec.Template
			_, e = r.client.AppsV1().Deployments(ns).Update(ctx, d, metav1.UpdateOptions{})
			return e
		}
	}
	return fmt.Errorf("revision %d not found", revision)
}
func (r *repository) ScaleDeployment(ctx context.Context, ns, name string, replicas int32) error {
	d, e := r.client.AppsV1().Deployments(ns).Get(ctx, name, metav1.GetOptions{})
	if e != nil {
		return e
	}
	d.Spec.Replicas = &replicas
	_, e = r.client.AppsV1().Deployments(ns).Update(ctx, d, metav1.UpdateOptions{})
	return e
}
