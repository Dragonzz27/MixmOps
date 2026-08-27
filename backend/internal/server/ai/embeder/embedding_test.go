package embeder

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"AutoOps/pkg/config"
	"github.com/cloudwego/eino/components/embedding"
)

type fakeEmbedder struct {
	calls   int
	vectors [][]float64
	err     error
}

func (f *fakeEmbedder) EmbedStrings(_ context.Context, texts []string, _ ...embedding.Option) ([][]float64, error) {
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	out := make([][]float64, len(texts))
	for i := range out {
		out[i] = append([]float64(nil), f.vectors[i%len(f.vectors)]...)
	}
	return out, nil
}

func TestProviderAdapterBatchesAndValidatesDimension(t *testing.T) {
	f := &fakeEmbedder{vectors: [][]float64{{3, 4}}}
	p := &providerAdapter{provider: "test", model: "m", dimension: 2, batchSize: 1, delegate: f}
	got, err := p.EmbedStrings(context.Background(), []string{"a", "b"})
	if err != nil || len(got) != 2 || f.calls != 2 {
		t.Fatalf("unexpected result: %#v calls=%d err=%v", got, f.calls, err)
	}
	if got[0][0] != 3 || got[0][1] != 4 {
		t.Fatalf("unexpected vector: %#v", got)
	}
}

func TestProviderAdapterDimensionError(t *testing.T) {
	p := &providerAdapter{provider: "test", model: "m", dimension: 3, batchSize: 2, delegate: &fakeEmbedder{vectors: [][]float64{{1, 2}}}}
	if _, err := p.EmbedStrings(context.Background(), []string{"a"}); err == nil {
		t.Fatal("expected dimension error")
	}
}

func TestNewProviderValidation(t *testing.T) {
	if _, err := NewProvider(context.Background(), config.EmbeddingConfig{Provider: "unknown", Model: "m", Dimension: 2}); err == nil {
		t.Fatal("expected unsupported provider error")
	}
	if _, err := NewProvider(context.Background(), config.EmbeddingConfig{Provider: "openai", Model: "m", Dimension: 2}); err == nil {
		t.Fatal("expected api key error")
	}
}

func TestOpenAICompatibleProvider(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/embeddings" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer secret" {
			t.Fatalf("missing authorization")
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{{"embedding": []float32{3, 4}, "index": 0}}})
	}))
	defer server.Close()
	p, err := NewProvider(context.Background(), config.EmbeddingConfig{Provider: "openai", APIKey: "secret", BaseURL: server.URL + "/v1", Model: "test-model", Dimension: 2, Normalize: true, MaxRetries: 0})
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Validate(context.Background()); err != nil {
		t.Fatal(err)
	}
	v, err := p.EmbedStrings(context.Background(), []string{"hello"})
	if err != nil {
		t.Fatal(err)
	}
	if len(v) != 1 || len(v[0]) != 2 || v[0][0] != 0.6 || v[0][1] != 0.8 {
		t.Fatalf("unexpected vector: %#v", v)
	}
}
