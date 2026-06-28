package ai

import (
	"context"
	"fmt"
	"strings"
)

type Collection struct {
	Name      string
	IndexName string
	Store     VectorStore
}

type RAGSystem struct {
	embedder  Embedder
	model     string
	stores    map[string]VectorStore
}

func NewRAGSystem(embedder Embedder) *RAGSystem {
	return &RAGSystem{
		embedder: embedder,
		model:    "text-embedding-3-small",
		stores:   make(map[string]VectorStore),
	}
}

func (r *RAGSystem) AddCollection(name string, store VectorStore) {
	r.stores[name] = store
}

func (r *RAGSystem) Index(ctx context.Context, collection string, docs []Document) error {
	store, ok := r.stores[collection]
	if !ok {
		return fmt.Errorf("collection %q not found", collection)
	}

	texts := make([]string, len(docs))
	for i, doc := range docs {
		texts[i] = doc.Content
	}

	resp, err := r.embedder.Embed(ctx, EmbeddingRequest{
		Input: texts,
		Model: r.model,
	})
	if err != nil {
		return fmt.Errorf("embedding: %w", err)
	}

	return store.Insert(ctx, docs, resp.Embeddings)
}

func (r *RAGSystem) Retrieve(ctx context.Context, collection string, query string, topK int) ([]DocumentWithScore, error) {
	store, ok := r.stores[collection]
	if !ok {
		return nil, nil
	}

	resp, err := r.embedder.Embed(ctx, EmbeddingRequest{
		Input: []string{query},
		Model: r.model,
	})
	if err != nil {
		return nil, fmt.Errorf("query embedding: %w", err)
	}

	if len(resp.Embeddings) == 0 {
		return nil, nil
	}

	return store.Search(ctx, resp.Embeddings[0], topK)
}

func (r *RAGSystem) EnrichPrompt(ctx context.Context, collection string, prompt string, topK int) (string, error) {
	if collection == "" {
		return prompt, nil
	}

	docs, err := r.Retrieve(ctx, collection, prompt, topK)
	if err != nil {
		return prompt, err
	}

	if len(docs) == 0 {
		return prompt, nil
	}

	var contextBuilder strings.Builder
	contextBuilder.WriteString("\n\nRelevant context:\n")
	for i, doc := range docs {
		contextBuilder.WriteString(fmt.Sprintf("\n[%d] %s\n", i+1, doc.Content))
	}

	return prompt + contextBuilder.String(), nil
}

func (r *RAGSystem) SetModel(model string) {
	r.model = model
}
