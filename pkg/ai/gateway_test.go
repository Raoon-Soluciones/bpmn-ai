package ai

import (
	"context"
	"errors"
	"testing"
)

func TestRequest_WithDefaults(t *testing.T) {
	r := Request{}
	r = r.WithDefaults()
	if r.MaxTokens != 4096 {
		t.Errorf("expected MaxTokens=4096, got %d", r.MaxTokens)
	}
	if r.Temperature != 0.7 {
		t.Errorf("expected Temperature=0.7, got %f", r.Temperature)
	}
}

func TestRequest_WithDefaults_PreservesValues(t *testing.T) {
	r := Request{MaxTokens: 1000, Temperature: 0.5}
	r = r.WithDefaults()
	if r.MaxTokens != 1000 {
		t.Errorf("expected MaxTokens=1000, got %d", r.MaxTokens)
	}
	if r.Temperature != 0.5 {
		t.Errorf("expected Temperature=0.5, got %f", r.Temperature)
	}
}

type mockProvider struct {
	response string
	err      error
}

func (m *mockProvider) Generate(_ context.Context, req Request) (Response, error) {
	if m.err != nil {
		return Response{}, m.err
	}
	return Response{
		Text:       m.response + " " + req.Messages[0].Content,
		Model:      "mock",
		TokensIn:   10,
		TokensOut:  5,
		DurationMs: 1,
	}, nil
}

func TestProviderRegistry(t *testing.T) {
	r := NewProviderRegistry()
	r.Register("mock", func(_, _ string) (Gateway, error) {
		return &mockProvider{response: "hello"}, nil
	})

	if !r.Has("mock") {
		t.Error("expected Has('mock') to be true")
	}

	g, err := r.Create("mock", "", "")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	resp, err := g.Generate(context.Background(), Request{
		Messages: []Message{{Role: RoleUser, Content: "world"}},
	})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if resp.Text != "hello world" {
		t.Errorf("expected 'hello world', got '%s'", resp.Text)
	}
}

func TestProviderRegistry_Unknown(t *testing.T) {
	r := NewProviderRegistry()
	_, err := r.Create("unknown", "", "")
	if err == nil {
		t.Fatal("expected error for unknown provider")
	}
}

func TestFallbackGateway_SuccessPrimary(t *testing.T) {
	primary := &mockProvider{response: "primary"}
	secondary := &mockProvider{response: "secondary"}
	fb := NewFallbackGateway(primary, secondary)

	resp, err := fb.Generate(context.Background(), Request{
		Messages: []Message{{Role: RoleUser, Content: "test"}},
	})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if resp.Text != "primary test" {
		t.Errorf("expected 'primary test', got '%s'", resp.Text)
	}
}

func TestFallbackGateway_Fallback(t *testing.T) {
	primary := &mockProvider{err: errors.New("primary down")}
	secondary := &mockProvider{response: "secondary"}
	fb := NewFallbackGateway(primary, secondary)

	resp, err := fb.Generate(context.Background(), Request{
		Messages: []Message{{Role: RoleUser, Content: "test"}},
	})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if resp.Text != "secondary test" {
		t.Errorf("expected 'secondary test', got '%s'", resp.Text)
	}
}

func TestFallbackGateway_AllFail(t *testing.T) {
	primary := &mockProvider{err: errors.New("primary down")}
	secondary := &mockProvider{err: errors.New("secondary down")}
	fb := NewFallbackGateway(primary, secondary)

	_, err := fb.Generate(context.Background(), Request{
		Messages: []Message{{Role: RoleUser, Content: "test"}},
	})
	if err == nil {
		t.Fatal("expected error when all providers fail")
	}
}

func TestGuardrailChain(t *testing.T) {
	called := false
	g := &testGuardrail{fn: func() error {
		called = true
		return nil
	}}
	chain := NewGuardrailChain(g)

	req := &Request{}
	err := chain.Before(context.Background(), req)
	if err != nil {
		t.Fatalf("Before: %v", err)
	}
	if !called {
		t.Error("expected guardrail to be called")
	}
}

func TestGuardrailChain_Error(t *testing.T) {
	g := &testGuardrail{fn: func() error {
		return errors.New("guardrail blocked")
	}}
	chain := NewGuardrailChain(g)

	err := chain.Before(context.Background(), &Request{})
	if err == nil {
		t.Fatal("expected guardrail error")
	}
}

func TestTokenLimitGuardrail(t *testing.T) {
	g := NewTokenLimitGuardrail(100)

	err := g.Check(context.Background(), &Request{MaxTokens: 50}, nil)
	if err != nil {
		t.Errorf("expected no error for 50 < 100: %v", err)
	}

	err = g.Check(context.Background(), &Request{MaxTokens: 150}, nil)
	if err == nil {
		t.Error("expected error for 150 > 100")
	}
}

type testGuardrail struct {
	fn func() error
}

func (g *testGuardrail) Check(_ context.Context, _ *Request, _ *Response) error {
	return g.fn()
}
