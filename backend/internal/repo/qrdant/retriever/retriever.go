package retriever

import (
	"context"
	qdrant_retriever "github.com/cloudwego/eino-ext/components/retriever/qdrant"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/qdrant/go-client/qdrant"
)

type RetrieverServer interface {
	NewRetrieverServer(ctx context.Context, collectionName string, ScoreThreshold float64, limit int) (*qdrant_retriever.Retriever, error)
}

type retrieverServer struct {
	client  *qdrant.Client
	embeder embedding.Embedder
}

func NewRetrieverServer(ctx context.Context, client *qdrant.Client, embeder embedding.Embedder) RetrieverServer {
	return &retrieverServer{
		client:  client,
		embeder: embeder,
	}
}

func (rs retrieverServer) NewRetrieverServer(ctx context.Context, collectionName string, ScoreThreshold float64, limit int) (*qdrant_retriever.Retriever, error) {
	return qdrant_retriever.NewRetriever(ctx, &qdrant_retriever.Config{
		Client:         rs.client,
		Collection:     collectionName,
		Embedding:      rs.embeder,
		ScoreThreshold: &ScoreThreshold,
		TopK:           limit, // 返回最相似的N个文档
	})
}
