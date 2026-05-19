package activities

import (
	"context"

	"github.com/Raoon-Soluciones/bpmn-ai/internal/element"
	"github.com/Raoon-Soluciones/bpmn-ai/pkg/bpmn"
	"github.com/Raoon-Soluciones/bpmn-ai/pkg/store"
)

type ScriptTask struct {
	id         string
	name       string
	taskType   bpmn.TaskType
	scriptBody string
}

func NewScriptTask(elem bpmn.Element) (element.Element, error) {
	scriptBody := ""
	if elem.ExtensionData != nil {
		if v, ok := elem.ExtensionData["scriptBody"]; ok {
			scriptBody = v
		}
	}
	return &ScriptTask{
		id:         elem.ID,
		name:       elem.Name,
		taskType:   elem.TaskType,
		scriptBody: scriptBody,
	}, nil
}

func (t *ScriptTask) ID() string {
	return t.id
}

func (t *ScriptTask) Type() bpmn.ElementType {
	return bpmn.ElementTypeScriptTask
}

func (t *ScriptTask) Execute(_ context.Context, execCtx element.ExecutionContext) element.ExecutionResult {
	flow := execCtx.Flow()
	flow.Status = store.FlowStatusCompleted

	if t.scriptBody != "" {
		execCtx.SetVariable("script_result", "executed")
	}

	return element.ExecutionResult{
		Action:   element.ActionRoute,
		FlowData: flow,
	}
}

func (t *ScriptTask) TaskType() bpmn.TaskType {
	return t.taskType
}

func (t *ScriptTask) Assignee() string {
	return ""
}

func (t *ScriptTask) CandidateUsers() []string {
	return nil
}

func (t *ScriptTask) CandidateGroups() []string {
	return nil
}
