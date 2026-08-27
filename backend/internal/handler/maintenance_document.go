package handler

import (
	"net/http"
	"strings"

	maintenancedocument "AutoOps/internal/server/maintenance_document"
	"github.com/gin-gonic/gin"
)

type MaintenanceDocumentHandler struct{ service maintenancedocument.Service }

func NewMaintenanceDocumentHandler(service maintenancedocument.Service) *MaintenanceDocumentHandler {
	return &MaintenanceDocumentHandler{service: service}
}
func (h *MaintenanceDocumentHandler) List() gin.HandlerFunc {
	return func(c *gin.Context) {
		docs, err := h.service.List(c.Request.Context(), c.Query("type"), c.Query("tag"), c.Query("keyword"))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"documents": docs})
	}
}
func (h *MaintenanceDocumentHandler) Get() gin.HandlerFunc {
	return func(c *gin.Context) {
		doc, content, err := h.service.Get(c.Request.Context(), c.Param("name"))
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"message": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"document": doc, "content": content})
	}
}
func (h *MaintenanceDocumentHandler) Upload() gin.HandlerFunc {
	return func(c *gin.Context) {
		file, err := c.FormFile("file")
		if err != nil || !strings.HasSuffix(strings.ToLower(file.Filename), ".md") {
			c.JSON(http.StatusBadRequest, gin.H{"message": "a Markdown file is required"})
			return
		}
		doc, err := h.service.Upload(c.Request.Context(), file)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"document": doc})
	}
}
func (h *MaintenanceDocumentHandler) Delete() gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := h.service.Delete(c.Request.Context(), c.Param("name")); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "document deleted"})
	}
}
