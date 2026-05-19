package flows

import (
	"context"
	"testing"

	"github.com/organization/bpmn-engine/internal/element"
	"github.com/organization/bpmn-engine/pkg/bpmn"
	"github.com/organization/bpmn-engine/pkg/store"
)

type mockExecCtx struct {
	flow *store.FlowRecord
}

func (m *mockExecCtx) Instance() element.Instance {
	return nil
}

func (m *mockExecCtx) Flow() *store.FlowRecord {
	return m.flow
}

func (m *mockExecCtx) GetVariable(key string) (any, bool) {
	return nil, false
}

func (m *mockExecCtx) SetVariable(key string, value any) {}

func (m *mockExecCtx) Store() store.Store {
	return nil
}

func (m *mockExecCtx) Element() (bpmn.Element, bool) {
	return bpmn.Element{}, true
}

func TestSequenceFlow_Execute(t *testing.T) {
	elem := bpmn.Element{
		ID:   "flow-1",
		Name: "Default Flow",
	}
	f, err := NewSequenceFlow(elem)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if f.ID() != "flow-1" {
		t.Errorf("expected ID flow-1, got %s", f.ID())
	}
	if f.Type() != bpmn.ElementTypeSequenceFlow {
		t.Errorf("expected type sequenceFlow, got %s", f.Type())
	}

	result := f.Execute(context.Background(), &mockExecCtx{
		flow: &store.FlowRecord{ElementID: "flow-1"},
	})

	if result.Action != element.ActionRoute {
		t.Errorf("expected ActionRoute, got %s", result.Action)
	}
}

func TestSequenceFlow_Interfaces(t *testing.T) {
	elem := bpmn.Element{ID: "flow-1"}
	raw, _ := NewSequenceFlow(elem)
	f := raw.(*SequenceFlow)

	if f.SourceRef() != "" {
		t.Errorf("expected empty source ref, got %s", f.SourceRef())
	}
	if f.TargetRef() != "" {
		t.Errorf("expected empty target ref, got %s", f.TargetRef())
	}
	if f.Condition() != "" {
		t.Errorf("expected empty condition, got %s", f.Condition())
	}
	if f.IsDefault() {
		t.Error("expected IsDefault to be false")
	}
}
