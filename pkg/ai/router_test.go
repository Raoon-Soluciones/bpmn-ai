package ai

import (
	"context"
	"testing"
)

func TestModelRouter_ResolveDefault(t *testing.T) {
	pool := NewProviderPool()
	pool.Add("openai", &mockProvider{response: "ok"})
	router := NewModelRouter(pool)

	gw, model, err := router.Resolve(context.Background(), "", "", false, false)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if gw == nil {
		t.Fatal("expected non-nil gateway")
	}
	if model != "openai/gpt-4o" {
		t.Errorf("expected default model 'openai/gpt-4o', got '%s'", model)
	}
}

func TestModelRouter_ResolveProfile(t *testing.T) {
	pool := NewProviderPool()
	pool.Add("openai", &mockProvider{response: "ok"})
	router := NewModelRouter(pool)

	gw, model, err := router.Resolve(context.Background(), "fast", "", false, false)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if model != "openai/gpt-4o-mini" {
		t.Errorf("expected 'openai/gpt-4o-mini' for fast profile, got '%s'", model)
	}
	_ = gw
}

func TestModelRouter_ResolveModelOverride(t *testing.T) {
	pool := NewProviderPool()
	pool.Add("openai", &mockProvider{response: "ok"})
	router := NewModelRouter(pool)

	gw, model, err := router.Resolve(context.Background(), "", "openai/gpt-4o-mini", false, false)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if model != "openai/gpt-4o-mini" {
		t.Errorf("expected 'openai/gpt-4o-mini', got '%s'", model)
	}
	_ = gw
}

func TestModelRouter_ResolveShortAlias(t *testing.T) {
	pool := NewProviderPool()
	pool.Add("openai", &mockProvider{response: "ok"})
	router := NewModelRouter(pool)

	gw, model, err := router.Resolve(context.Background(), "", "gpt-4o-mini", false, false)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if model != "openai/gpt-4o-mini" {
		t.Errorf("expected 'openai/gpt-4o-mini', got '%s'", model)
	}
	_ = gw
}

func TestModelRouter_ResolveCheapProfile(t *testing.T) {
	pool := NewProviderPool()
	pool.Add("openai", &mockProvider{response: "ok"})
	router := NewModelRouter(pool)

	gw, model, err := router.Resolve(context.Background(), "cheap", "", false, false)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if model == "" {
		t.Fatal("expected non-empty model")
	}
	_ = gw
}

func TestModelRouter_ResolveComplexProfile(t *testing.T) {
	pool := NewProviderPool()
	pool.Add("openai", &mockProvider{response: "ok"})
	router := NewModelRouter(pool)

	gw, model, err := router.Resolve(context.Background(), "complex", "", true, true)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if model == "" {
		t.Fatal("expected non-empty model")
	}
	_ = gw
}

func TestModelRouter_NoProviders(t *testing.T) {
	pool := NewProviderPool()
	router := NewModelRouter(pool)

	_, _, err := router.Resolve(context.Background(), "", "", false, false)
	if err == nil {
		t.Fatal("expected error with no providers")
	}
}

func TestModelRouter_AddCustomModelAndProfile(t *testing.T) {
	pool := NewProviderPool()
	pool.Add("openai", &mockProvider{response: "ok"})
	router := NewModelRouter(pool)

	router.AddModel(ModelInfo{
		Provider:   "openai",
		Name:       "my-custom-model",
		ContextWin: 32000,
		MaxOutput:  4096,
	})
	router.AddProfile("custom", Profile{
		Model:     "openai/my-custom-model",
		MaxTokens: 2048,
		Priority:  "cost",
	})

	gw, model, err := router.Resolve(context.Background(), "custom", "", false, false)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if model != "openai/my-custom-model" {
		t.Errorf("expected 'openai/my-custom-model', got '%s'", model)
	}
	_ = gw
}

func TestModelRouter_ResolveProviderPrefix(t *testing.T) {
	pool := NewProviderPool()
	pool.Add("anthropic", &mockProvider{response: "claude"})
	router := NewModelRouter(pool)

	gw, model, err := router.Resolve(context.Background(), "", "anthropic/claude-sonnet-4-6", false, false)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if model != "anthropic/claude-sonnet-4-6" {
		t.Errorf("expected 'anthropic/claude-sonnet-4-6', got '%s'", model)
	}
	_ = gw
}

func TestModelRouter_ResolveNonCatalogModel(t *testing.T) {
	pool := NewProviderPool()
	pool.Add("openai", &mockProvider{response: "ok"})
	router := NewModelRouter(pool)

	gw, model, err := router.Resolve(context.Background(), "", "openai/unknown-model", false, false)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if model != "unknown-model" {
		t.Errorf("expected 'unknown-model' (stripped prefix), got '%s'", model)
	}
	_ = gw
}

func TestModelRouter_ListModels(t *testing.T) {
	pool := NewProviderPool()
	router := NewModelRouter(pool)
	models := router.ListModels()
	if len(models) < 30 {
		t.Errorf("expected at least 30 models in catalog, got %d", len(models))
	}
}

func TestModelRouter_ListProfiles(t *testing.T) {
	pool := NewProviderPool()
	router := NewModelRouter(pool)
	profiles := router.ListProfiles()
	if len(profiles) == 0 {
		t.Fatal("expected at least 1 profile")
	}
	if _, ok := profiles["complex"]; !ok {
		t.Error("expected 'complex' profile")
	}
}

