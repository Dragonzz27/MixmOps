package main

import (
	kuberepo "AutoOps/internal/repo/kubernetes"
	indexerr "AutoOps/internal/repo/qrdant/indexer"
	initQdrantRepo "AutoOps/internal/repo/qrdant/init"
	"AutoOps/internal/repo/qrdant/retriever"
	"AutoOps/internal/router"
	"AutoOps/internal/server/ai/agent/chat"
	knowledgeindex "AutoOps/internal/server/ai/agent/knowledge_index"
	"AutoOps/internal/server/ai/embeder"
	aitools "AutoOps/internal/server/ai/tools"
	"AutoOps/internal/server/model"
	"AutoOps/pkg/config"
	"AutoOps/pkg/log"
	"context"
	"fmt"
	"os"

	"github.com/gin-gonic/gin"
)

func main() {
	ctx := context.Background()
	// 初始化日志记录器
	//初始化配置
	envFile := os.Getenv("AUTOOPS_ENV_FILE")
	config, err := config.InitConfig(envFile)
	if err != nil {
		panic(err)
	}
	log := log.InitLogger(config.Log.Level, config.Log.File)
	kubernetes, err := kuberepo.NewRepository(config.Kubernetes)
	if err != nil {
		log.Warnf("Kubernetes integration unavailable; continuing without cluster tools: %v", err)
		kubernetes = nil
	}
	//初始化qdrant
	indexer, err := initQdrantRepo.NewQdrantIndexer(ctx, config)
	if err != nil {
		panic(err)
	}
	//初始化embedding
	embedder, err := embeder.NewProvider(ctx, config.Embedding)
	if err != nil {
		panic(err)
	}
	if err := embedder.Validate(ctx); err != nil {
		panic(fmt.Errorf("embedding validation failed: %w", err))
	}
	//初始化retriever
	retriever := retriever.NewRetrieverServer(ctx, indexer, embedder)
	run, err := retriever.NewRetrieverServer(ctx, config.Qdrant.Collection, 0.5, 2)
	if err != nil {
		panic(err)
	}
	aitools.InitRAGTool(run)
	//初始化chatAgent
	re := chat.NewChatServer(run, config, kubernetes)
	runner, err := re.BuildChatAgent(ctx)
	if err != nil {
		panic(err)
	}
	//新建集合
	indexerr := indexerr.NewQdranIndexerServer(ctx, indexer, embedder, config.Qdrant.Collection, uint64(config.Embedding.Dimension))
	err = indexerr.NewQdrantIndexer(ctx)
	if err != nil {
		panic(err)
	}
	//初始化RAGagent
	knowledgeIndex := knowledgeindex.NewKnowledgeIndex(embeder.NewEmbeddingServer(embedder, config.Embedding.Dimension), indexerr)
	runnerRAG, err := knowledgeIndex.NewGraph(ctx)
	if err != nil {
		panic(err)
	}
	//初始化model
	chatModel, err := model.NewOpenaiModel(ctx, config)
	if err != nil {
		panic(err)
	}
	// 初始化gin
	r := gin.Default()
	router.InitRouter(ctx, r, log, config, runnerRAG, runner, chatModel, run, kubernetes, indexerr)
	// 启动 HTTP 服务
	addr := fmt.Sprintf("%s:%d", config.Server.Host, config.Server.Port)
	if err = r.Run(addr); err != nil {
		panic(err)
	}
}
