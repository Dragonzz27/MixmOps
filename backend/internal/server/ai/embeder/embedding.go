package embeder

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"strings"
	"time"

	"AutoOps/pkg/config"
	"github.com/cloudwego/eino-ext/components/embedding/ollama"
	aclopenai "github.com/cloudwego/eino-ext/libs/acl/openai"
	"github.com/cloudwego/eino/components/embedding"
)

type EmbeddingProvider interface {
	embedding.Embedder
	Provider() string
	Model() string
	Dimension() int
	Validate(context.Context) error
}
type EmbeddingServer interface {
	Embedding(context.Context, []string) ([][]float64, error)
	Average([][]float64) ([]float64, error)
	Normalize([]float64) []float64
	Dimension() int
}
type embeddingServer struct {
	provider  EmbeddingProvider
	dimension int
}

func NewProvider(ctx context.Context, cfg config.EmbeddingConfig) (EmbeddingProvider, error) {
	name := strings.ToLower(strings.TrimSpace(cfg.Provider))
	if name == "" {
		name = "openai"
	}
	if cfg.Model == "" {
		return nil, fmt.Errorf("embedding model is required")
	}
	if cfg.Dimension <= 0 {
		return nil, fmt.Errorf("embedding dimension must be positive")
	}
	timeout := 30 * time.Second
	if cfg.Timeout != "" {
		d, e := time.ParseDuration(cfg.Timeout)
		if e != nil || d <= 0 {
			return nil, fmt.Errorf("invalid embedding timeout %q", cfg.Timeout)
		}
		timeout = d
	}
	batch := cfg.BatchSize
	if batch <= 0 {
		batch = 32
	}
	if batch > 128 {
		return nil, fmt.Errorf("embedding batch_size cannot exceed 128")
	}
	retries := cfg.MaxRetries
	if retries < 0 || retries > 8 {
		return nil, fmt.Errorf("embedding max_retries must be between 0 and 8")
	}
	var delegate embedding.Embedder
	var err error
	switch name {
	case "openai":
		if strings.TrimSpace(cfg.APIKey) == "" {
			return nil, fmt.Errorf("embedding api_key is required for openai provider")
		}
		delegate, err = aclopenai.NewEmbeddingClient(ctx, &aclopenai.EmbeddingConfig{APIKey: cfg.APIKey, BaseURL: strings.TrimRight(cfg.BaseURL, "/"), Model: cfg.Model, Dimensions: &cfg.Dimension, HTTPClient: &http.Client{Timeout: timeout}})
	case "ollama":
		base := cfg.BaseURL
		if base == "" {
			base = "http://127.0.0.1:11434"
		}
		delegate, err = ollama.NewEmbedder(ctx, &ollama.EmbeddingConfig{BaseURL: strings.TrimRight(base, "/"), Model: cfg.Model})
	default:
		return nil, fmt.Errorf("unsupported embedding provider %q", name)
	}
	if err != nil {
		return nil, fmt.Errorf("create %s embedding provider: %w", name, err)
	}
	return &providerAdapter{provider: name, model: cfg.Model, dimension: cfg.Dimension, delegate: delegate, batchSize: batch, maxRetries: retries, normalizeEnabled: cfg.Normalize}, nil
}

type providerAdapter struct {
	provider, model                  string
	dimension, batchSize, maxRetries int
	normalizeEnabled                 bool
	delegate                         embedding.Embedder
}

func (p *providerAdapter) Provider() string { return p.provider }
func (p *providerAdapter) Model() string    { return p.model }
func (p *providerAdapter) Dimension() int   { return p.dimension }
func (p *providerAdapter) EmbedStrings(ctx context.Context, texts []string, opts ...embedding.Option) ([][]float64, error) {
	if len(texts) == 0 {
		return [][]float64{}, nil
	}
	out := make([][]float64, 0, len(texts))
	for start := 0; start < len(texts); start += p.batchSize {
		end := start + p.batchSize
		if end > len(texts) {
			end = len(texts)
		}
		var vectors [][]float64
		var err error
		for attempt := 0; attempt <= p.maxRetries; attempt++ {
			vectors, err = p.delegate.EmbedStrings(ctx, texts[start:end], opts...)
			if err == nil {
				break
			}
			if attempt < p.maxRetries {
				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				case <-time.After(time.Duration(200*(1<<attempt)) * time.Millisecond):
				}
			}
		}
		if err != nil {
			return nil, fmt.Errorf("embedding provider=%s model=%s batch=%d: %w", p.provider, p.model, start/p.batchSize, err)
		}
		if len(vectors) != end-start {
			return nil, fmt.Errorf("embedding provider=%s returned %d vectors for %d inputs", p.provider, len(vectors), end-start)
		}
		for _, v := range vectors {
			if len(v) != p.dimension {
				return nil, fmt.Errorf("embedding provider=%s model=%s dimension mismatch: expected %d, got %d", p.provider, p.model, p.dimension, len(v))
			}
			if p.normalizeEnabled {
				normalize(v)
			}
		}
		out = append(out, vectors...)
	}
	return out, nil
}
func (p *providerAdapter) Validate(ctx context.Context) error {
	_, err := p.EmbedStrings(ctx, []string{"autoops embedding dimension validation"})
	return err
}
func normalize(v []float64) {
	var s float64
	for _, x := range v {
		s += x * x
	}
	n := math.Sqrt(s)
	if n == 0 {
		return
	}
	for i := range v {
		v[i] /= n
	}
}
func NewEmbeddingServer(provider EmbeddingProvider, dimension int) EmbeddingServer {
	return &embeddingServer{provider: provider, dimension: dimension}
}
func (e *embeddingServer) Embedding(ctx context.Context, t []string) ([][]float64, error) {
	return e.provider.EmbedStrings(ctx, t)
}
func (e *embeddingServer) Dimension() int                  { return e.dimension }
func (e *embeddingServer) Normalize(v []float64) []float64 { normalize(v); return v }
func (e *embeddingServer) Average(es [][]float64) ([]float64, error) {
	if len(es) == 0 {
		return nil, fmt.Errorf("no embeddings provided")
	}
	dim := len(es[0])
	avg := make([]float64, dim)
	for j, v := range es {
		if len(v) != dim {
			return nil, fmt.Errorf("embedding %d has different length", j)
		}
		for i, x := range v {
			avg[i] += x
		}
	}
	for i := range avg {
		avg[i] /= float64(len(es))
	}
	return avg, nil
}
