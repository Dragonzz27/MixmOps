package kubernetes

import (
	"context"
	"fmt"
	"io"
	"strings"

	corev1 "k8s.io/api/core/v1"
)

type LogOptions struct {
	TailLines    *int64
	SinceSeconds *int64
	LimitBytes   int64
}

func (r *repository) GetPodLogs(ctx context.Context, namespace, pod, container string, opts LogOptions) (string, error) {
	request := r.client.CoreV1().Pods(namespace).GetLogs(pod, &corev1.PodLogOptions{Container: container, TailLines: opts.TailLines, SinceSeconds: opts.SinceSeconds, LimitBytes: func() *int64 {
		if opts.LimitBytes > 0 {
			return &opts.LimitBytes
		}
		return nil
	}()})
	stream, err := request.Stream(ctx)
	if err != nil {
		return "", fmt.Errorf("open pod log stream: %w", err)
	}
	defer stream.Close()
	b, err := io.ReadAll(stream)
	if err != nil {
		return "", fmt.Errorf("read pod logs: %w", err)
	}
	return strings.TrimSpace(string(b)), nil
}
