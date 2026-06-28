package ai

import (
	"context"
	"strings"
	"testing"
)

func TestCostLimiterGuardrail_Before_UnderBudget(t *testing.T) {
	g := NewCostLimiterGuardrail(1.0, DefaultCostEstimator)
	err := g.Check(context.Background(), &Request{
		System: "short",
		Messages: []Message{
			{Role: RoleUser, Content: "hello"},
		},
	}, nil)
	if err != nil {
		t.Fatalf("expected no error for small request: %v", err)
	}
}

func TestCostLimiterGuardrail_After_UnderBudget(t *testing.T) {
	g := NewCostLimiterGuardrail(1.0, DefaultCostEstimator)
	err := g.Check(context.Background(), &Request{}, &Response{
		TokensIn:  100,
		TokensOut: 50,
	})
	if err != nil {
		t.Fatalf("expected no error for cheap response: %v", err)
	}
}

func TestCostLimiterGuardrail_After_OverBudget(t *testing.T) {
	g := NewCostLimiterGuardrail(0.001, DefaultCostEstimator)
	err := g.Check(context.Background(), &Request{}, &Response{
		TokensIn:  1000,
		TokensOut: 500,
	})
	if err == nil {
		t.Fatal("expected error for expensive response")
	}
	if !IsCostLimitError(err) {
		t.Errorf("expected CostLimitError, got %T", err)
	}
}

func TestPIIRedactorGuardrail_Email(t *testing.T) {
	g := NewPIIRedactorGuardrail()
	req := &Request{
		System: "Contact support at help@example.com for assistance",
		Messages: []Message{
			{Role: RoleUser, Content: "My email is john.doe@test.co.uk"},
		},
	}
	err := g.Check(context.Background(), req, nil)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if req.System != "Contact support at [EMAIL] for assistance" {
		t.Errorf("expected redacted system, got: %s", req.System)
	}
	if req.Messages[0].Content != "My email is [EMAIL]" {
		t.Errorf("expected redacted message, got: %s", req.Messages[0].Content)
	}
}

func TestPIIRedactorGuardrail_Phone(t *testing.T) {
	g := NewPIIRedactorGuardrail()
	req := &Request{
		Messages: []Message{
			{Role: RoleUser, Content: "Call me at 555-123-4567"},
		},
	}
	err := g.Check(context.Background(), req, nil)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if !strings.Contains(req.Messages[0].Content, "[PHONE]") {
		t.Errorf("expected [PHONE] in redacted message, got: %s", req.Messages[0].Content)
	}
}

func TestPIIRedactorGuardrail_SSN(t *testing.T) {
	g := NewPIIRedactorGuardrail()
	req := &Request{
		Messages: []Message{
			{Role: RoleUser, Content: "My SSN is 123-45-6789"},
		},
	}
	err := g.Check(context.Background(), req, nil)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if !strings.Contains(req.Messages[0].Content, "[SSN]") {
		t.Errorf("expected [SSN] in redacted message, got: %s", req.Messages[0].Content)
	}
}

func TestPIIRedactorGuardrail_MultiplePatterns(t *testing.T) {
	g := NewPIIRedactorGuardrail()
	req := &Request{
		Messages: []Message{
			{Role: RoleUser, Content: "Email: a@b.com, IP: 192.168.1.1"},
		},
	}
	err := g.Check(context.Background(), req, nil)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	content := req.Messages[0].Content
	if !strings.Contains(content, "[EMAIL]") {
		t.Errorf("expected [EMAIL], got: %s", content)
	}
	if !strings.Contains(content, "[IP]") {
		t.Errorf("expected [IP], got: %s", content)
	}
}

func TestPIIRedactorGuardrail_NilRequest(t *testing.T) {
	g := NewPIIRedactorGuardrail()
	err := g.Check(context.Background(), nil, nil)
	if err != nil {
		t.Fatalf("expected no error for nil request: %v", err)
	}
}

