package events

import (
	"context"
	"fmt"

	"github.com/organization/bpmn-engine/internal/element"
	"github.com/organization/bpmn-engine/pkg/bpmn"
)

type TimerEvent struct {
	id          string
	name        string
	eventDef    bpmn.EventDefinition
}

func NewTimerEvent(elem bpmn.Element) (element.Element, error) {
	return &TimerEvent{
		id:       elem.ID,
		name:     elem.Name,
		eventDef: elem.EventDefinition,
	}, nil
}

func (e *TimerEvent) ID() string {
	return e.id
}

func (e *TimerEvent) Type() bpmn.ElementType {
	return bpmn.ElementTypeTimerEvent
}

func (e *TimerEvent) Execute(_ context.Context, execCtx element.ExecutionContext) element.ExecutionResult {
	flow := execCtx.Flow()

	if e.eventDef.TimerValue == "" {
		return element.ExecutionResult{
			Action:   element.ActionRoute,
			FlowData: flow,
		}
	}

	execCtx.SetVariable("timer_type", string(e.eventDef.TimerType))
	execCtx.SetVariable("timer_value", e.eventDef.TimerValue)

	return element.ExecutionResult{
		Action:   element.ActionWait,
		FlowData: flow,
	}
}

func (e *TimerEvent) EventDefinition() bpmn.EventDefinition {
	return e.eventDef
}

func timerEventKey(elemID string) string {
	return fmt.Sprintf("timer:%s", elemID)
}
