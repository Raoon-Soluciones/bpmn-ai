package events

import (
	"context"
	"testing"

	"github.com/Raoon-Soluciones/bpmn-ai/internal/element"
	"github.com/Raoon-Soluciones/bpmn-ai/pkg/bpmn"
	"github.com/Raoon-Soluciones/bpmn-ai/pkg/store"
)

type mockExecutionContext struct {
	flow       *store.FlowRecord
	instance   element.Instance
	stored     element.ElementStore
	variables  map[string]any
}

func (m *mockExecutionContext) Instance() element.Instance {
	return m.instance
}

func (m *mockExecutionContext) Flow() *store.FlowRecord {
	return m.flow
}

func (m *mockExecutionContext) GetVariable(key string) (any, bool) {
	v, ok := m.variables[key]
	return v, ok
}

func (m *mockExecutionContext) SetVariable(key string, value any) {
	if m.variables == nil {
		m.variables = make(map[string]any)
	}
	m.variables[key] = value
}

func (m *mockExecutionContext) Store() element.ElementStore {
	return m.stored
}

func (m *mockExecutionContext) Element() (bpmn.Element, bool) {
	return bpmn.Element{}, false
}

func TestStartEvent_Execute(t *testing.T) {
	elem := bpmn.Element{ID: "start-1", Name: "Start"}
	s, err := NewStartEvent(elem)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if s.ID() != "start-1" {
		t.Errorf("expected ID start-1, got %s", s.ID())
	}
	if s.Type() != bpmn.ElementTypeStartEvent {
		t.Errorf("expected type startEvent, got %s", s.Type())
	}

	result := s.Execute(context.Background(), &mockExecutionContext{
		flow: &store.FlowRecord{ElementID: "start-1"},
	})

	if result.Action != element.ActionRoute {
		t.Errorf("expected ActionRoute, got %s", result.Action)
	}
}

func TestEndEvent_Execute(t *testing.T) {
	elem := bpmn.Element{ID: "end-1", Name: "End"}
	e, err := NewEndEvent(elem)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if e.ID() != "end-1" {
		t.Errorf("expected ID end-1, got %s", e.ID())
	}
	if e.Type() != bpmn.ElementTypeEndEvent {
		t.Errorf("expected type endEvent, got %s", e.Type())
	}

	result := e.Execute(context.Background(), &mockExecutionContext{
		flow: &store.FlowRecord{ElementID: "end-1"},
	})

	if result.Action != element.ActionComplete {
		t.Errorf("expected ActionComplete, got %s", result.Action)
	}
}

func TestTerminateEvent_Execute(t *testing.T) {
	elem := bpmn.Element{
		ID: "term-1", Name: "Terminate",
		EventDefinition: bpmn.EventDefinition{Type: bpmn.EventTypeTerminate},
	}
	e, err := NewTerminateEvent(elem)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if e.ID() != "term-1" {
		t.Errorf("expected ID term-1, got %s", e.ID())
	}
	if e.Type() != bpmn.ElementTypeTerminateEvent {
		t.Errorf("expected type terminateEvent, got %s", e.Type())
	}

	result := e.Execute(context.Background(), &mockExecutionContext{
		flow: &store.FlowRecord{ElementID: "term-1"},
	})

	if result.Action != element.ActionTerminate {
		t.Errorf("expected ActionTerminate, got %s", result.Action)
	}
}

func TestTimerEvent_Execute(t *testing.T) {
	tests := []struct {
		name       string
		eventDef   bpmn.EventDefinition
		wantAction element.Action
	}{
		{
			name:       "with timer value",
			eventDef:   bpmn.EventDefinition{Type: bpmn.EventTypeTimer, TimerType: bpmn.TimerTypeDuration, TimerValue: "PT1H"},
			wantAction: element.ActionWait,
		},
		{
			name:       "without timer value",
			eventDef:   bpmn.EventDefinition{Type: bpmn.EventTypeNone},
			wantAction: element.ActionRoute,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			elem := bpmn.Element{ID: "timer-1", EventDefinition: tt.eventDef}
			e, err := NewTimerEvent(elem)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if e.Type() != bpmn.ElementTypeTimerEvent {
				t.Errorf("expected type timerEvent, got %s", e.Type())
			}

			ctx := &mockExecutionContext{
				flow:      &store.FlowRecord{ElementID: "timer-1"},
				variables: make(map[string]any),
			}
			result := e.Execute(context.Background(), ctx)
			if result.Action != tt.wantAction {
				t.Errorf("expected %s, got %s", tt.wantAction, result.Action)
			}
		})
	}
}

func TestMessageThrowEvent_Execute(t *testing.T) {
	elem := bpmn.Element{
		ID: "msg-throw-1", Name: "Send Message",
		EventDefinition: bpmn.EventDefinition{Type: bpmn.EventTypeMessage, MessageRef: "msg-1"},
	}
	e, err := NewMessageThrowEvent(elem)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if e.Type() != bpmn.ElementTypeMessageThrow {
		t.Errorf("expected type messageThrow, got %s", e.Type())
	}

	ctx := &mockExecutionContext{
		flow:      &store.FlowRecord{ElementID: "msg-throw-1"},
		variables: make(map[string]any),
	}
	result := e.Execute(context.Background(), ctx)
	if result.Action != element.ActionRoute {
		t.Errorf("expected ActionRoute, got %s", result.Action)
	}

	msgRef, ok := ctx.GetVariable("message_ref")
	if !ok || msgRef != "msg-1" {
		t.Errorf("expected message_ref to be msg-1, got %v", msgRef)
	}
}

func TestMessageCatchEvent_Execute(t *testing.T) {
	elem := bpmn.Element{
		ID: "msg-catch-1", Name: "Receive Message",
		EventDefinition: bpmn.EventDefinition{Type: bpmn.EventTypeMessage, MessageRef: "msg-1"},
	}
	e, err := NewMessageCatchEvent(elem)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if e.Type() != bpmn.ElementTypeMessageCatch {
		t.Errorf("expected type messageCatch, got %s", e.Type())
	}

	ctx := &mockExecutionContext{
		flow:      &store.FlowRecord{ElementID: "msg-catch-1"},
		variables: make(map[string]any),
	}
	result := e.Execute(context.Background(), ctx)
	if result.Action != element.ActionWait {
		t.Errorf("expected ActionWait, got %s", result.Action)
	}

	msgRef, ok := ctx.GetVariable("expected_message")
	if !ok || msgRef != "msg-1" {
		t.Errorf("expected expected_message to be msg-1, got %v", msgRef)
	}
}
