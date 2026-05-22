package element

import (
	"context"

	"github.com/Raoon-Soluciones/bpmn-ai/pkg/store"
)

// ElementStore is the persistence interface that BPMN elements can access.
// It exposes only the methods that elements legitimately need.
type ElementStore interface {
	GetFlowsByInstance(ctx context.Context, instanceID string) ([]*store.FlowRecord, error)
}
