package handler

import (
	"strings"

	"AutoOps/internal/repo/kubernetes"
	"AutoOps/internal/server/background"
	"AutoOps/internal/server/cases"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type ModeHandler struct {
	background *background.Service
	cases      *cases.Service
	kube       kubernetes.KubernetesRepository
}

func NewModeHandler(bg *background.Service, cs *cases.Service, kube kubernetes.KubernetesRepository) *ModeHandler {
	return &ModeHandler{background: bg, cases: cs, kube: kube}
}
func (h *ModeHandler) AlertmanagerWebhook() gin.HandlerFunc {
	return func(c *gin.Context) {
		var body struct {
			Alerts []struct {
				Fingerprint string            `json:"fingerprint"`
				Labels      map[string]string `json:"labels"`
				Annotations map[string]string `json:"annotations"`
			} `json:"alerts"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(400, gin.H{"message": "invalid Alertmanager payload"})
			return
		}
		created := 0
		for _, a := range body.Alerts {
			name := a.Labels["alertname"]
			ns := a.Labels["namespace"]
			if ns == "" {
				ns = "autoops-test"
			}
			_, ok, err := h.background.CreateFromAlert(c.Request.Context(), background.Alert{Fingerprint: a.Fingerprint, Name: name, Namespace: ns, Labels: a.Labels, Annotations: a.Annotations})
			if err != nil {
				c.JSON(500, gin.H{"message": err.Error()})
				return
			}
			if ok {
				created++
			}
		}
		c.JSON(202, gin.H{"created": created})
	}
}
func (h *ModeHandler) Tasks() gin.HandlerFunc {
	return func(c *gin.Context) {
		tasks, err := h.background.List(c.Request.Context())
		if err != nil {
			c.JSON(500, gin.H{"message": err.Error()})
			return
		}
		c.JSON(200, gin.H{"tasks": tasks})
	}
}
func (h *ModeHandler) Task() gin.HandlerFunc {
	return func(c *gin.Context) {
		t, err := h.background.Get(c.Request.Context(), c.Param("id"))
		if err != nil {
			c.JSON(404, gin.H{"message": err.Error()})
			return
		}
		c.JSON(200, t)
	}
}
func (h *ModeHandler) Approve() gin.HandlerFunc {
	return func(c *gin.Context) {
		t, err := h.background.Approve(c.Request.Context(), c.Param("id"), c.GetHeader("X-Operator"))
		if err != nil {
			c.JSON(400, gin.H{"message": err.Error()})
			return
		}
		if t.TargetPod != "" && h.kube != nil {
			actions, ok := h.kube.(kubernetes.PodActions)
			if ok {
				if err := actions.DeletePod(c.Request.Context(), t.Namespace, t.TargetPod); err != nil {
					c.JSON(502, gin.H{"message": "pod isolation failed", "error": err.Error()})
					return
				}
			}
		}
		t, _ = h.background.MarkWaitingHuman(c.Request.Context(), t.ID)
		c.JSON(200, t)
	}
}
func (h *ModeHandler) Reject() gin.HandlerFunc {
	return func(c *gin.Context) {
		t, err := h.background.Reject(c.Request.Context(), c.Param("id"), c.GetHeader("X-Operator"))
		if err != nil {
			c.JSON(400, gin.H{"message": err.Error()})
			return
		}
		c.JSON(200, t)
	}
}
func (h *ModeHandler) Incidents() gin.HandlerFunc {
	return func(c *gin.Context) {
		items, err := h.cases.ListIncidents(c.Request.Context())
		if err != nil {
			c.JSON(500, gin.H{"message": err.Error()})
			return
		}
		c.JSON(200, gin.H{"incidents": items})
	}
}
func (h *ModeHandler) Incident() gin.HandlerFunc {
	return func(c *gin.Context) {
		item, err := h.cases.GetIncident(c.Request.Context(), c.Param("id"))
		if err != nil {
			c.JSON(404, gin.H{"message": err.Error()})
			return
		}
		c.JSON(200, item)
	}
}
func (h *ModeHandler) CreateIncident() gin.HandlerFunc {
	return func(c *gin.Context) {
		var in struct {
			Fingerprint string `json:"alert_fingerprint"`
			AlertName   string `json:"alert_name"`
		}
		if c.ShouldBindJSON(&in) != nil || strings.TrimSpace(in.AlertName) == "" {
			c.JSON(400, gin.H{"message": "alert_name is required"})
			return
		}
		item, err := h.cases.CreateIncident(c.Request.Context(), in.Fingerprint, in.AlertName, gin.H{"alert_name": in.AlertName, "fingerprint": in.Fingerprint})
		if err != nil {
			c.JSON(500, gin.H{"message": err.Error()})
			return
		}
		c.JSON(201, item)
	}
}
func (h *ModeHandler) WorkOrders() gin.HandlerFunc {
	return func(c *gin.Context) {
		items, err := h.cases.ListWorkOrders(c)
		if err != nil {
			c.JSON(500, gin.H{"message": err.Error()})
			return
		}
		c.JSON(200, gin.H{"work_orders": items})
	}
}
func (h *ModeHandler) CreateWorkOrder() gin.HandlerFunc {
	return func(c *gin.Context) {
		var w cases.WorkOrder
		if c.ShouldBindJSON(&w) != nil {
			c.JSON(400, gin.H{"message": "invalid work order"})
			return
		}
		item, err := h.cases.CreateWorkOrder(c.Request.Context(), w)
		if err != nil {
			c.JSON(400, gin.H{"message": err.Error()})
			return
		}
		c.JSON(201, item)
	}
}
func OwnerID() string { return uuid.New().String() }
