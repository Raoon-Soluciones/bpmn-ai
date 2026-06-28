package ai

import (
	"context"
	"testing"
)

func TestInMemoryVectorStore_InsertAndSearch(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryVectorStore()

	err := store.Insert(ctx, []Document{
		{ID: "1", Content: "The sky is blue"},
		{ID: "2", Content: "Grass is green"},
	}, [][]float32{
		{0.1, 0.2, 0.3},
		{0.9, 0.8, 0.7},
	})
	if err != nil {
		t.Fatalf("Insert: %v", err)
	}

	results, err := store.Search(ctx, []float32{0.1, 0.2, 0.3}, 5)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}
	if results[0].ID != "1" {
		t.Errorf("expected top result ID '1', got '%s'", results[0].ID)
	}
}

func TestInMemoryVectorStore_Delete(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryVectorStore()

	store.Insert(ctx, []Document{{ID: "1", Content: "doc1"}}, [][]float32{{0.1, 0.2}})
	store.Delete(ctx, []string{"1"})

	results, _ := store.Search(ctx, []float32{0.1, 0.2}, 5)
	if len(results) != 0 {
		t.Errorf("expected 0 results after delete, got %d", len(results))
	}
}

func TestInMemoryVectorStore_Clear(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryVectorStore()

	store.Insert(ctx, []Document{{ID: "a", Content: "doc a"}}, [][]float32{{0.1}})
	store.Insert(ctx, []Document{{ID: "b", Content: "doc b"}}, [][]float32{{0.2}})
	store.Clear(ctx)

	results, _ := store.Search(ctx, []float32{0.1}, 5)
	if len(results) != 0 {
		t.Errorf("expected 0 results after clear, got %d", len(results))
	}
}

func TestCosineSimilarity(t *testing.T) {
	a := []float32{1, 0, 0}
	b := []float32{1, 0, 0}
	score := cosineSimilarity(a, b)
	if score != 1.0 {
		t.Errorf("expected 1.0 for identical vectors, got %f", score)
	}

	c := []float32{-1, 0, 0}
	score = cosineSimilarity(a, c)
	if score != -1.0 {
		t.Errorf("expected -1.0 for opposite vectors, got %f", score)
	}

	d := []float32{0, 1, 0}
	score = cosineSimilarity(a, d)
	if score != 0.0 {
		t.Errorf("expected 0.0 for orthogonal vectors, got %f", score)
	}
}

func TestCosineSimilarity_Empty(t *testing.T) {
	if score := cosineSimilarity(nil, []float32{1}); score != 0 {
		t.Errorf("expected 0 for empty a, got %f", score)
	}
	if score := cosineSimilarity([]float32{1}, nil); score != 0 {
		t.Errorf("expected 0 for empty b, got %f", score)
	}
	if score := cosineSimilarity([]float32{1}, []float32{1, 2}); score != 0 {
		t.Errorf("expected 0 for mismatched lengths, got %f", score)
	}
}

func TestRAGSystem_EnrichPrompt_NoCollection(t *testing.T) {
	rag := NewRAGSystem(NewNoopEmbedder())
	enriched, err := rag.EnrichPrompt(context.Background(), "", "hello", 5)
	if err != nil {
		t.Fatalf("EnrichPrompt: %v", err)
	}
	if enriched != "hello" {
		t.Errorf("expected unchanged prompt, got '%s'", enriched)
	}
}

func TestRAGSystem_EnrichPrompt_UnknownCollection(t *testing.T) {
	rag := NewRAGSystem(NewNoopEmbedder())
	enriched, err := rag.EnrichPrompt(context.Background(), "nonexistent", "hello", 5)
	if err != nil {
		t.Fatalf("EnrichPrompt: %v", err)
	}
	if enriched != "hello" {
		t.Errorf("expected unchanged prompt, got '%s'", enriched)
	}
}

func TestParseAgents(t *testing.T) {
	agents, err := ParseAgents(`[{"name":"agent1","prompt":"analyze","outputKey":"analysis"}]`)
	if err != nil {
		t.Fatalf("ParseAgents: %v", err)
	}
	if len(agents) != 1 {
		t.Errorf("expected 1 agent, got %d", len(agents))
	}
	if agents[0].Name != "agent1" {
		t.Errorf("expected name 'agent1', got '%s'", agents[0].Name)
	}
}

func TestParseAgents_Empty(t *testing.T) {
	agents, err := ParseAgents("")
	if err != nil {
		t.Fatalf("ParseAgents: %v", err)
	}
	if agents != nil {
		t.Errorf("expected nil for empty, got %v", agents)
	}
}

func TestParseAgents_Invalid(t *testing.T) {
	_, err := ParseAgents("not json")
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestMultiAgentExecutor(t *testing.T) {
	gw := &mockProvider{response: "result"}
	exec := NewMultiAgentExecutor(gw)

	results, err := exec.Execute(context.Background(), []SubAgentTask{
		{Name: "step1", Prompt: "do {{task}}", OutputKey: "step1_out"},
	}, map[string]string{"task": "test"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if results["step1_out"] != "result do test" {
		t.Errorf("expected 'result do test', got '%s'", results["step1_out"])
	}
}
