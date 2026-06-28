package ai

import (
	"context"
	"math"
	"sort"
	"sync"
	"time"
)

type Document struct {
	ID        string    `json:"id"`
	Content   string    `json:"content"`
	Metadata  map[string]string `json:"metadata,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

type DocumentWithScore struct {
	Document
	Score float64
}

type VectorStore interface {
	Insert(ctx context.Context, docs []Document, embeddings [][]float32) error
	Search(ctx context.Context, queryEmbedding []float32, topK int) ([]DocumentWithScore, error)
	Delete(ctx context.Context, ids []string) error
	Clear(ctx context.Context) error
}

type InMemoryVectorStore struct {
	mu         sync.RWMutex
	documents  map[string]Document
	vectors    map[string][]float32
}

func NewInMemoryVectorStore() *InMemoryVectorStore {
	return &InMemoryVectorStore{
		documents: make(map[string]Document),
		vectors:   make(map[string][]float32),
	}
}

func (s *InMemoryVectorStore) Insert(_ context.Context, docs []Document, embeddings [][]float32) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(docs) != len(embeddings) {
		return nil
	}

	for i, doc := range docs {
		if doc.ID == "" {
			continue
		}
		s.documents[doc.ID] = doc
		s.vectors[doc.ID] = embeddings[i]
	}
	return nil
}

func (s *InMemoryVectorStore) Search(_ context.Context, queryEmbedding []float32, topK int) ([]DocumentWithScore, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	type scored struct {
		id    string
		score float64
	}

	var scores []scored
	for id, vec := range s.vectors {
		score := cosineSimilarity(queryEmbedding, vec)
		scores = append(scores, scored{id: id, score: score})
	}

	sort.Slice(scores, func(i, j int) bool {
		return scores[i].score > scores[j].score
	})

	if topK <= 0 || topK > len(scores) {
		topK = len(scores)
	}
	scores = scores[:topK]

	results := make([]DocumentWithScore, 0, topK)
	for _, sc := range scores {
		doc, ok := s.documents[sc.id]
		if !ok {
			continue
		}
		results = append(results, DocumentWithScore{
			Document: doc,
			Score:    sc.score,
		})
	}

	return results, nil
}

func (s *InMemoryVectorStore) Delete(_ context.Context, ids []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, id := range ids {
		delete(s.documents, id)
		delete(s.vectors, id)
	}
	return nil
}

func (s *InMemoryVectorStore) Clear(_ context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.documents = make(map[string]Document)
	s.vectors = make(map[string][]float32)
	return nil
}

func cosineSimilarity(a, b []float32) float64 {
	if len(a) == 0 || len(b) == 0 || len(a) != len(b) {
		return 0
	}
	var dot, normA, normB float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dot / (math.Sqrt(normA) * math.Sqrt(normB))
}
