package activities

import (
	"context"
	"testing"

	"github.com/Raoon-Soluciones/bpmn-ai/internal/element"
	"github.com/Raoon-Soluciones/bpmn-ai/pkg/bpmn"
	"github.com/Raoon-Soluciones/bpmn-ai/pkg/store"
)

type mockExecCtx struct {
	flow      *store.FlowRecord
	elem      bpmn.Element
	instance  element.Instance
	stored    element.ElementStore
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

func (m *mockExecCtx) Store() element.ElementStore {
	return m.stored
}

func (m *mockExecCtx) Element() (bpmn.Element, bool) {
	return m.elem, true
}

func TestUserTask_Execute(t *testing.T) {
	tests := []struct {
		name           string
		assignee       string
		candidateUsers []string
		candidateGroups []string
	}{
		{
			name:     "with assignee",
			assignee: "user-1",
		},
		{
			name:           "with candidates",
			candidateUsers: []string{"user-a", "user-b"},
			candidateGroups: []string{"group-x"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			elem := bpmn.Element{
				ID:             "task-1",
				Name:           "Review Document",
				TaskType:       bpmn.TaskTypeUser,
				Assignee:       tt.assignee,
				CandidateUsers: tt.candidateUsers,
				CandidateGroups: tt.candidateGroups,
			}
	rawTask, err := NewUserTask(elem)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	task := rawTask.(*UserTask)

	if task.ID() != "task-1" {
		t.Errorf("expected ID task-1, got %s", task.ID())
	}
	if task.Type() != bpmn.ElementTypeUserTask {
		t.Errorf("expected type userTask, got %s", task.Type())
	}
	if task.TaskType() != bpmn.TaskTypeUser {
		t.Errorf("expected TaskTypeUser, got %s", task.TaskType())
	}
	if task.Assignee() != tt.assignee {
		t.Errorf("expected assignee %s, got %s", tt.assignee, task.Assignee())
	}

			ctx := &mockExecCtx{
				flow:      &store.FlowRecord{ElementID: "task-1"},
				variables: make(map[string]any),
			}
			result := task.Execute(context.Background(), ctx)

			if result.Action != element.ActionForm {
				t.Errorf("expected ActionForm, got %s", result.Action)
			}
		})
	}
}

func TestScriptTask_Execute(t *testing.T) {
	elem := bpmn.Element{
		ID:       "script-1",
		Name:     "Run Script",
		TaskType: bpmn.TaskTypeScript,
		ExtensionData: map[string]string{
			"scriptBody": "return true;",
		},
	}
	rawTask, err := NewScriptTask(elem)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	task := rawTask.(*ScriptTask)

	if task.ID() != "script-1" {
		t.Errorf("expected ID script-1, got %s", task.ID())
	}
	if task.Type() != bpmn.ElementTypeScriptTask {
		t.Errorf("expected type scriptTask, got %s", task.Type())
	}
	if task.TaskType() != bpmn.TaskTypeScript {
		t.Errorf("expected TaskTypeScript, got %s", task.TaskType())
	}

	ctx := &mockExecCtx{
		flow:      &store.FlowRecord{ElementID: "script-1"},
		variables: make(map[string]any),
	}
	result := task.Execute(context.Background(), ctx)

	if result.Action != element.ActionRoute {
		t.Errorf("expected ActionRoute, got %s", result.Action)
	}

	val, ok := ctx.GetVariable("script_result")
	if !ok || val != "executed" {
		t.Errorf("expected script_result='executed', got %v", val)
	}
}

func TestScriptTask_NoScript(t *testing.T) {
	elem := bpmn.Element{
		ID:       "script-1",
		Name:     "Empty Script",
		TaskType: bpmn.TaskTypeScript,
	}
	rawTask, err := NewScriptTask(elem)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	task := rawTask.(*ScriptTask)

	ctx := &mockExecCtx{
		flow:      &store.FlowRecord{ElementID: "script-1"},
		variables: make(map[string]any),
	}
	result := task.Execute(context.Background(), ctx)

	if result.Action != element.ActionRoute {
		t.Errorf("expected ActionRoute, got %s", result.Action)
	}
}

func TestServiceTask_Execute(t *testing.T) {
	elem := bpmn.Element{
		ID:       "svc-1",
		Name:     "Call API",
		TaskType: bpmn.TaskTypeService,
	}
	rawTask, err := NewServiceTask(elem)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	task := rawTask.(*ServiceTask)

	if task.ID() != "svc-1" {
		t.Errorf("expected ID svc-1, got %s", task.ID())
	}
	if task.Type() != bpmn.ElementTypeServiceTask {
		t.Errorf("expected type serviceTask, got %s", task.Type())
	}
	if task.TaskType() != bpmn.TaskTypeService {
		t.Errorf("expected TaskTypeService, got %s", task.TaskType())
	}

	result := task.Execute(context.Background(), &mockExecCtx{
		flow: &store.FlowRecord{ElementID: "svc-1"},
	})

	if result.Action != element.ActionQueue {
		t.Errorf("expected ActionQueue, got %s", result.Action)
	}
}
