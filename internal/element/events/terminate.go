package events

import (
	"context"

	"github.com/organization/bpmn-engine/internal/element"
	"github.com/organization/bpmn-engine/pkg/bpmn"
)

type TerminateEvent struct {
	id   string
	name string
}

func NewTerminateEvent(elem bpmn.Element) (element.Element, error) {
	return &TerminateEvent{
		id:   elem.ID,
		name: elem.Name,
	}, nil
}

func (e *TerminateEvent) ID() string {
	return e.id
}

func (e *TerminateEvent) Type() bpmn.ElementType {
	return bpmn.ElementTypeTerminateEvent
}

func (e *TerminateEvent) Execute(_ context.Context, execCtx element.ExecutionContext) element.ExecutionResult {
	flow := execCtx.Flow()
	flow.Status = "COMPLETED"

	return element.ExecutionResult{
		Action:   element.ActionTerminate,
		FlowData: flow,
	}
}

func (e *TerminateEvent) EventDefinition() bpmn.EventDefinition {
	return bpmn.EventDefinition{Type: bpmn.EventTypeTerminate}
}
