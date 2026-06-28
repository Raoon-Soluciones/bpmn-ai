package ai

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"
)

type Guardrail interface {
	Check(ctx context.Context, req *Request, resp *Response) error
}

type GuardrailChain struct {
	guards []Guardrail
}

func NewGuardrailChain(guards ...Guardrail) *GuardrailChain {
	return &GuardrailChain{guards: guards}
}

func (c *GuardrailChain) Add(g Guardrail) {
	c.guards = append(c.guards, g)
}

func (c *GuardrailChain) Before(ctx context.Context, req *Request) error {
	for _, g := range c.guards {
		if err := g.Check(ctx, req, nil); err != nil {
			return err
		}
	}
	return nil
}

func (c *GuardrailChain) After(ctx context.Context, req *Request, resp *Response) error {
	for _, g := range c.guards {
		if err := g.Check(ctx, req, resp); err != nil {
			return err
		}
	}
	return nil
}

// --- Timeout Guardrail ---

type TimeoutGuardrail struct {
	timeout time.Duration
}

func NewTimeoutGuardrail(timeout time.Duration) *TimeoutGuardrail {
	return &TimeoutGuardrail{timeout: timeout}
}

func (g *TimeoutGuardrail) Check(ctx context.Context, req *Request, resp *Response) error {
	if g.timeout <= 0 {
		return nil
	}
	_, ok := ctx.Deadline()
	if ok {
		return nil
	}
	return fmt.Errorf("context has no deadline, timeout guardrail requires one")
}

// --- Token Limit Guardrail ---

type TokenLimitGuardrail struct {
	maxTokens int
}

func NewTokenLimitGuardrail(maxTokens int) *TokenLimitGuardrail {
	return &TokenLimitGuardrail{maxTokens: maxTokens}
}

func (g *TokenLimitGuardrail) Check(ctx context.Context, req *Request, resp *Response) error {
	if req.MaxTokens > g.maxTokens {
		return fmt.Errorf("request max_tokens %d exceeds limit %d", req.MaxTokens, g.maxTokens)
	}
	return nil
}

// --- Cost Limiter Guardrail ---

type CostEstimator struct {
	PricePer1KInput  float64
	PricePer1KOutput float64
}

var DefaultCostEstimator = CostEstimator{
	PricePer1KInput:  0.01,  // $0.01 per 1K input tokens (GPT-4o-mini pricing)
	PricePer1KOutput: 0.03, // $0.03 per 1K output tokens
}

type CostLimiterGuardrail struct {
	maxCostUSD float64
	pricing    CostEstimator
}

func NewCostLimiterGuardrail(maxCostUSD float64, pricing CostEstimator) *CostLimiterGuardrail {
	return &CostLimiterGuardrail{
		maxCostUSD: maxCostUSD,
		pricing:    pricing,
	}
}

func (g *CostLimiterGuardrail) Check(ctx context.Context, req *Request, resp *Response) error {
	if resp == nil {
		totalChars := len(req.System)
		for _, m := range req.Messages {
			totalChars += len(m.Content)
		}
		estimatedTokens := totalChars / 4
		if estimatedTokens < 1 {
			estimatedTokens = 1
		}
		estimatedCost := float64(estimatedTokens) / 1000 * g.pricing.PricePer1KInput
		if estimatedCost > g.maxCostUSD {
			return &CostLimitError{
				Message:       fmt.Sprintf("estimated cost $%.4f exceeds limit $%.4f", estimatedCost, g.maxCostUSD),
				EstimatedCost: estimatedCost,
				MaxCost:       g.maxCostUSD,
			}
		}
		return nil
	}

	actualCost := float64(resp.TokensIn)/1000*g.pricing.PricePer1KInput +
		float64(resp.TokensOut)/1000*g.pricing.PricePer1KOutput
	if actualCost > g.maxCostUSD {
		return &CostLimitError{
			Message:       fmt.Sprintf("actual cost $%.4f exceeds limit $%.4f", actualCost, g.maxCostUSD),
			EstimatedCost: actualCost,
			MaxCost:       g.maxCostUSD,
		}
	}
	return nil
}

type CostLimitError struct {
	Message       string
	EstimatedCost float64
	MaxCost       float64
}

