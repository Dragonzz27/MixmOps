package handler

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	kuberepo "AutoOps/internal/repo/kubernetes"
	"AutoOps/internal/server/ai/tools"
	"AutoOps/pkg/config"
	"github.com/gin-gonic/gin"
)

type ObservabilityHandler struct {
	cfg  *config.Config
	kube kuberepo.KubernetesRepository
}

func NewObservabilityHandler(cfg *config.Config, kube kuberepo.KubernetesRepository) *ObservabilityHandler {
	return &ObservabilityHandler{cfg: cfg, kube: kube}
}

func (h *ObservabilityHandler) Alerts() gin.HandlerFunc {
	return func(c *gin.Context) {
		result, err := tools.QueryPrometheusAlerts(h.cfg.GetPrometheusURL())
		if err != nil {
			c.JSON(http.StatusBadGateway, result)
			return
		}
		c.JSON(http.StatusOK, result)
	}
}

func (h *ObservabilityHandler) Pods() gin.HandlerFunc {
	return func(c *gin.Context) {
		h.withNamespace(c, func(ctx context.Context, ns string) (any, error) { return h.kube.ListPods(ctx, ns) })
	}
}
func (h *ObservabilityHandler) Deployments() gin.HandlerFunc {
	return func(c *gin.Context) {
		h.withNamespace(c, func(ctx context.Context, ns string) (any, error) { return h.kube.ListDeployments(ctx, ns) })
	}
}
func (h *ObservabilityHandler) Events() gin.HandlerFunc {
	return func(c *gin.Context) {
		h.withNamespace(c, func(ctx context.Context, ns string) (any, error) {
			events, err := h.kube.ListEvents(ctx, ns)
			limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))
			if limit < 1 {
				limit = 100
			}
			if limit > 500 {
				limit = 500
			}
			if len(events) > limit {
				events = events[len(events)-limit:]
			}
			return events, err
		})
	}
}

func (h *ObservabilityHandler) Summary() gin.HandlerFunc {
	return func(c *gin.Context) {
		h.withNamespace(c, func(ctx context.Context, ns string) (any, error) {
			if h.kube == nil {
				return nil, os.ErrNotExist
			}
			pods, err := h.kube.ListPods(ctx, ns)
			if err != nil {
				return nil, err
			}
			deployments, err := h.kube.ListDeployments(ctx, ns)
			if err != nil {
				return nil, err
			}
			events, err := h.kube.ListEvents(ctx, ns)
			if err != nil {
				return nil, err
			}
			ready, unhealthy := 0, 0
			for _, pod := range pods {
				if pod.Ready {
					ready++
				} else {
					unhealthy++
				}
			}
			deploymentReady := 0
			for _, deployment := range deployments {
				if deployment.Ready {
					deploymentReady++
				}
			}
			warning := 0
			for _, event := range events {
				if strings.EqualFold(event.Type, "Warning") {
					warning++
				}
			}
			return gin.H{"namespace": ns, "pods_total": len(pods), "pods_ready": ready, "pods_unhealthy": unhealthy, "deployments_total": len(deployments), "deployments_ready": deploymentReady, "events_warning": warning, "healthy": unhealthy == 0 && deploymentReady == len(deployments) && warning == 0}, nil
		})
	}
}

func (h *ObservabilityHandler) Logs() gin.HandlerFunc {
	return func(c *gin.Context) {
		h.withNamespace(c, func(ctx context.Context, ns string) (any, error) {
			tail, _ := strconv.ParseInt(c.DefaultQuery("tail_lines", "200"), 10, 64)
			if tail < 1 {
				tail = 200
			}
			if tail > 1000 {
				tail = 1000
			}
			since, _ := strconv.ParseInt(c.DefaultQuery("since_seconds", "0"), 10, 64)
			if since > 86400 {
				since = 86400
			}
			var sincePtr *int64
			if since > 0 {
				sincePtr = &since
			}
			logs, err := h.kube.GetPodLogs(ctx, ns, c.Param("pod"), c.Query("container"), kuberepo.LogOptions{TailLines: &tail, SinceSeconds: sincePtr, LimitBytes: 100000})
			return gin.H{"namespace": ns, "pod": c.Param("pod"), "container": c.Query("container"), "logs": logs}, err
		})
	}
}

func (h *ObservabilityHandler) withNamespace(c *gin.Context, fn func(context.Context, string) (any, error)) {
	if h.kube == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"message": "Kubernetes integration unavailable"})
		return
	}
	ns := c.DefaultQuery("namespace", h.cfg.Kubernetes.Namespace)
	if ns != h.cfg.Kubernetes.Namespace {
		c.JSON(http.StatusForbidden, gin.H{"message": "namespace is outside configured scope"})
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()
	data, err := fn(ctx, ns)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"message": "cluster query failed", "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"namespace": ns, "data": data})
}

type DocumentInfo struct {
	Name       string    `json:"name"`
	Size       int64     `json:"size"`
	ModifiedAt time.Time `json:"modified_at"`
	Indexed    bool      `json:"indexed"`
}
type DocumentHandler struct {
	root    string
	indexer interface {
		DeleteDocument(context.Context, string) error
	}
}

func NewDocumentHandler(root string, index interface {
	DeleteDocument(context.Context, string) error
}) *DocumentHandler { return &DocumentHandler{root: root, indexer: index} }
func (h *DocumentHandler) List() gin.HandlerFunc {
	return func(c *gin.Context) {
		entries, err := os.ReadDir(h.root)
		if err != nil && !os.IsNotExist(err) {
			c.JSON(500, gin.H{"message": err.Error()})
			return
		}
		docs := []DocumentInfo{}
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".md") {
				continue
			}
			info, statErr := entry.Info()
			if statErr == nil {
				docs = append(docs, DocumentInfo{Name: entry.Name(), Size: info.Size(), ModifiedAt: info.ModTime(), Indexed: true})
			}
		}
		c.JSON(200, gin.H{"documents": docs})
	}
}
func (h *DocumentHandler) Delete() gin.HandlerFunc {
	return func(c *gin.Context) {
		name := filepath.Base(c.Param("name"))
		if name != c.Param("name") || !strings.HasSuffix(strings.ToLower(name), ".md") {
			c.JSON(400, gin.H{"message": "invalid document name"})
			return
		}
		path := filepath.Join(h.root, name)
		if _, err := os.Stat(path); err != nil {
			c.JSON(404, gin.H{"message": "document not found"})
			return
		}
		if h.indexer != nil {
			if err := h.indexer.DeleteDocument(c.Request.Context(), name); err != nil {
				c.JSON(502, gin.H{"message": "failed to delete indexed document", "error": err.Error()})
				return
			}
		}
		if err := os.Remove(path); err != nil {
			c.JSON(500, gin.H{"message": "failed to delete document", "error": err.Error()})
			return
		}
		c.JSON(200, gin.H{"message": "document deleted"})
	}
}
