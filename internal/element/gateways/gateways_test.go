package gateways

import (
	"context"
	"testing"

	"github.com/organization/bpmn-engine/internal/element"
	"github.com/organization/bpmn-engine/pkg/bpmn"
	"github.com/organization/bpmn-engine/pkg/store"
)

type mockExecCtx struct {
	flow      *store.FlowRecord
	elem      bpmn.Element
	instance  element.Instance
	stored    store.Store
	variables map[string]any
}

func (m *mockExecCtx) Instance() element.Instance {
	return m.instance
}

func (m *mockExecCtx) Flow() *store.FlowRecord {
	return m.flow
}

func (m *mockExecCtx) GetVariable(key string) (any, bool) {
	v, ok := m.variables[key]
	return v, ok
}

func (m *mockExecCtx) SetVariable(key string, value any) {
	if m.variables == nil {
		m.variables = make(map[string]any)
	}
	m.variables[key] = value
}

func (m *mockExecCtx) Store() store.Store {
	return m.stored
}

func (m *mockExecCtx) Element() (bpmn.Element, bool) {
	return m.elem, true
}

func TestExclusiveGateway_Execute(t *testing.T) {
	elem := bpmn.Element{
		ID:            "gw-1",
		Name:          "Exclusive",
		GatewayType:   bpmn.GatewayTypeExclusive,
		DefaultFlowID: "flow-default",
		OutgoingFlows: []string{"flow-a", "flow-b", "flow-default"},
	}
	rawG, err := NewExclusiveGateway(elem)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	g := rawG.(*ExclusiveGateway)

	if g.ID() != "gw-1" {
		t.Errorf("expected ID gw-1, got %s", g.ID())
	}
	if g.Type() != bpmn.ElementTypeExclusiveGateway {
		t.Errorf("expected type exclusiveGateway, got %s", g.Type())
	}
	if g.DefaultFlowID() != "flow-default" {
		t.Errorf("expected default flow flow-default, got %s", g.DefaultFlowID())
	}

	result := g.Execute(context.Background(), &mockExecCtx{
		flow: &store.FlowRecord{ElementID: "gw-1"},
		elem: bpmn.Element{OutgoingFlows: []string{"flow-a", "flow-b", "flow-default"}},
	})

	if result.Action != element.ActionRoute {
		t.Errorf("expected ActionRoute, got %s", result.Action)
	}
}

func TestParallelGateway_Execute(t *testing.T) {
	elem := bpmn.Element{
		ID:          "gw-p-1",
		Name:        "Parallel",
		GatewayType: bpmn.GatewayTypeParallel,
	}
	rawG, err := NewParallelGateway(elem)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	g := rawG.(*ParallelGateway)

	if g.ID() != "gw-p-1" {
		t.Errorf("expected ID gw-p-1, got %s", g.ID())
	}
	if g.Type() != bpmn.ElementTypeParallelGateway {
		t.Errorf("expected type parallelGateway, got %s", g.Type())
	}
	if g.GatewayType() != bpmn.GatewayTypeParallel {
		t.Errorf("expected GatewayTypeParallel, got %s", g.GatewayType())
	}
}

func TestParallelGateway_Diverging(t *testing.T) {
	g, _ := NewParallelGateway(bpmn.Element{
		ID: "gw-p-1", GatewayType: bpmn.GatewayTypeParallel,
	})

	result := g.Execute(context.Background(), &mockExecCtx{
		flow: &store.FlowRecord{ElementID: "gw-p-1"},
		elem: bpmn.Element{
			OutgoingFlows: []string{"flow-a", "flow-b"},
			IncomingFlows: []string{"flow-in"},
		},
	})

	if result.Action != element.ActionRoute {
		t.Errorf("diverging parallel: expected ActionRoute, got %s", result.Action)
	}
	if len(result.FlowFilters) != 0 {
		t.Errorf("diverging parallel: expected no filters, got %v", result.FlowFilters)
	}
}

func TestInclusiveGateway_Execute(t *testing.T) {
	tests := []struct {
		name        string
		outgoing    []string
		conditions  map[string]string
		defaultFlow string
		wantMin     int
	}{
		{
			name:     "multiple outgoing with no conditions",
			outgoing: []string{"flow-a", "flow-b"},
			wantMin:  2,
		},
		{
			name:        "uses default when no match",
			outgoing:    []string{"flow-a", "flow-b"},
			defaultFlow: "flow-b",
			wantMin:     1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			elem := bpmn.Element{
				ID:            "gw-inc-1",
				GatewayType:   bpmn.GatewayTypeInclusive,
				DefaultFlowID: tt.defaultFlow,
				Conditions:    tt.conditions,
				OutgoingFlows: tt.outgoing,
			}
			g, err := NewInclusiveGateway(elem)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if g.Type() != bpmn.ElementTypeInclusiveGateway {
				t.Errorf("expected type inclusiveGateway, got %s", g.Type())
			}

			result := g.Execute(context.Background(), &mockExecCtx{
				flow: &store.FlowRecord{ElementID: "gw-inc-1"},
				elem: bpmn.Element{
					OutgoingFlows: tt.outgoing,
				},
				variables: make(map[string]any),
			})

			if result.Action != element.ActionRoute {
				t.Errorf("expected ActionRoute, got %s", result.Action)
			}
			if len(result.FlowFilters) < tt.wantMin {
				t.Errorf("expected at least %d flow filters, got %d", tt.wantMin, len(result.FlowFilters))
			}
		})
	}
}

