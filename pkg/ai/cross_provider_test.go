package ai

import (
	"context"
	"errors"
	"testing"
)

func TestCrossProviderGateway_SuccessPrimary(t *testing.T) {
	gw := NewCrossProviderGateway([]ProviderOption{
		{Gateway: &mockProvider{response: "primary"}, Model: ""},
		{Gateway: &mockProvider{response: "fallback"}, Model: ""},
	})
	resp, err := gw.Generate(context.Background(), Request{
		Messages: []Message{{Role: RoleUser, Content: "test"}},
	})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if resp.Text != "primary test" {
		t.Errorf("expected 'primary test', got '%s'", resp.Text)
	}
}

func TestCrossProviderGateway_Fallback(t *testing.T) {
	gw := NewCrossProviderGateway([]ProviderOption{
		{Gateway: &mockProvider{err: errors.New("primary down")}, Model: ""},
		{Gateway: &mockProvider{response: "fallback"}, Model: ""},
	})
	resp, err := gw.Generate(context.Background(), Request{
		Messages: []Message{{Role: RoleUser, Content: "test"}},
	})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if resp.Text != "fallback test" {
		t.Errorf("expected 'fallback test', got '%s'", resp.Text)
	}
}

func TestCrossProviderGateway_ModelOverride(t *testing.T) {
	gw := NewCrossProviderGateway([]ProviderOption{
		{Gateway: &mockProvider{err: errors.New("fail")}, Model: "gpt-4o"},
		{Gateway: &mockProvider{response: "ok"}, Model: "gpt-4o-mini"},
	})
	resp, err := gw.Generate(context.Background(), Request{
		Messages: []Message{{Role: RoleUser, Content: "hello"}},
	})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if resp.Text != "ok hello" {
		t.Errorf("expected 'ok hello', got '%s'", resp.Text)
	}
}

func TestCrossProviderGateway_AllFail(t *testing.T) {
	gw := NewCrossProviderGateway([]ProviderOption{
		{Gateway: &mockProvider{err: errors.New("fail1")}},
		{Gateway: &mockProvider{err: errors.New("fail2")}},
	})
	_, err := gw.Generate(context.Background(), Request{
		Messages: []Message{{Role: RoleUser, Content: "test"}},
	})
	if err == nil {
		t.Fatal("expected error when all providers fail")
	}
}

func TestCrossProviderGateway_NoProviders(t *testing.T) {
	gw := NewCrossProviderGateway(nil)
	_, err := gw.Generate(context.Background(), Request{
		Messages: []Message{{Role: RoleUser, Content: "test"}},
	})
	if err == nil {
		t.Fatal("expected error with no providers")
	}
}

func TestCrossProviderGateway_AddOption(t *testing.T) {
	gw := NewCrossProviderGateway(nil)
	gw.AddOption(&mockProvider{response: "added"}, "custom-model")
	resp, err := gw.Generate(context.Background(), Request{
		Messages: []Message{{Role: RoleUser, Content: "x"}},
	})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if resp.Text != "added x" {
		t.Errorf("expected 'added x', got '%s'", resp.Text)
	}
}
