package events

import (
	"context"

	"github.com/organization/bpmn-engine/internal/element"
	"github.com/organization/bpmn-engine/pkg/bpmn"
)

type MessageCatchEvent struct {
	id       string
	name     string
	eventDef bpmn.EventDefinition
}

func NewMessageCatchEvent(elem bpmn.Element) (element.Element, error) {
	return &MessageCatchEvent{
		id:       elem.ID,
		name:     elem.Name,
		eventDef: elem.EventDefinition,
	}, nil
}

func (e *MessageCatchEvent) ID() string {
	return e.id
}

func (e *MessageCatchEvent) Type() bpmn.ElementType {
	return bpmn.ElementTypeMessageCatch
}

func (e *MessageCatchEvent) Execute(_ context.Context, execCtx element.ExecutionContext) element.ExecutionResult {
	flow := execCtx.Flow()
	flow.Status = "COMPLETED"

	if e.eventDef.MessageRef != "" {
		execCtx.SetVariable("expected_message", e.eventDef.MessageRef)
	}

	return element.ExecutionResult{
		Action:   element.ActionWait,
		FlowData: flow,
	}
}

func (e *MessageCatchEvent) EventDefinition() bpmn.EventDefinition {
	return e.eventDef
}
