package ai

import (
	"context"
	"fmt"
	"time"

	goopenai "github.com/sashabaranov/go-openai"
)

type EmbeddingRequest struct {
	Input []string
	Model string
}

type EmbeddingResponse struct {
	Embeddings [][]float32
	Model      string
	TokensUsed int
	DurationMs int
}

type Embedder interface {
	Embed(ctx context.Context, req EmbeddingRequest) (EmbeddingResponse, error)
}

type OpenAIEmbedder struct {
	client *goopenai.Client
}

func NewOpenAIEmbedder(apiKey, baseURL string) (*OpenAIEmbedder, error) {
	cfg := goopenai.DefaultConfig(apiKey)
	if baseURL != "" {
		cfg.BaseURL = baseURL
	}
	return &OpenAIEmbedder{
		client: goopenai.NewClientWithConfig(cfg),
	}, nil
}

func (e *OpenAIEmbedder) Embed(ctx context.Context, req EmbeddingRequest) (EmbeddingResponse, error) {
	start := time.Now()
	model := req.Model
	if model == "" {
		model = "text-embedding-3-small"
	}

	resp, err := e.client.CreateEmbeddings(ctx, goopenai.EmbeddingRequest{
		Input: req.Input,
		Model: goopenai.EmbeddingModel(model),
	})
	if err != nil {
		return EmbeddingResponse{}, fmt.Errorf("embedding: %w", err)
	}

	embeddings := make([][]float32, len(resp.Data))
	for i, d := range resp.Data {
		embeddings[i] = d.Embedding
	}

	return EmbeddingResponse{
		Embeddings: embeddings,
		Model:      model,
		TokensUsed: resp.Usage.TotalTokens,
		DurationMs: int(time.Since(start).Milliseconds()),
	}, nil
}

type noopEmbedder struct{}

func NewNoopEmbedder() *noopEmbedder {
	return &noopEmbedder{}
}

func (e *noopEmbedder) Embed(_ context.Context, _ EmbeddingRequest) (EmbeddingResponse, error) {
	return EmbeddingResponse{}, nil
}
