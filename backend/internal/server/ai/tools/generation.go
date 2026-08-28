package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
)

type ManifestInput struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Image     string `json:"image"`
	Replicas  int    `json:"replicas"`
	Port      int    `json:"port"`
}
type ScriptInput struct {
	Goal      string `json:"goal"`
	Namespace string `json:"namespace"`
	Target    string `json:"target"`
}

func NewGenerationTools() ([]tool.BaseTool, error) {
	manifest, err := utils.InferTool("generate_kubernetes_manifest", "Generate a Kubernetes Deployment and Service YAML without applying it to a cluster.", func(_ context.Context, in ManifestInput) (string, error) {
		if strings.TrimSpace(in.Name) == "" || strings.TrimSpace(in.Image) == "" {
			return "", fmt.Errorf("name and image are required")
		}
		if in.Namespace == "" {
			in.Namespace = "autoops-test"
		}
		if in.Replicas <= 0 {
			in.Replicas = 1
		}
		if in.Port <= 0 {
			in.Port = 8080
		}
		return fmt.Sprintf("apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: %s\n  namespace: %s\nspec:\n  replicas: %d\n  selector:\n    matchLabels:\n      app: %s\n  template:\n    metadata:\n      labels:\n        app: %s\n    spec:\n      containers:\n        - name: %s\n          image: %s\n          ports:\n            - containerPort: %d\n---\napiVersion: v1\nkind: Service\nmetadata:\n  name: %s\n  namespace: %s\nspec:\n  selector:\n    app: %s\n  ports:\n    - port: %d\n      targetPort: %d\n", in.Name, in.Namespace, in.Replicas, in.Name, in.Name, in.Name, in.Image, in.Port, in.Name, in.Namespace, in.Name, in.Port, in.Port), nil
	})
	if err != nil {
		return nil, err
	}
	script, err := utils.InferTool("generate_operation_script", "Generate a reviewed shell command template for a daily operations work order; never execute it.", func(_ context.Context, in ScriptInput) (string, error) {
		if strings.TrimSpace(in.Goal) == "" {
			return "", fmt.Errorf("goal is required")
		}
		return fmt.Sprintf("#!/usr/bin/env bash\nset -euo pipefail\n# Goal: %s\n# Namespace: %s\n# Target: %s\n# Review this script before execution.\necho 'DRY-RUN: %s'\n", in.Goal, in.Namespace, in.Target, in.Goal), nil
	})
	if err != nil {
		return nil, err
	}
	return []tool.BaseTool{manifest, script}, nil
}
