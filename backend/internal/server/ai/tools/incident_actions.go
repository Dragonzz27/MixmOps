package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"strings"
)

type IncidentActionProposalInput struct {
	Action       string         `json:"action"`
	Namespace    string         `json:"namespace"`
	Target       string         `json:"target"`
	Reason       string         `json:"reason"`
	Risk         string         `json:"risk"`
	Rollback     string         `json:"rollback"`
	Verification []string       `json:"verification"`
	Payload      map[string]any `json:"payload,omitempty"`
}

func NewIncidentActionProposalTool(defaultNamespace string) (tool.InvokableTool, error) {
	return utils.InferTool("propose_incident_action", "提出需要用户明确确认的受控 Incident 修复动作；此工具不会执行 Kubernetes 写操作。允许 delete_managed_pod、rollout_restart_deployment、rollback_deployment、scale_deployment。", func(_ context.Context, in IncidentActionProposalInput) (string, error) {
		if in.Namespace == "" {
			in.Namespace = defaultNamespace
		}
		if in.Namespace != defaultNamespace {
			return "", fmt.Errorf("namespace outside policy")
		}
		allowed := map[string]bool{"delete_managed_pod": true, "rollout_restart_deployment": true, "rollback_deployment": true, "scale_deployment": true}
		if !allowed[in.Action] {
			return "", fmt.Errorf("unsupported action %q", in.Action)
		}
		if strings.TrimSpace(in.Target) == "" || strings.TrimSpace(in.Reason) == "" {
			return "", fmt.Errorf("target and reason are required")
		}
		b, _ := json.Marshal(map[string]any{"status": "pending_confirmation", "action": in.Action, "namespace": in.Namespace, "target": in.Target, "reason": in.Reason, "risk": in.Risk, "rollback": in.Rollback, "verification": in.Verification, "payload": in.Payload})
		return string(b), nil
	})
}
