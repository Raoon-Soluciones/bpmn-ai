package gateways

import (
	"context"

	"github.com/organization/bpmn-engine/internal/element"
	"github.com/organization/bpmn-engine/pkg/bpmn"
	"github.com/organization/bpmn-engine/pkg/store"
)

type EventBasedGateway struct {
	id          string
	name        string
	gatewayType bpmn.GatewayType
}

func NewEventBasedGateway(elem bpmn.Element) (element.Element, error) {
	return &EventBasedGateway{
		id:          elem.ID,
		name:        elem.Name,
		gatewayType: elem.GatewayType,
	}, nil
}

func (g *EventBasedGateway) ID() string {
	return g.id
}

func (g *EventBasedGateway) Type() bpmn.ElementType {
	return bpmn.ElementTypeEventBasedGateway
}

func (g *EventBasedGateway) Execute(_ context.Context, execCtx element.ExecutionContext) element.ExecutionResult {
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

	outgoing := elem.OutgoingFlows

	if len(outgoing) == 1 {
		return element.ExecutionResult{
			Action:   element.ActionRoute,
			FlowData: flow,
		}
	}

	return element.ExecutionResult{
		Action:   element.ActionWait,
		FlowData: flow,
	}
}

func (g *EventBasedGateway) GatewayType() bpmn.GatewayType {
	return g.gatewayType
}

func (g *EventBasedGateway) DefaultFlowID() string {
	return ""
}

func (g *EventBasedGateway) Conditions() map[string]string {
	return nil
}