func (e *CostLimitError) Error() string {
	return e.Message
}

// --- PII Redactor Guardrail ---

type piiPattern struct {
	regex    *regexp.Regexp
	template string
}

type PIIRedactorGuardrail struct {
	mu       sync.RWMutex
	patterns []piiPattern
}

func NewPIIRedactorGuardrail() *PIIRedactorGuardrail {
	g := &PIIRedactorGuardrail{}
	g.addDefaultPatterns()
	return g
}

func (g *PIIRedactorGuardrail) addDefaultPatterns() {
	g.patterns = append(g.patterns,
		piiPattern{regexp.MustCompile(`[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}`), "[EMAIL]"},
		piiPattern{regexp.MustCompile(`\+?1?\s*\(?\d{3}\)?[\s.-]?\d{3}[\s.-]?\d{4}`), "[PHONE]"},
		piiPattern{regexp.MustCompile(`\d{3}-\d{2}-\d{4}`), "[SSN]"},
		piiPattern{regexp.MustCompile(`\b(?:\d{4}[-\s]?){3}\d{4}\b`), "[CC]"},
		piiPattern{regexp.MustCompile(`\b\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}\b`), "[IP]"},
	)
}

func (g *PIIRedactorGuardrail) AddPattern(regex *regexp.Regexp, template string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.patterns = append(g.patterns, piiPattern{regex, template})
}

func (g *PIIRedactorGuardrail) Check(ctx context.Context, req *Request, resp *Response) error {
	if req == nil {
		return nil
	}

	g.mu.RLock()
	cp := make([]piiPattern, len(g.patterns))
	copy(cp, g.patterns)
	g.mu.RUnlock()

	redact := func(s string) string {
		for _, p := range cp {
			s = p.regex.ReplaceAllString(s, p.template)
		}
		return s
	}

	req.System = redact(req.System)
	for i := range req.Messages {
		req.Messages[i].Content = redact(req.Messages[i].Content)
	}

	return nil
}

// --- Human In The Loop Guardrail ---

type HumanInTheLoopGuardrail struct {
	confidenceThreshold float64

	confidenceFn func(ctx context.Context, req *Request, resp *Response) (float64, error)
}

func NewHumanInTheLoopGuardrail(threshold float64, confidenceFn func(ctx context.Context, req *Request, resp *Response) (float64, error)) *HumanInTheLoopGuardrail {
	if threshold <= 0 {
		threshold = 0.8
	}
	return &HumanInTheLoopGuardrail{
		confidenceThreshold: threshold,
		confidenceFn:        confidenceFn,
	}
}

func (g *HumanInTheLoopGuardrail) Check(ctx context.Context, req *Request, resp *Response) error {
	if resp == nil {
		return nil
	}

	confidence, err := g.confidenceFn(ctx, req, resp)
	if err != nil {
		return err
	}
	if confidence < g.confidenceThreshold {
		return &HumanInTheLoopError{
			Message:    fmt.Sprintf("confidence %.2f below threshold %.2f", confidence, g.confidenceThreshold),
			Confidence: confidence,
			Threshold:  g.confidenceThreshold,
		}
	}
	return nil
}

func LowConfidenceChecker(ctx context.Context, req *Request, resp *Response) (float64, error) {
	responseLen := len(resp.Text)
	if responseLen == 0 {
		return 0.0, nil
	}
	if responseLen < 3 {
		return 0.3, nil
	}
	if strings.Contains(resp.Text, "I'm not sure") || strings.Contains(resp.Text, "I don't know") {
		return 0.4, nil
	}
	if len(resp.Text) > 20 {
		words := len(strings.Fields(resp.Text))
		if words < 3 {
			return 0.5, nil
		}
	}
	return 0.9, nil
}

type HumanInTheLoopError struct {
	Message    string
	Confidence float64
	Threshold  float64
}

func (e *HumanInTheLoopError) Error() string {
	return e.Message
}

func IsHumanInTheLoopError(err error) bool {
	_, ok := err.(*HumanInTheLoopError)
	return ok
}

func IsCostLimitError(err error) bool {
	_, ok := err.(*CostLimitError)
	return ok
}
