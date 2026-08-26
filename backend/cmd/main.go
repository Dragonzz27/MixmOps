package main

import (
	indexerr "AutoOps/internal/repo/qrdant/indexer"
	initQdrantRepo "AutoOps/internal/repo/qrdant/init"
	"AutoOps/internal/repo/qrdant/retriever"
	"AutoOps/internal/router"
	"AutoOps/internal/server/ai/agent/chat"
	knowledgeindex "AutoOps/internal/server/ai/agent/knowledge_index"
	"AutoOps/internal/server/ai/embeder"
	"AutoOps/internal/server/model"
	"AutoOps/pkg/config"
	"AutoOps/pkg/log"
	"context"
	"fmt"

	"github.com/gin-gonic/gin"
)

func main() {
	ctx := context.Background()
	// 初始化日志记录器
	log := log.InitLogger("info", "log/AutoOps.log")
	//初始化配置
	config, err := config.InitConfig("./config/config.json")
	if err != nil {
		panic(err)
	}
	//初始化qdrant
	indexer, err := initQdrantRepo.NewQdrantIndexer(ctx, config)
	if err != nil {
		panic(err)
	}
	//初始化embedding
	embedder, err := embeder.NewEmbedder(ctx, config)
	if err != nil {
		panic(err)
	}
	//初始化retriever
	retriever := retriever.NewRetrieverServer(ctx, indexer, *embedder)
	run, err := retriever.NewRetrieverServer(ctx, "autoops", 0.5, 2)
	if err != nil {
		panic(err)
	}
	//初始化chatAgent
	re := chat.NewChatServer(run, config)
	runner, err := re.BuildChatAgent(ctx)
	if err != nil {
		panic(err)
	}
	//新建集合
	indexerr := indexerr.NewQdranIndexerServer(ctx, indexer, *embedder)
	err = indexerr.NewQdrantIndexer(ctx)
	if err != nil {
		panic(err)
	}
	//初始化RAGagent
	knowledgeIndex := knowledgeindex.NewKnowledgeIndex(embeder.NewEmbeddingServer(embedder), indexerr)
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
	router.InitRouter(ctx, r, log, config, runnerRAG, runner, chatModel, run)
	// 启动 HTTP 服务
	addr := fmt.Sprintf("%s:%d", config.Server.Host, config.Server.Port)
	if err = r.Run(addr); err != nil {
		panic(err)
	}
}
