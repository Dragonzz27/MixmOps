package handler

import (
	"AutoOps/internal/server/conversation"
	"fmt"
	"github.com/gin-gonic/gin"
	"net/http"
)

type ConversationHandler struct{ service *conversation.Service }

func NewConversationHandler(s *conversation.Service) *ConversationHandler {
	return &ConversationHandler{service: s}
}
func (h *ConversationHandler) History(owner string) gin.HandlerFunc {
	return func(c *gin.Context) {
		messages, err := h.service.History(c.Request.Context(), owner, c.Param("id"))
		if err != nil {
			c.JSON(404, gin.H{"message": err.Error()})
			return
		}
		c.JSON(200, gin.H{"messages": messages})
	}
}
func (h *ConversationHandler) Chat(owner string) gin.HandlerFunc {
	return func(c *gin.Context) {
		var in struct {
			Message string `json:"message"`
		}
		if c.ShouldBindJSON(&in) != nil {
			c.JSON(400, gin.H{"message": "message is required"})
			return
		}
		out, err := h.service.Chat(c.Request.Context(), owner, c.Param("id"), in.Message)
		if err != nil {
			c.JSON(500, gin.H{"message": err.Error()})
			return
		}
		c.JSON(200, gin.H{"message": out})
	}
}
func (h *ConversationHandler) Stream(owner string) gin.HandlerFunc {
	return func(c *gin.Context) {
		var in struct {
			Message string `json:"message"`
		}
		if c.ShouldBindJSON(&in) != nil {
			c.JSON(400, gin.H{"message": "message is required"})
			return
		}
		c.Header("Content-Type", "text/event-stream")
		c.Header("Cache-Control", "no-cache")
		ch := make(chan string, 8)
		errCh := make(chan error, 1)
		go func() {
			errCh <- h.service.Stream(c.Request.Context(), owner, c.Param("id"), in.Message, ch)
			close(ch)
		}()
		for token := range ch {
			c.SSEvent("message", fmt.Sprintf("data: %s", token))
			c.Writer.Flush()
		}
		if err := <-errCh; err != nil {
			c.SSEvent("error", err.Error())
			c.Writer.Flush()
		} else {
			c.SSEvent("message", "data: [DONE]")
			c.Writer.Flush()
		}
	}
}

var _ = http.StatusOK
