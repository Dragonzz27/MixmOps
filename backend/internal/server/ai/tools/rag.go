package tools

import (
	"context"
	"sync"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"

	qdrant_retriever "github.com/cloudwego/eino-ext/components/retriever/qdrant"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/schema"
	"github.com/qdrant/go-client/qdrant"
)

// RAGTool 信息检索工具

var ragToolGlobal *qdrant_retriever.Retriever
var mu sync.Mutex

// InitRAGTool 初始化 RAG 工具
func InitRAGTool(retriever *qdrant_retriever.Retriever) {
	mu.Lock()
	defer mu.Unlock()
	ragToolGlobal = retriever
}

func NewRetrieverServer(ctx context.Context, client *qdrant.Client, collectionName string, embeddder embedding.Embedder, ScoreThreshold float64, limit int) (*qdrant_retriever.Retriever, error) {
	mu.Lock()
	defer mu.Unlock()
	var err error
	if ragToolGlobal == nil {
		ragToolGlobal, err = qdrant_retriever.NewRetriever(ctx, &qdrant_retriever.Config{
			Client:         client,
			Collection:     collectionName,
			Embedding:      embeddder,
			ScoreThreshold: &ScoreThreshold,
			TopK:           limit, // 返回最相似的N个文档
		})
	}
	if err != nil {
		return nil, err
	}
	return ragToolGlobal, nil
}

type RetrieveRequest struct {
	Query string `json:"query" jsonschema:"description=The query string to search in internal documentation for relevant information and processing steps"`
}

func retrieve(ctx context.Context, query RetrieveRequest) (docs []*schema.Document, err error) {
	return ragToolGlobal.Retrieve(ctx, query.Query)
}

func RetrieveTool() (tool.InvokableTool, error) {
	return utils.InferTool("query_internal_docs",
		"查询内部维护文档，包括集群架构、运维手册、故障预案和变更规范。使用检索结果中的文档类型、描述和标签来选择最合适的内部依据。",
		retrieve)
}
