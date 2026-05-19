package gateways

import (
	"context"

	"github.com/Raoon-Soluciones/bpmn-ai/internal/element"
	"github.com/Raoon-Soluciones/bpmn-ai/pkg/bpmn"
	"github.com/Raoon-Soluciones/bpmn-ai/pkg/store"
)

// ExclusiveGateway implements the BPMN exclusive gateway.
type ExclusiveGateway struct {
	id            string
	name          string
	gatewayType   bpmn.GatewayType
	defaultFlowID string
	conditions    map[string]string
}

// NewExclusiveGateway creates a new exclusive gateway element.
func NewExclusiveGateway(elem bpmn.Element) (element.Element, error) {
	return &ExclusiveGateway{
		id:            elem.ID,
		name:          elem.Name,
		gatewayType:   elem.GatewayType,
		defaultFlowID: elem.DefaultFlowID,
		conditions:    make(map[string]string),
	}, nil
}

// ID returns the element ID.
func (g *ExclusiveGateway) ID() string {
	return g.id
}

// Type returns the element type.
func (g *ExclusiveGateway) Type() bpmn.ElementType {
	return bpmn.ElementTypeExclusiveGateway
}

// Execute evaluates conditions and routes to the first matching flow.
func (g *ExclusiveGateway) Execute(_ context.Context, execCtx element.ExecutionContext) element.ExecutionResult {
	flow := execCtx.Flow()
	flow.Status = store.FlowStatusCompleted

	// For diverging gateways, evaluate conditions
	var matchedFlowID string

	for _, flowID := range g.getOutgoingFlowIDs(execCtx) {
		cond, hasCond := g.conditions[flowID]
		if !hasCond || cond == "" {
			// No condition, skip
			continue
		}

		// Evaluate condition (simplified - in production use expression engine)
		if g.evaluateCondition(cond, execCtx) {
			matchedFlowID = flowID
			break
		}
	}

	// If no condition matched, use default flow
	if matchedFlowID == "" && g.defaultFlowID != "" {
		matchedFlowID = g.defaultFlowID
	}

	// If still no match and no default, route to all (shouldn't happen in valid BPMN)
	if matchedFlowID == "" {
		matchedFlowID = g.getFirstOutgoingFlowID(execCtx)
	}

	return element.ExecutionResult{
		Action:      element.ActionRoute,
		FlowData:    flow,
		FlowFilters: []string{matchedFlowID},
	}
}

// GatewayType returns the gateway type.
func (g *ExclusiveGateway) GatewayType() bpmn.GatewayType {
	return g.gatewayType
}

// DefaultFlowID returns the default flow ID.
func (g *ExclusiveGateway) DefaultFlowID() string {
	return g.defaultFlowID
}

// Conditions returns the condition expressions.
func (g *ExclusiveGateway) Conditions() map[string]string {
	return g.conditions
}

// SetCondition sets a condition for a flow.
func (g *ExclusiveGateway) SetCondition(flowID, expression string) {
	g.conditions[flowID] = expression
}

func (g *ExclusiveGateway) getOutgoingFlowIDs(execCtx element.ExecutionContext) []string {
	elem, ok := execCtx.Element()
	if !ok {
		return nil
	}
	return elem.OutgoingFlows
}

func (g *ExclusiveGateway) getFirstOutgoingFlowID(execCtx element.ExecutionContext) string {
	elem, ok := execCtx.Element()
	if !ok {
		return ""
	}
	if len(elem.OutgoingFlows) > 0 {
		return elem.OutgoingFlows[0]
	}
	return ""
}

func (g *ExclusiveGateway) evaluateCondition(_ string, _ element.ExecutionContext) bool {
	// TODO: Implement expression evaluation
	// For now, return true to allow routing
	return true
}
