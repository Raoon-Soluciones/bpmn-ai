package memory

import (
	"context"
	"testing"

	"github.com/Raoon-Soluciones/bpmn-ai/pkg/bpmn"
	"github.com/Raoon-Soluciones/bpmn-ai/pkg/store"
)

func TestStore_SaveAndGetProcess(t *testing.T) {
	s := NewStore()
	ctx := context.Background()

	proc := &bpmn.Process{
		ID:           "proc-1",
		Name:         "Test Process",
		Elements:     make(map[string]bpmn.Element),
		Flows:        make(map[string]bpmn.Flow),
	}

	if err := s.SaveProcess(ctx, proc); err != nil {
		t.Fatalf("save process: %v", err)
	}

	got, err := s.GetProcess(ctx, "proc-1")
	if err != nil {
		t.Fatalf("get process: %v", err)
	}
	if got.ID != "proc-1" {
		t.Errorf("expected ID proc-1, got %s", got.ID)
	}
	if got.Name != "Test Process" {
		t.Errorf("expected name Test Process, got %s", got.Name)
	}
}

func TestStore_GetProcess_NotFound(t *testing.T) {
	s := NewStore()
	_, err := s.GetProcess(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent process")
	}
}

func TestStore_CreateGetInstance(t *testing.T) {
	s := NewStore()
	ctx := context.Background()

	inst := &store.InstanceRecord{
		ID:        "case-1",
		ProcessID: "proc-1",
		Status:    store.InstanceStatusCreated,
		Variables: map[string]any{"key": "value"},
	}

	if err := s.CreateInstance(ctx, inst); err != nil {
		t.Fatalf("create instance: %v", err)
	}

	got, err := s.GetInstance(ctx, "case-1")
	if err != nil {
		t.Fatalf("get instance: %v", err)
	}
	if got.ID != "case-1" {
		t.Errorf("expected ID case-1, got %s", got.ID)
	}
	if got.Variables["key"] != "value" {
		t.Errorf("expected variable value, got %v", got.Variables["key"])
	}
}

func TestStore_UpdateInstance(t *testing.T) {
	s := NewStore()
	ctx := context.Background()

	inst := &store.InstanceRecord{
		ID:     "case-1",
		Status: store.InstanceStatusCreated,
	}
	s.CreateInstance(ctx, inst)

	inst.Status = store.InstanceStatusInProgress
	s.UpdateInstance(ctx, inst)

	got, _ := s.GetInstance(ctx, "case-1")
	if got.Status != store.InstanceStatusInProgress {
		t.Errorf("expected status IN_PROGRESS, got %s", got.Status)
	}
}

func TestStore_CreateGetFlow(t *testing.T) {
	s := NewStore()
	ctx := context.Background()

	flow := &store.FlowRecord{
		ID:          "flow-1",
		InstanceID:  "case-1",
		ElementID:   "start-1",
		ElementType: bpmn.ElementTypeStartEvent,
		Status:      store.FlowStatusActive,
	}

	if err := s.CreateFlow(ctx, flow); err != nil {
		t.Fatalf("create flow: %v", err)
	}

	got, err := s.GetFlow(ctx, "flow-1")
	if err != nil {
		t.Fatalf("get flow: %v", err)
	}
	if got.ElementID != "start-1" {
		t.Errorf("expected elementID start-1, got %s", got.ElementID)
	}
}

func TestStore_GetFlowsByInstance(t *testing.T) {
	s := NewStore()
	ctx := context.Background()

	s.CreateFlow(ctx, &store.FlowRecord{ID: "f1", InstanceID: "case-1", ElementID: "e1"})
	s.CreateFlow(ctx, &store.FlowRecord{ID: "f2", InstanceID: "case-1", ElementID: "e2"})
	s.CreateFlow(ctx, &store.FlowRecord{ID: "f3", InstanceID: "case-2", ElementID: "e3"})

	flows, err := s.GetFlowsByInstance(ctx, "case-1")
	if err != nil {
		t.Fatalf("get flows: %v", err)
	}
	if len(flows) != 2 {
		t.Errorf("expected 2 flows, got %d", len(flows))
	}
}

func TestStore_Jobs(t *testing.T) {
	s := NewStore()
	ctx := context.Background()

	job := &store.JobRecord{
		ID:         "job-1",
		InstanceID: "case-1",
		Type:       "timer",
		Status:     store.JobStatusPending,
		MaxRetries: 3,
	}

	if err := s.CreateJob(ctx, job); err != nil {
		t.Fatalf("create job: %v", err)
	}

	pending, err := s.GetPendingJobs(ctx, 10)
	if err != nil {
		t.Fatalf("get pending jobs: %v", err)
	}
	if len(pending) != 1 {
		t.Errorf("expected 1 pending job, got %d", len(pending))
	}

	got, err := s.GetJob(ctx, "job-1")
	if err != nil {
		t.Fatalf("get job: %v", err)
	}
	if got.Type != "timer" {
		t.Errorf("expected type timer, got %s", got.Type)
	}
}

func TestStore_ExecutionLog(t *testing.T) {
	s := NewStore()
	ctx := context.Background()

	entry := &store.ExecutionLogEntry{
		InstanceID:  "case-1",
		ElementID:   "start-1",
		ElementType: bpmn.ElementTypeStartEvent,
		Action:      "ROUTE",
		DurationMs:  5,
	}

	if err := s.LogExecution(ctx, entry); err != nil {
		t.Fatalf("log execution: %v", err)
	}

	logs, err := s.GetExecutionLog(ctx, "case-1")
	if err != nil {
		t.Fatalf("get execution log: %v", err)
	}
	if len(logs) != 1 {
		t.Errorf("expected 1 log entry, got %d", len(logs))
	}
	if logs[0].Action != "ROUTE" {
		t.Errorf("expected action ROUTE, got %s", logs[0].Action)
	}
}

func TestStore_Reset(t *testing.T) {
	s := NewStore()
	ctx := context.Background()

	s.SaveProcess(ctx, &bpmn.Process{ID: "p1", Elements: make(map[string]bpmn.Element), Flows: make(map[string]bpmn.Flow)})
	s.CreateInstance(ctx, &store.InstanceRecord{ID: "c1"})
	s.CreateFlow(ctx, &store.FlowRecord{ID: "f1"})
	s.CreateJob(ctx, &store.JobRecord{ID: "j1"})

	s.Reset()

	procs, _ := s.ListProcesses(ctx)
	if len(procs) != 0 {
		t.Errorf("expected 0 processes after reset, got %d", len(procs))
	}

	insts, _ := s.ListInstances(ctx, "")
	if len(insts) != 0 {
		t.Errorf("expected 0 instances after reset, got %d", len(insts))
	}
}
