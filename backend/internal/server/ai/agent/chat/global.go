package chat

import (
	kuberepo "AutoOps/internal/repo/kubernetes"
	"AutoOps/pkg/config"

	qdrant_retriever "github.com/cloudwego/eino-ext/components/retriever/qdrant"
)

type chatServer struct {
	retriever  *qdrant_retriever.Retriever
	config     *config.Config
	kubernetes kuberepo.KubernetesRepository
}

func NewChatServer(retriever *qdrant_retriever.Retriever, cfg *config.Config, repositories ...kuberepo.KubernetesRepository) chatServer {
	var repository kuberepo.KubernetesRepository
	if len(repositories) > 0 {
		repository = repositories[0]
	}
	return chatServer{
		retriever:  retriever,
		config:     cfg,
		kubernetes: repository,
	}
}