func TestHumanInTheLoopGuardrail_HighConfidence(t *testing.T) {
	g := NewHumanInTheLoopGuardrail(0.5, func(_ context.Context, _ *Request, _ *Response) (float64, error) {
		return 0.9, nil
	})
	err := g.Check(context.Background(), &Request{}, &Response{
		Text: "This is a confident response with enough words.",
	})
	if err != nil {
		t.Fatalf("expected no error for high confidence: %v", err)
	}
}

func TestHumanInTheLoopGuardrail_LowConfidence(t *testing.T) {
	g := NewHumanInTheLoopGuardrail(0.8, func(_ context.Context, _ *Request, _ *Response) (float64, error) {
		return 0.3, nil
	})
	err := g.Check(context.Background(), &Request{}, &Response{Text: "yes"})
	if err == nil {
		t.Fatal("expected error for low confidence")
	}
	if !IsHumanInTheLoopError(err) {
		t.Errorf("expected HumanInTheLoopError, got %T", err)
	}
}

func TestHumanInTheLoopGuardrail_NilResponse(t *testing.T) {
	g := NewHumanInTheLoopGuardrail(0.8, func(_ context.Context, _ *Request, _ *Response) (float64, error) {
		return 0.9, nil
	})
	err := g.Check(context.Background(), &Request{}, nil)
	if err != nil {
		t.Fatalf("expected no error for nil response: %v", err)
	}
}

func TestLowConfidenceChecker_ShortResponse(t *testing.T) {
	confidence, err := LowConfidenceChecker(context.Background(), &Request{}, &Response{Text: ""})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if confidence != 0.0 {
		t.Errorf("expected 0.0 for empty response, got %f", confidence)
	}
}

func TestLowConfidenceChecker_UncertainResponse(t *testing.T) {
	confidence, err := LowConfidenceChecker(context.Background(), &Request{}, &Response{Text: "I'm not sure about this"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if confidence != 0.4 {
		t.Errorf("expected 0.4 for 'I\\'m not sure', got %f", confidence)
	}
}

func TestIsHumanInTheLoopError(t *testing.T) {
	err := &HumanInTheLoopError{Message: "test"}
	if !IsHumanInTheLoopError(err) {
		t.Error("expected IsHumanInTheLoopError to return true")
	}
	if IsHumanInTheLoopError(nil) {
		t.Error("expected IsHumanInTheLoopError(nil) to return false")
	}
}

func TestIsCostLimitError(t *testing.T) {
	err := &CostLimitError{Message: "test"}
	if !IsCostLimitError(err) {
		t.Error("expected IsCostLimitError to return true")
	}
	if IsCostLimitError(nil) {
		t.Error("expected IsCostLimitError(nil) to return false")
	}
}

func TestGuardrailChain_BeforeMultiple(t *testing.T) {
	called1, called2 := false, false
	chain := NewGuardrailChain(
		&testGuardrail{fn: func() error { called1 = true; return nil }},
		&testGuardrail{fn: func() error { called2 = true; return nil }},
	)
	err := chain.Before(context.Background(), &Request{})
	if err != nil {
		t.Fatalf("Before: %v", err)
	}
	if !called1 || !called2 {
		t.Error("expected both guardrails to be called")
	}
}

func TestGuardrailChain_AfterMultiple(t *testing.T) {
	called1, called2 := false, false
	chain := NewGuardrailChain(
		&testGuardrail{fn: func() error { called1 = true; return nil }},
		&testGuardrail{fn: func() error { called2 = true; return nil }},
	)
	err := chain.After(context.Background(), &Request{}, &Response{})
	if err != nil {
		t.Fatalf("After: %v", err)
	}
	if !called1 || !called2 {
		t.Error("expected both guardrails to be called")
	}
}

func TestGuardrailChain_BeforeShortCircuit(t *testing.T) {
	called2 := false
	chain := NewGuardrailChain(
		&testGuardrail{fn: func() error { return &CostLimitError{Message: "over budget"} }},
		&testGuardrail{fn: func() error { called2 = true; return nil }},
	)
	err := chain.Before(context.Background(), &Request{})
	if err == nil {
		t.Fatal("expected error")
	}
	if called2 {
		t.Error("expected second guardrail NOT to be called after first error")
	}
}
