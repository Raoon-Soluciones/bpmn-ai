package ai

import (
	"testing"
)

func TestPromptManager_RegisterAndGet(t *testing.T) {
	m := NewPromptManager()
	err := m.Register("classify", "Classify this: {{input}}")
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	tmpl, ok := m.Get("classify", 0)
	if !ok {
		t.Fatal("expected to find template")
	}
	if tmpl.Version != 1 {
		t.Errorf("expected version 1, got %d", tmpl.Version)
	}
	if tmpl.Body != "Classify this: {{input}}" {
		t.Errorf("unexpected body: '%s'", tmpl.Body)
	}
}

func TestPromptManager_Versioning(t *testing.T) {
	m := NewPromptManager()
	m.Register("prompt", "v1")
	m.Register("prompt", "v2")

	v1, ok := m.Get("prompt", 1)
	if !ok || v1.Body != "v1" {
		t.Error("expected v1 body")
	}
	v2, ok := m.Get("prompt", 2)
	if !ok || v2.Body != "v2" {
		t.Error("expected v2 body")
	}
	latest, ok := m.Get("prompt", 0)
	if !ok || latest.Body != "v2" {
		t.Error("expected latest to be v2")
	}
}

func TestPromptManager_ResolveByName(t *testing.T) {
	m := NewPromptManager()
	m.Register("greet", "Hello {{name}}")

	body, hash, ok := m.Resolve("greet")
	if !ok {
		t.Fatal("expected to resolve")
	}
	if body != "Hello {{name}}" {
		t.Errorf("unexpected body: '%s'", body)
	}
	if hash == "" {
		t.Error("expected non-empty hash")
	}
}

func TestPromptManager_ResolveByVersion(t *testing.T) {
	m := NewPromptManager()
	m.Register("t", "version1")
	m.Register("t", "version2")

	body, _, ok := m.Resolve("t@1")
	if !ok || body != "version1" {
		t.Errorf("expected 'version1', got '%s'", body)
	}
}

func TestPromptManager_ResolveByHash(t *testing.T) {
	m := NewPromptManager()
	m.Register("t", "content")
	tmpl, _ := m.Get("t", 0)

	body, hash, ok := m.Resolve("t#" + tmpl.SHA256)
	if !ok {
		t.Fatal("expected to resolve by hash")
	}
	if body != "content" {
		t.Errorf("expected 'content', got '%s'", body)
	}
	if hash != tmpl.SHA256 {
		t.Errorf("expected hash '%s', got '%s'", tmpl.SHA256, hash)
	}
}

func TestPromptManager_ResolveMissing(t *testing.T) {
	m := NewPromptManager()
	_, _, ok := m.Resolve("nonexistent")
	if ok {
		t.Fatal("expected not found")
	}
}

func TestPromptManager_GetByHash(t *testing.T) {
	m := NewPromptManager()
	m.Register("x", "hello")
	tmpl, _ := m.Get("x", 0)

	found, ok := m.GetByHash(tmpl.SHA256)
	if !ok || found.Body != "hello" {
		t.Error("expected to find by hash")
	}
}

func TestPromptManager_List(t *testing.T) {
	m := NewPromptManager()
	m.Register("a", "body a")
	m.Register("b", "body b")
	names := m.List()
	if len(names) != 2 {
		t.Errorf("expected 2 names, got %d", len(names))
	}
}

func TestNewPromptTemplate_Hash(t *testing.T) {
	tmpl := NewPromptTemplate("test", "body", 1)
	if tmpl.SHA256 == "" {
		t.Error("expected non-empty hash")
	}
	if len(tmpl.SHA256) != 16 {
		t.Errorf("expected 16-char hash, got %d", len(tmpl.SHA256))
	}
}

func TestHashPrompt_Deterministic(t *testing.T) {
	h1 := hashPrompt("hello world")
	h2 := hashPrompt("hello world")
	if h1 != h2 {
		t.Error("expected deterministic hashing")
	}
}
