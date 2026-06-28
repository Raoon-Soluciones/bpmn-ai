package ai

import (
	"context"
	"testing"
	"time"
)

func TestCacheKey(t *testing.T) {
	key1 := CacheKey("gpt-4o", "you are helpful", []Message{{Role: RoleUser, Content: "hello"}})
	key2 := CacheKey("gpt-4o", "you are helpful", []Message{{Role: RoleUser, Content: "hello"}})
	key3 := CacheKey("gpt-4o", "you are helpful", []Message{{Role: RoleUser, Content: "world"}})

	if key1 != key2 {
		t.Error("expected same keys for same inputs")
	}
	if key1 == key3 {
		t.Error("expected different keys for different inputs")
	}
}

func TestMemoryCache_SetGet(t *testing.T) {
	cache := NewMemoryCache(5 * time.Minute)
	ctx := context.Background()

	entry := &CacheEntry{
		Response:  Response{Text: "hello", Model: "gpt-4o", TokensIn: 10, TokensOut: 5},
		CreatedAt: time.Now(),
		TTL:       5 * time.Minute,
	}

	err := cache.Set(ctx, "test-key", entry)
	if err != nil {
		t.Fatalf("Set: %v", err)
	}

	got, err := cache.Get(ctx, "test-key")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got == nil {
		t.Fatal("expected non-nil entry")
	}
	if got.Response.Text != "hello" {
		t.Errorf("expected 'hello', got '%s'", got.Response.Text)
	}
}

func TestMemoryCache_Miss(t *testing.T) {
	cache := NewMemoryCache(5 * time.Minute)
	entry, err := cache.Get(context.Background(), "nonexistent")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if entry != nil {
		t.Fatal("expected nil for cache miss")
	}
}

func TestMemoryCache_Expired(t *testing.T) {
	cache := NewMemoryCache(5 * time.Minute)
	ctx := context.Background()

	cache.mu["key"] = &CacheEntry{
		Response:  Response{Text: "data"},
		CreatedAt: time.Now().Add(-10 * time.Minute),
		TTL:       5 * time.Minute,
	}

	entry, err := cache.Get(ctx, "key")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if entry != nil {
		t.Fatal("expected nil for expired entry")
	}
}

func TestMemoryCache_Delete(t *testing.T) {
	cache := NewMemoryCache(5 * time.Minute)
	ctx := context.Background()

	cache.Set(ctx, "k", &CacheEntry{Response: Response{Text: "v"}})
	cache.Delete(ctx, "k")

	entry, _ := cache.Get(ctx, "k")
	if entry != nil {
		t.Fatal("expected nil after delete")
	}
}

func TestMemoryCache_Clear(t *testing.T) {
	cache := NewMemoryCache(5 * time.Minute)
	ctx := context.Background()

	cache.Set(ctx, "a", &CacheEntry{Response: Response{Text: "1"}})
	cache.Set(ctx, "b", &CacheEntry{Response: Response{Text: "2"}})
	cache.Clear(ctx)

	entry, _ := cache.Get(ctx, "a")
	if entry != nil {
		t.Fatal("expected nil after clear")
	}
}

func TestNoopCache(t *testing.T) {
	cache := NewNoopCache()
	ctx := context.Background()

	entry, err := cache.Get(ctx, "key")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if entry != nil {
		t.Fatal("expected nil from noop cache")
	}

	err = cache.Set(ctx, "key", &CacheEntry{Response: Response{Text: "v"}})
	if err != nil {
		t.Fatalf("Set: %v", err)
	}
}

func TestCachedGateway_Hit(t *testing.T) {
	inner := &mockProvider{response: "expensive"}
	cache := NewMemoryCache(5 * time.Minute)
	cg := NewCachedGateway(inner, cache, 5*time.Minute)
	ctx := context.Background()

	req := Request{Messages: []Message{{Role: RoleUser, Content: "test"}}}

	resp1, err := cg.Generate(ctx, req)
	if err != nil {
		t.Fatalf("first call: %v", err)
	}
	if resp1.Text != "expensive test" {
		t.Errorf("expected 'expensive test', got '%s'", resp1.Text)
	}

	inner.response = "different" // shouldn't be used

	resp2, err := cg.Generate(ctx, req)
	if err != nil {
		t.Fatalf("second call: %v", err)
	}
	if resp2.Text != "expensive test" {
		t.Errorf("expected cached 'expensive test', got '%s'", resp2.Text)
	}
}

func TestCachedGateway_Miss(t *testing.T) {
	inner := &mockProvider{response: "fresh"}
	cg := NewCachedGateway(inner, NewMemoryCache(5*time.Minute), 5*time.Minute)
	ctx := context.Background()

	req1 := Request{Messages: []Message{{Role: RoleUser, Content: "q1"}}}
	req2 := Request{Messages: []Message{{Role: RoleUser, Content: "q2"}}}

	resp1, _ := cg.Generate(ctx, req1)
	resp2, _ := cg.Generate(ctx, req2)

	if resp1.Text == resp2.Text {
		t.Error("expected different responses for different queries")
	}
}

func TestCachedGateway_NilCache(t *testing.T) {
	inner := &mockProvider{response: "direct"}
	cg := NewCachedGateway(inner, nil, 0)
	ctx := context.Background()

	resp, err := cg.Generate(ctx, Request{
		Messages: []Message{{Role: RoleUser, Content: "test"}},
	})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if resp.Text != "direct test" {
		t.Errorf("expected 'direct test', got '%s'", resp.Text)
	}
}

func TestCacheEntry_IsExpired(t *testing.T) {
	e := &CacheEntry{CreatedAt: time.Now(), TTL: -1}
	if e.IsExpired() {
		t.Error("expected not expired for negative TTL")
	}

	e = &CacheEntry{CreatedAt: time.Now(), TTL: 0}
	if e.IsExpired() {
		t.Error("expected not expired for zero TTL")
	}

	e = &CacheEntry{CreatedAt: time.Now().Add(-1 * time.Hour), TTL: 5 * time.Minute}
	if !e.IsExpired() {
		t.Error("expected expired for old entry with short TTL")
	}
}
