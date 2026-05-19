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
func (g *ParallelGateway) Execute(ctx context.Context, execCtx element.ExecutionContext) element.ExecutionResult {
	flow := execCtx.Flow()
	flow.Status = store.FlowStatusCompleted

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
		}
	}

	// Converging: check if all incoming flows have reached this gateway
	if len(elem.IncomingFlows) > 1 {
		allComplete := g.areAllIncomingComplete(ctx, execCtx)
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

func (g *ParallelGateway) areAllIncomingComplete(ctx context.Context, execCtx element.ExecutionContext) bool {
	elem, ok := execCtx.Element()
	if !ok {
		return false
	}

	flows, err := execCtx.Store().GetFlowsByInstance(ctx, execCtx.Flow().InstanceID)
	if err != nil {
		return false
	}

	arrived := 0
	for _, f := range flows {
		if f.ElementID == g.id && f.Status != store.FlowStatusError {
			arrived++
		}
	}

	return arrived >= len(elem.IncomingFlows)
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
