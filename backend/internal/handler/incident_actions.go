package handler

import (
	kuberepo "AutoOps/internal/repo/kubernetes"
	"AutoOps/internal/server/cases"
	"AutoOps/pkg/config"
	"context"
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"net/http"
	"strings"
)

type IncidentActionHandler struct {
	cases *cases.Service
	kube  kuberepo.KubernetesRepository
	cfg   *config.Config
}

func NewIncidentActionHandler(cs *cases.Service, k kubeRepoAlias, cfg *config.Config) *IncidentActionHandler {
	return &IncidentActionHandler{cases: cs, kube: k, cfg: cfg}
}

type kubeRepoAlias = kuberepo.KubernetesRepository

func (h *IncidentActionHandler) List() gin.HandlerFunc {
	return func(c *gin.Context) {
		a, e := h.cases.ListIncidentActions(c, c.Param("id"))
		if e != nil {
			c.JSON(500, gin.H{"message": e.Error()})
			return
		}
		c.JSON(200, gin.H{"actions": a})
	}
}
func (h *IncidentActionHandler) Propose() gin.HandlerFunc {
	return func(c *gin.Context) {
		var a cases.IncidentAction
		if c.ShouldBindJSON(&a) != nil {
			c.JSON(400, gin.H{"message": "invalid action"})
			return
		}
		a.IncidentID = c.Param("id")
		if a.Namespace == "" {
			a.Namespace = h.cfg.Kubernetes.Namespace
		}
		x, e := h.cases.ProposeIncidentAction(c, a.IncidentID, a)
		if e != nil {
			c.JSON(400, gin.H{"message": e.Error()})
			return
		}
		c.JSON(201, x)
	}
}
func (h *IncidentActionHandler) Confirm() gin.HandlerFunc {
	return func(c *gin.Context) {
		a, e := h.cases.ConfirmIncidentAction(c, c.Param("action_id"), c.GetHeader("X-Operator"))
		if e != nil {
			c.JSON(400, gin.H{"message": e.Error()})
			return
		}
		if a.Status != "confirmed" {
			c.JSON(400, gin.H{"message": "action is not confirmed"})
			return
		}
		if !h.cfg.Incident.ActionsEnabled || !allowed(h.cfg.Incident.AllowedActions, a.ActionType) {
			c.JSON(403, gin.H{"message": "action disabled by policy"})
			return
		}
		if h.kube == nil {
			c.JSON(503, gin.H{"message": "Kubernetes unavailable"})
			return
		}
		var err error
		_ = h.cases.SetIncidentActionResult(c, a.ID, "executing", "", "")
		ctx, cancel := context.WithTimeout(c, 60_000_000_000)
		defer cancel()
		switch a.ActionType {
		case "delete_managed_pod":
			err = h.kube.(kuberepo.PodActions).DeleteManagedPod(ctx, a.Namespace, a.Target)
		case "rollout_restart_deployment":
			err = h.kube.(interface {
				RolloutRestartDeployment(context.Context, string, string) error
			}).RolloutRestartDeployment(ctx, a.Namespace, a.Target)
		case "rollback_deployment":
			var p struct {
				Revision int64 `json:"revision"`
			}
			_ = json.Unmarshal(a.Payload, &p)
			err = h.kube.(interface {
				RollbackDeployment(context.Context, string, string, int64) error
			}).RollbackDeployment(ctx, a.Namespace, a.Target, p.Revision)
		case "scale_deployment":
			var p struct {
				Replicas int32 `json:"replicas"`
			}
			_ = json.Unmarshal(a.Payload, &p)
			if p.Replicas < 1 || p.Replicas > h.cfg.Incident.MaxReplicas {
				err = fmt.Errorf("replicas outside policy")
			} else {
				err = h.kube.(interface {
					ScaleDeployment(context.Context, string, string, int32) error
				}).ScaleDeployment(ctx, a.Namespace, a.Target, p.Replicas)
			}
		}
		if err != nil {
			_ = h.cases.SetIncidentActionResult(c, a.ID, "failed", "", err.Error())
			c.JSON(http.StatusBadGateway, gin.H{"message": "action failed", "error": err.Error()})
			return
		}
		_ = h.cases.SetIncidentActionResult(c, a.ID, "succeeded", "action executed", "")
		c.JSON(200, gin.H{"status": "succeeded", "action": a})
	}
}
func (h *IncidentActionHandler) Reject() gin.HandlerFunc {
	return func(c *gin.Context) {
		a, e := h.cases.RejectIncidentAction(c, c.Param("action_id"), c.GetHeader("X-Operator"))
		if e != nil {
			c.JSON(400, gin.H{"message": e.Error()})
			return
		}
		c.JSON(200, a)
	}
}
func allowed(list, action string) bool {
	for _, x := range strings.Split(list, ",") {
		if strings.TrimSpace(x) == action {
			return true
		}
	}
	return false
}
