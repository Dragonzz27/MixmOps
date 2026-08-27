package router

import (
	"AutoOps/internal/handler"
	kuberepo "AutoOps/internal/repo/kubernetes"
	indexer "AutoOps/internal/repo/qrdant/indexer"
	"AutoOps/internal/repo/sqlite"
	"AutoOps/internal/server/ai/agent/chat"
	"AutoOps/internal/server/background"
	"AutoOps/internal/server/cases"
	"AutoOps/internal/server/chatServer"
	knowledgeindex "AutoOps/internal/server/knowledge_index"
	maintenancedocument "AutoOps/internal/server/maintenance_document"
	"AutoOps/internal/server/plan"
	"AutoOps/pkg/config"
	"context"

	"github.com/cloudwego/eino-ext/components/model/openai"
	qdrant_retriever "github.com/cloudwego/eino-ext/components/retriever/qdrant"
	"github.com/cloudwego/eino/components/document"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

func InitRouter(ctx context.Context, r *gin.Engine, loger *logrus.Logger, config *config.Config, runner compose.Runnable[document.Source, bool], runnerChat compose.Runnable[*chat.UserMessage, *schema.Message], model *openai.ChatModel, retriever *qdrant_retriever.Retriever, kube kuberepo.KubernetesRepository, docIndexer indexer.QdranIndexerServer, database *sqlite.DB) {
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
	//对话
	chater := chatServer.NewChatServer(loger, runnerChat)
	chaterHandler := handler.NewChatHandler(chater)
	r.POST("/chat", chaterHandler.Chat())
	r.POST("/chatStream", chaterHandler.ChatSream())
	//运维
	planer := plan.NewPlanServer(*config, model, loger, retriever, kube)
	planerH := handler.NewPlanHandler(planer)
	r.GET("/plan", planerH.Plan())
	observability := handler.NewObservabilityHandler(config, kube)
	r.GET("/alerts", observability.Alerts())
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
	r.POST("/webhooks/alertmanager", modes.AlertmanagerWebhook())
	r.GET("/background/tasks", modes.Tasks())
	r.GET("/background/tasks/:id", modes.Task())
	r.POST("/background/tasks/:id/approve", modes.Approve())
	r.POST("/background/tasks/:id/reject", modes.Reject())
	r.GET("/incidents", modes.Incidents())
	r.GET("/incidents/:id", modes.Incident())
	r.POST("/incidents", modes.CreateIncident())
	r.GET("/work-orders", modes.WorkOrders())
	r.POST("/work-orders", modes.CreateWorkOrder())
}