func TestEventBasedGateway_Execute(t *testing.T) {
	tests := []struct {
		name     string
		outgoing []string
		wantAct  element.Action
	}{
		{
			name:     "single outgoing routes directly",
			outgoing: []string{"flow-a"},
			wantAct:  element.ActionRoute,
		},
		{
			name:     "multiple outgoing waits for event",
			outgoing: []string{"flow-a", "flow-b"},
			wantAct:  element.ActionWait,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			elem := bpmn.Element{
				ID:          "gw-eb-1",
				GatewayType: bpmn.GatewayTypeEventBased,
			}
			g, err := NewEventBasedGateway(elem)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if g.Type() != bpmn.ElementTypeEventBasedGateway {
				t.Errorf("expected type eventBasedGateway, got %s", g.Type())
			}

			result := g.Execute(context.Background(), &mockExecCtx{
				flow: &store.FlowRecord{ElementID: "gw-eb-1"},
				elem: bpmn.Element{OutgoingFlows: tt.outgoing},
			})

			if result.Action != tt.wantAct {
				t.Errorf("expected %s, got %s", tt.wantAct, result.Action)
			}
		})
	}
}

func TestExclusiveGateway_NoElement(t *testing.T) {
	g, _ := NewExclusiveGateway(bpmn.Element{ID: "gw-1"})

	result := g.Execute(context.Background(), &mockExecCtx{
		flow: &store.FlowRecord{ElementID: "gw-1"},
		elem: bpmn.Element{OutgoingFlows: []string{"flow-a"}},
	})

	if result.Action != element.ActionRoute {
		t.Errorf("expected ActionRoute, got %s", result.Action)
	}
}

func TestInclusiveGateway_NoElement(t *testing.T) {
	g, _ := NewInclusiveGateway(bpmn.Element{ID: "gw-inc-1"})

	result := g.Execute(context.Background(), &mockExecCtx{
		flow: &store.FlowRecord{ElementID: "gw-inc-1"},
		elem: bpmn.Element{OutgoingFlows: []string{"flow-a"}},
	})

	if result.Action != element.ActionRoute {
		t.Errorf("expected ActionRoute, got %s", result.Action)
	}
}

func TestEventBasedGateway_NoElement(t *testing.T) {
	g, _ := NewEventBasedGateway(bpmn.Element{ID: "gw-eb-1"})

	result := g.Execute(context.Background(), &mockExecCtx{
		flow: &store.FlowRecord{ElementID: "gw-eb-1"},
		elem: bpmn.Element{OutgoingFlows: []string{"flow-a"}},
	})

	if result.Action != element.ActionRoute {
		t.Errorf("expected ActionRoute, got %s", result.Action)
	}
}

func TestParallelGateway_Converging(t *testing.T) {
	g, _ := NewParallelGateway(bpmn.Element{
		ID: "gw-p-1", GatewayType: bpmn.GatewayTypeParallel,
	})

	result := g.Execute(context.Background(), &mockExecCtx{
		flow: &store.FlowRecord{ElementID: "gw-p-1"},
		elem: bpmn.Element{
			IncomingFlows: []string{"flow-a", "flow-b"},
			OutgoingFlows: []string{"flow-out"},
		},
	})

	if result.Action != element.ActionRoute {
		t.Errorf("expected ActionRoute, got %s", result.Action)
	}
}

func TestExclusiveGateway_Conditions(t *testing.T) {
	g, _ := NewExclusiveGateway(bpmn.Element{
		ID:            "gw-1",
		DefaultFlowID: "flow-default",
	})
	eg := g.(*ExclusiveGateway)
	eg.SetCondition("flow-a", "amount > 1000")

	if eg.DefaultFlowID() != "flow-default" {
		t.Errorf("expected default flow-default, got %s", eg.DefaultFlowID())
	}
	if len(eg.Conditions()) != 1 {
		t.Errorf("expected 1 condition, got %d", len(eg.Conditions()))
	}
}

func TestParallelGateway_Conditions(t *testing.T) {
	g, _ := NewParallelGateway(bpmn.Element{ID: "gw-p-1"})
	pg := g.(*ParallelGateway)

	if pg.DefaultFlowID() != "" {
		t.Errorf("expected empty default flow, got %s", pg.DefaultFlowID())
	}
	if pg.Conditions() != nil {
		t.Error("expected nil conditions")
	}
}
