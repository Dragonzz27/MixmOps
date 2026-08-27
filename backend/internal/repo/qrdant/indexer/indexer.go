package indexer

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/components/embedding"
	"github.com/qdrant/go-client/qdrant"
)

const (
	CollectionName = "autoops"
)

type QdranIndexerServer interface {
	NewQdrantIndexer(ctx context.Context) error
	AddVector(ctx context.Context, points *qdrant.UpsertPoints) error
	DeleteDocument(ctx context.Context, document string) error
}

func (qs qdrantIndexerServer) DeleteDocument(ctx context.Context, document string) error {
	collection := qs.collection
	if collection == "" {
		collection = CollectionName
	}
	points, err := qs.client.Scroll(ctx, &qdrant.ScrollPoints{CollectionName: collection, Filter: &qdrant.Filter{Must: []*qdrant.Condition{qdrant.NewMatchText("content", document)}}})
	if err != nil {
		return err
	}
	ids := make([]*qdrant.PointId, 0, len(points))
	for _, point := range points {
		if point != nil && point.Id != nil {
			ids = append(ids, point.Id)
		}
	}
	if len(ids) == 0 {
		return nil
	}
	_, err = qs.client.Delete(ctx, &qdrant.DeletePoints{CollectionName: collection, Points: qdrant.NewPointsSelectorIDs(ids)})
	return err
}

type qdrantIndexerServer struct {
	client     *qdrant.Client
	embedder   embedding.Embedder
	collection string
	dimension  uint64
}

func NewQdranIndexerServer(ctx context.Context, client *qdrant.Client, embedder embedding.Embedder, collection string, dimension uint64) qdrantIndexerServer {
	return qdrantIndexerServer{
		client:     client,
		embedder:   embedder,
		collection: collection,
		dimension:  dimension,
	}
}

func (qs qdrantIndexerServer) NewQdrantIndexer(ctx context.Context) error {
	collection := qs.collection
	if collection == "" {
		collection = CollectionName
	}
	if exists, err := qs.client.CollectionExists(ctx, collection); err != nil {
		return err
	} else {
		if exists {
			info, err := qs.client.GetCollectionInfo(ctx, collection)
			if err != nil {
				return err
			}
			if info.Config == nil || info.Config.Params == nil || info.Config.Params.VectorsConfig == nil || info.Config.Params.VectorsConfig.GetParams() == nil {
				return fmt.Errorf("collection %s has no single dense vector configuration", collection)
			}
			actual := info.Config.Params.VectorsConfig.GetParams().GetSize()
			if actual != qs.dimension {
				return fmt.Errorf("collection %s dimension mismatch: expected %d, got %d", collection, qs.dimension, actual)
			}
			return nil
		}
		err := qs.client.CreateCollection(ctx, &qdrant.CreateCollection{
			CollectionName: collection,
			VectorsConfig: &qdrant.VectorsConfig{
				Config: &qdrant.VectorsConfig_Params{
					Params: &qdrant.VectorParams{
						Size:     qs.dimension,
						Distance: qdrant.Distance_Dot, //使用内积
					},
				},
			},
		})
		return err
	}
	return nil
}

// AddVector 添加向量
func (qs qdrantIndexerServer) AddVector(ctx context.Context, points *qdrant.UpsertPoints) error {
	points.CollectionName = qs.collection
	if points.CollectionName == "" {
		points.CollectionName = CollectionName
	}
	res, err := qs.client.Upsert(ctx, points)
	if err != nil {
		return err
	}
	fmt.Println(res)
	return nil
}
