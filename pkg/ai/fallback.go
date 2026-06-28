package ai

import (
	"context"
	"fmt"
	"log"
)

type FallbackGateway struct {
	primary   Gateway
	secondary Gateway
}

func NewFallbackGateway(primary, secondary Gateway) *FallbackGateway {
	return &FallbackGateway{
		primary:   primary,
		secondary: secondary,
	}
}

func (g *FallbackGateway) Generate(ctx context.Context, req Request) (Response, error) {
	resp, err := g.primary.Generate(ctx, req)
	if err == nil {
		return resp, nil
	}

	log.Printf("AI primary failed: %v — falling back to secondary", err)

	resp, err = g.secondary.Generate(ctx, req)
	if err == nil {
		return resp, nil
	}

	return Response{}, fmt.Errorf("all AI providers failed: primary=%v, secondary=%v", err, err)
}
