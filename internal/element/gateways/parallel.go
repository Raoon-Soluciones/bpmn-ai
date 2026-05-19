package gateways

import (
	"context"

	"github.com/Raoon-Soluciones/bpmn-ai/internal/element"
	"github.com/Raoon-Soluciones/bpmn-ai/pkg/bpmn"
	"github.com/Raoon-Soluciones/bpmn-ai/pkg/store"
)

// ParallelGateway implements the BPMN parallel gateway.
type ParallelGateway struct {
	id          string
	name        string
	gatewayType bpmn.GatewayType
}

// NewParallelGateway creates a new parallel gateway element.
func NewParallelGateway(elem bpmn.Element) (element.Element, error) {
	return &ParallelGateway{
		id:          elem.ID,
		name:        elem.Name,
		gatewayType: elem.GatewayType,
	}, nil
}

// ID returns the element ID.
func (g *ParallelGateway) ID() string {
	return g.id
}

// Type returns the element type.
func (g *ParallelGateway) Type() bpmn.ElementType {
	return bpmn.ElementTypeParallelGateway
}

// Execute routes to all outgoing flows (diverging) or waits for all incoming (converging).
func (g *ParallelGateway) Execute(_ context.Context, execCtx element.ExecutionContext) element.ExecutionResult {
	flow := execCtx.Flow()
	flow.Status = store.FlowStatusCompleted

	// For diverging: route to all outgoing flows
	// For converging: wait until all incoming flows are complete
	elem, ok := execCtx.Element()
	if !ok {
		return element.ExecutionResult{
			Action:   element.ActionError,
			FlowData: flow,
			Error:    elementNotFoundError(g.id),
		}
	}

	// Diverging: route to all flows
	if len(elem.OutgoingFlows) > 1 {
		return element.ExecutionResult{
			Action:   element.ActionRoute,
			FlowData: flow,
			// No filters = route to all
		}
	}

	// Converging: check if all incoming flows are complete
	if len(elem.IncomingFlows) > 1 {
		allComplete := g.areAllIncomingComplete(execCtx)
		if !allComplete {
			return element.ExecutionResult{
				Action:   element.ActionWait,
				FlowData: flow,
			}
		}
	}

	return element.ExecutionResult{
		Action:   element.ActionRoute,
		FlowData: flow,
	}
}

// GatewayType returns the gateway type.
func (g *ParallelGateway) GatewayType() bpmn.GatewayType {
	return g.gatewayType
}

// DefaultFlowID returns empty (parallel gateways don't have defaults).
func (g *ParallelGateway) DefaultFlowID() string {
	return ""
}

// Conditions returns nil (parallel gateways don't have conditions).
func (g *ParallelGateway) Conditions() map[string]string {
	return nil
}

func (g *ParallelGateway) areAllIncomingComplete(execCtx element.ExecutionContext) bool {
	_, ok := execCtx.Element()
	if !ok {
		return false
	}

	// Check all incoming flows are complete
	// This would need access to flow records from the store
	// Simplified for now - in production, query store for flow statuses
	_ = execCtx.Store()

	// For now, assume all complete if we reached here
	return true
}

func elementNotFoundError(id string) error {
	return &elementNotFound{id: id}
}

type elementNotFound struct {
	id string
}

func (e *elementNotFound) Error() string {
	return "element not found: " + e.id
}
