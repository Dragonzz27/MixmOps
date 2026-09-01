package handler

import (
	kuberepo "AutoOps/internal/repo/kubernetes"
	"AutoOps/internal/server/cases"
	incidentaction "AutoOps/internal/server/incident/action"
	"AutoOps/pkg/config"
	"github.com/gin-gonic/gin"
	"net/http"
)

type IncidentActionHandler struct {
	cases  *cases.Service
	kube   kuberepo.KubernetesRepository
	cfg    *config.Config
	broker *incidentaction.Broker
}

func NewIncidentActionHandler(cs *cases.Service, k kubeRepoAlias, cfg *config.Config) *IncidentActionHandler {
	return &IncidentActionHandler{cases: cs, kube: k, cfg: cfg, broker: incidentaction.NewBroker(cs, k, cfg)}
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
		x, e := h.broker.Propose(c, a.IncidentID, a)
		if e != nil {
			c.JSON(400, gin.H{"message": e.Error()})
			return
		}
		c.JSON(201, x)
	}
}
func (h *IncidentActionHandler) Confirm() gin.HandlerFunc {
	return func(c *gin.Context) {
		result, e := h.broker.Confirm(c, c.Param("id"), c.Param("action_id"), c.GetHeader("X-Operator"))
		if e != nil {
			c.JSON(http.StatusBadGateway, gin.H{"message": "action failed", "error": e.Error()})
			return
		}
		c.JSON(200, result)
	}
}
func (h *IncidentActionHandler) Reject() gin.HandlerFunc {
	return func(c *gin.Context) {
		e := h.broker.Reject(c, c.Param("id"), c.Param("action_id"), c.GetHeader("X-Operator"))
		if e != nil {
			c.JSON(400, gin.H{"message": e.Error()})
			return
		}
		c.JSON(200, gin.H{"status": "rejected"})
	}
}
