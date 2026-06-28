package ai

import (
	"context"
	"fmt"
	"log"
)

type ProviderOption struct {
	Gateway Gateway
	Model   string
}

type CrossProviderGateway struct {
	options []ProviderOption
}

func NewCrossProviderGateway(options []ProviderOption) *CrossProviderGateway {
	return &CrossProviderGateway{options: options}
}

func (g *CrossProviderGateway) Generate(ctx context.Context, req Request) (Response, error) {
	if len(g.options) == 0 {
		return Response{}, fmt.Errorf("cross-provider: no providers configured")
	}

	var lastErr error
	for i, opt := range g.options {
		r := req
		if opt.Model != "" {
			r.Model = opt.Model
		}
		resp, err := opt.Gateway.Generate(ctx, r)
		if err == nil {
			if i > 0 {
				log.Printf("cross-provider: primary failed (%v), succeeded with fallback #%d (model=%s)", lastErr, i, r.Model)
			}
			return resp, nil
		}
		lastErr = err
	}

	return Response{}, fmt.Errorf("cross-provider: all %d providers failed — last error: %w", len(g.options), lastErr)
}

func (g *CrossProviderGateway) AddOption(gw Gateway, model string) {
	g.options = append(g.options, ProviderOption{Gateway: gw, Model: model})
}
