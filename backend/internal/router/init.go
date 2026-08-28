package router

import (
	"AutoOps/internal/handler"
	kuberepo "AutoOps/internal/repo/kubernetes"
	indexer "AutoOps/internal/repo/qrdant/indexer"
	"AutoOps/internal/repo/sqlite"
	"AutoOps/internal/server/ai/agent/sharedchat"
	"AutoOps/internal/server/background"
	"AutoOps/internal/server/cases"
	"AutoOps/internal/server/conversation"
	knowledgeindex "AutoOps/internal/server/knowledge_index"
	maintenancedocument "AutoOps/internal/server/maintenance_document"
	"AutoOps/pkg/config"
	"context"

	"github.com/cloudwego/eino/components/document"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

func InitRouter(ctx context.Context, r *gin.Engine, loger *logrus.Logger, config *config.Config, runner compose.Runnable[document.Source, bool], runnerChat compose.Runnable[*sharedchat.UserMessage, *schema.Message], kube kuberepo.KubernetesRepository, docIndexer indexer.QdranIndexerServer, database *sqlite.DB) {
	//cors
	corsConfig := cors.DefaultConfig()
	corsConfig.AllowOrigins = []string{"*"}
	corsConfig.AllowMethods = []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}
	corsConfig.AllowHeaders = []string{"Origin", "Content-Length", "Content-Type", "Authorization"}
	r.Use(cors.New(corsConfig))
	r.GET("/ping", func(ctx *gin.Context) {
		ctx.JSON(200, gin.H{
			"message": "pong",
		})
	})
	//文件上传
	uploder := knowledgeindex.NewFileUploaderServer(loger, runner)
	uploderHandler := handler.NewFileUploader("./docs/", uploder)
	r.POST("/upload", uploderHandler.Upload())
	observability := handler.NewObservabilityHandler(config, kube)
	r.GET("/cluster/summary", observability.Summary())
	r.GET("/cluster/pods", observability.Pods())
	r.GET("/cluster/deployments", observability.Deployments())
	r.GET("/cluster/events", observability.Events())
	r.GET("/cluster/pods/:pod/logs", observability.Logs())
	documents := handler.NewDocumentHandler("./docs", docIndexer)
	r.GET("/knowledge/documents", documents.List())
	r.DELETE("/knowledge/documents/:name", documents.Delete())
	maintenance := maintenancedocument.NewService("./docs", runner, docIndexer, loger)
	maintenanceHandler := handler.NewMaintenanceDocumentHandler(maintenance)
	r.GET("/maintenance-documents", maintenanceHandler.List())
	r.GET("/maintenance-documents/:name", maintenanceHandler.Get())
	r.POST("/maintenance-documents", maintenanceHandler.Upload())
	r.DELETE("/maintenance-documents/:name", maintenanceHandler.Delete())
	bg := background.NewService(database.DB, kube)
	cs := cases.NewService(database.DB)
	modes := handler.NewModeHandler(bg, cs, kube)
	supervisor := background.NewSupervisor(bg, cs, kube, config.Background, config.GetPrometheusURL(), runnerChat)
	modes.SetSupervisor(supervisor)
	supervisor.Start(ctx)
	r.POST("/webhooks/alertmanager", modes.AlertmanagerWebhook())
	r.GET("/background/status", modes.BackgroundStatus())
	r.GET("/background/tasks", modes.Tasks())
	r.GET("/background/tasks/:id", modes.Task())
	r.POST("/background/tasks/:id/approve", modes.Approve())
	r.POST("/background/tasks/:id/reject", modes.Reject())
	r.GET("/incidents", modes.Incidents())
	r.GET("/incidents/:id", modes.Incident())
	r.POST("/incidents", modes.CreateIncident())
	r.GET("/incidents/:id/context", modes.Incident())
	r.POST("/incidents/:id/context/refresh", modes.RefreshIncidentContext())
	r.POST("/incidents/:id/create-isolation-task", modes.CreateIsolationTask())
	r.POST("/incidents/:id/resolve", modes.ResolveIncident())
	r.GET("/work-orders", modes.WorkOrders())
	r.POST("/work-orders", modes.CreateWorkOrder())
	r.GET("/work-orders/:id", modes.WorkOrder())
	r.POST("/work-orders/:id/complete", modes.CompleteWorkOrder())
	r.POST("/work-orders/:id/cancel", modes.CancelWorkOrder())
	conversationHandler := handler.NewConversationHandler(conversation.NewService(cs, runnerChat))
	r.GET("/incidents/:id/messages", conversationHandler.History(conversation.OwnerIncident))
	r.POST("/incidents/:id/chat", conversationHandler.Chat(conversation.OwnerIncident))
	r.POST("/incidents/:id/chatStream", conversationHandler.Stream(conversation.OwnerIncident))
	r.GET("/work-orders/:id/messages", conversationHandler.History(conversation.OwnerWorkOrder))
	r.POST("/work-orders/:id/chat", conversationHandler.Chat(conversation.OwnerWorkOrder))
	r.POST("/work-orders/:id/chatStream", conversationHandler.Stream(conversation.OwnerWorkOrder))
}