func TestModelRouter_ResolveUnknownProfile(t *testing.T) {
	pool := NewProviderPool()
	pool.Add("openai", &mockProvider{response: "ok"})
	router := NewModelRouter(pool)

	gw, model, err := router.Resolve(context.Background(), "nonexistent", "", false, false)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if model == "" {
		t.Fatal("expected non-empty model for fallback to auto profile")
	}
	_ = gw
}

func TestModelRouter_MultiProviderFallback(t *testing.T) {
	pool := NewProviderPool()
	pool.Add("openai", &mockProvider{response: "openai"})
	pool.Add("anthropic", &mockProvider{response: "anthropic"})
	router := NewModelRouter(pool)

	// Request a model whose provider isn't in the pool
	gw, model, err := router.Resolve(context.Background(), "", "openai/gpt-4o", false, false)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if model != "openai/gpt-4o" {
		t.Errorf("expected 'openai/gpt-4o', got '%s'", model)
	}
	_ = gw
}

func TestModelRouter_AddAlias(t *testing.T) {
	pool := NewProviderPool()
	pool.Add("openai", &mockProvider{response: "ok"})
	router := NewModelRouter(pool)
	router.AddAlias("my-model", "openai/gpt-4o")

	gw, model, err := router.Resolve(context.Background(), "", "my-model", false, false)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if model != "openai/gpt-4o" {
		t.Errorf("expected 'openai/gpt-4o', got '%s'", model)
	}
	_ = gw
}

func TestProviderPool_AddGet(t *testing.T) {
	pool := NewProviderPool()
	gw := &mockProvider{response: "hello"}
	pool.Add("test", gw)

	got, ok := pool.Get("test")
	if !ok {
		t.Fatal("expected to find provider")
	}
	resp, err := got.Generate(context.Background(), Request{
		Messages: []Message{{Role: RoleUser, Content: "world"}},
	})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if resp.Text != "hello world" {
		t.Errorf("expected 'hello world', got '%s'", resp.Text)
	}
}

func TestProviderPool_GetMissing(t *testing.T) {
	pool := NewProviderPool()
	_, ok := pool.Get("nonexistent")
	if ok {
		t.Fatal("expected not found")
	}
}

func TestProviderPool_List(t *testing.T) {
	pool := NewProviderPool()
	pool.Add("a", &mockProvider{response: "1"})
	pool.Add("b", &mockProvider{response: "2"})
	names := pool.List()
	if len(names) != 2 {
		t.Errorf("expected 2 names, got %d", len(names))
	}
}

func TestModelRouter_LabAlias(t *testing.T) {
	pool := NewProviderPool()
	pool.Add("openrouter", &mockProvider{response: "or"})
	router := NewModelRouter(pool)

	// zhipuai/glm-5.2 → lab alias "zhipuai" → "openrouter"
	gw, model, err := router.Resolve(context.Background(), "", "zhipuai/glm-5.2", false, false)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if model != "glm-5.2" {
		t.Errorf("expected 'glm-5.2', got '%s'", model)
	}
	// Verify it used openrouter
	resp, _ := gw.Generate(context.Background(), Request{
		Messages: []Message{{Role: RoleUser, Content: "test"}},
	})
	if resp.Text != "or test" {
		t.Errorf("expected openrouter response 'or test', got '%s'", resp.Text)
	}
}

func TestModelRouter_LabAlias_FallbackOrder(t *testing.T) {
	pool := NewProviderPool()
	pool.Add("anthropic", &mockProvider{response: "claude"})
	pool.Add("openrouter", &mockProvider{response: "or"})
	router := NewModelRouter(pool)

	// Direct provider match should take priority over lab alias
	gw, model, err := router.Resolve(context.Background(), "", "anthropic/claude-sonnet-4-6", false, false)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if model != "anthropic/claude-sonnet-4-6" {
		t.Errorf("expected 'anthropic/claude-sonnet-4-6', got '%s'", model)
	}
	resp, _ := gw.Generate(context.Background(), Request{
		Messages: []Message{{Role: RoleUser, Content: "test"}},
	})
	if resp.Text != "claude test" {
		t.Errorf("expected anthropic response 'claude test', got '%s'", resp.Text)
	}
}

func TestModelRouter_ExtraProvider(t *testing.T) {
	pool := NewProviderPool()
	pool.Add("custom-vendor", &mockProvider{response: "custom"})
	router := NewModelRouter(pool)

	gw, model, err := router.Resolve(context.Background(), "", "custom-vendor/custom-model-v2", false, false)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if model != "custom-model-v2" {
		t.Errorf("expected 'custom-model-v2', got '%s'", model)
	}
	resp, _ := gw.Generate(context.Background(), Request{
		Messages: []Message{{Role: RoleUser, Content: "hello"}},
	})
	if resp.Text != "custom hello" {
		t.Errorf("expected 'custom hello', got '%s'", resp.Text)
	}
}

func TestSplitModelName(t *testing.T) {
	prov, model := splitModelName("openai/gpt-4o")
	if prov != "openai" || model != "gpt-4o" {
		t.Errorf("expected (openai, gpt-4o), got (%s, %s)", prov, model)
	}

	prov, model = splitModelName("gpt-4o")
	if prov != "" || model != "gpt-4o" {
		t.Errorf("expected ('', gpt-4o), got (%s, %s)", prov, model)
	}
}
