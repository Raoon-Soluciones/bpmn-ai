package engine

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/organization/bpmn-engine/internal/element/activities"
	"github.com/organization/bpmn-engine/internal/element/events"
	"github.com/organization/bpmn-engine/internal/element/gateways"
	"github.com/organization/bpmn-engine/internal/observability"
	"github.com/organization/bpmn-engine/internal/process"
	"github.com/organization/bpmn-engine/pkg/bpmn"
	"github.com/organization/bpmn-engine/pkg/store"
	"github.com/organization/bpmn-engine/pkg/store/memory"
)

func TestEngine_Run_SimpleSequence(t *testing.T) {
	// Build a simple process: start -> end
	proc := &bpmn.Process{
		ID:           "proc-1",
		Name:         "Simple Sequence",
		StartEventID: "start-1",
		Elements: map[string]bpmn.Element{
			"start-1": {ID: "start-1", Type: bpmn.ElementTypeStartEvent, OutgoingFlows: []string{"flow-1"}},
			"end-1":   {ID: "end-1", Type: bpmn.ElementTypeEndEvent, IncomingFlows: []string{"flow-1"}},
		},
		Flows: map[string]bpmn.Flow{
			"flow-1": {ID: "flow-1", SourceRef: "start-1", TargetRef: "end-1"},
		},
	}

	// Create registry
	registry := NewElementRegistry()
	registry.Register(bpmn.ElementTypeStartEvent, events.NewStartEvent)
	registry.Register(bpmn.ElementTypeEndEvent, events.NewEndEvent)

	// Create store and engine
	store := memory.NewStore()
	logger, _ := observability.NewFromConfig("debug", "text")
	if logger == nil {
		t.Fatal("failed to create logger")
	}

	cfg := Config{
		WorkerCount:      1,
		MaxLoops:         100,
		ExecutionTimeout: 10 * time.Second,
	}

	eng := New(cfg, registry, store, logger, nil)

	// Create instance and run
	instance := process.NewInstance(proc, nil)
	if err := store.CreateInstance(context.Background(), instance.ToRecord()); err != nil {
		t.Fatalf("create instance: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := eng.Run(ctx, instance)
	if err != nil {
		t.Fatalf("engine run failed: %v", err)
	}

	if instance.State != process.StateCompleted {
		t.Errorf("expected state COMPLETED, got %s", instance.State)
	}

	// Verify flows were created
	flowRecs, err := store.GetFlowsByInstance(context.Background(), instance.ID)
	if err != nil {
		t.Fatalf("get flows: %v", err)
	}
	if len(flowRecs) < 2 {
		t.Errorf("expected at least 2 flow records, got %d", len(flowRecs))
	}
}

func TestEngine_Run_ParallelGateway(t *testing.T) {
	// Build process: start -> parallel(div) -> end-a, end-b -> parallel(conv) -> end
	proc := &bpmn.Process{
		ID:           "proc-parallel",
		Name:         "Parallel Process",
		StartEventID: "start-1",
		Elements: map[string]bpmn.Element{
			"start-1": {ID: "start-1", Type: bpmn.ElementTypeStartEvent, OutgoingFlows: []string{"flow-1"}},
			"gw-div":  {ID: "gw-div", Type: bpmn.ElementTypeParallelGateway, IncomingFlows: []string{"flow-1"}, OutgoingFlows: []string{"flow-2a", "flow-2b"}},
			"end-a":   {ID: "end-a", Type: bpmn.ElementTypeEndEvent, IncomingFlows: []string{"flow-2a"}},
			"end-b":   {ID: "end-b", Type: bpmn.ElementTypeEndEvent, IncomingFlows: []string{"flow-2b"}},
		},
		Flows: map[string]bpmn.Flow{
			"flow-1":  {ID: "flow-1", SourceRef: "start-1", TargetRef: "gw-div"},
			"flow-2a": {ID: "flow-2a", SourceRef: "gw-div", TargetRef: "end-a"},
			"flow-2b": {ID: "flow-2b", SourceRef: "gw-div", TargetRef: "end-b"},
		},
	}

	registry := NewElementRegistry()
	registry.Register(bpmn.ElementTypeStartEvent, events.NewStartEvent)
	registry.Register(bpmn.ElementTypeEndEvent, events.NewEndEvent)
	registry.Register(bpmn.ElementTypeParallelGateway, gateways.NewParallelGateway)

	store := memory.NewStore()
	logger, _ := observability.NewFromConfig("info", "text")

	cfg := Config{
		WorkerCount:      2,
		MaxLoops:         100,
		ExecutionTimeout: 10 * time.Second,
	}

	eng := New(cfg, registry, store, logger, nil)

	instance := process.NewInstance(proc, nil)
	if err := store.CreateInstance(context.Background(), instance.ToRecord()); err != nil {
		t.Fatalf("create instance: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := eng.Run(ctx, instance)
	if err != nil {
		t.Fatalf("engine run failed: %v", err)
	}

	// Verify parallel branches created threads
	threads, err := store.GetThreadsByInstance(context.Background(), instance.ID)
	if err != nil {
		t.Fatalf("get threads: %v", err)
	}
	if len(threads) < 2 {
		t.Errorf("expected at least 2 threads for parallel branches, got %d", len(threads))
	}
}

func TestEngine_Run_ExclusiveGateway(t *testing.T) {
	// Build process: start -> exclusive -> end-approved, end-rejected
	proc := &bpmn.Process{
		ID:           "proc-exclusive",
		Name:         "Exclusive Process",
		StartEventID: "start-1",
		Elements: map[string]bpmn.Element{
			"start-1": {ID: "start-1", Type: bpmn.ElementTypeStartEvent, OutgoingFlows: []string{"flow-1"}},
			"gw-1":    {ID: "gw-1", Type: bpmn.ElementTypeExclusiveGateway, IncomingFlows: []string{"flow-1"}, OutgoingFlows: []string{"flow-2", "flow-3"}, DefaultFlowID: "flow-3"},
			"end-app": {ID: "end-app", Type: bpmn.ElementTypeEndEvent, IncomingFlows: []string{"flow-2"}},
			"end-rej": {ID: "end-rej", Type: bpmn.ElementTypeEndEvent, IncomingFlows: []string{"flow-3"}},
		},
		Flows: map[string]bpmn.Flow{
			"flow-1": {ID: "flow-1", SourceRef: "start-1", TargetRef: "gw-1"},
			"flow-2": {ID: "flow-2", SourceRef: "gw-1", TargetRef: "end-app"},
			"flow-3": {ID: "flow-3", SourceRef: "gw-1", TargetRef: "end-rej"},
		},
	}

	registry := NewElementRegistry()
	registry.Register(bpmn.ElementTypeStartEvent, events.NewStartEvent)
	registry.Register(bpmn.ElementTypeEndEvent, events.NewEndEvent)
	registry.Register(bpmn.ElementTypeExclusiveGateway, gateways.NewExclusiveGateway)

	store := memory.NewStore()
	logger, _ := observability.NewFromConfig("info", "text")

	cfg := Config{
		WorkerCount:      1,
		MaxLoops:         100,
		ExecutionTimeout: 10 * time.Second,
	}

	eng := New(cfg, registry, store, logger, nil)

	instance := process.NewInstance(proc, nil)
	if err := store.CreateInstance(context.Background(), instance.ToRecord()); err != nil {
		t.Fatalf("create instance: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := eng.Run(ctx, instance)
	if err != nil {
		t.Fatalf("engine run failed: %v", err)
	}

	if instance.State != process.StateCompleted {
		t.Errorf("expected state COMPLETED, got %s", instance.State)
	}
}

func TestEngine_Run_NoStartEvent(t *testing.T) {
	proc := &bpmn.Process{
		ID:           "proc-nostart",
		Name:         "No Start",
		StartEventID: "",
		Elements:     map[string]bpmn.Element{},
		Flows:        map[string]bpmn.Flow{},
	}

	registry := NewElementRegistry()
	store := memory.NewStore()
	logger, _ := observability.NewFromConfig("info", "text")
	eng := New(Config{WorkerCount: 1}, registry, store, logger, nil)

	instance := process.NewInstance(proc, nil)
	err := eng.Run(context.Background(), instance)
	if err == nil {
		t.Fatal("expected error for process with no start event")
	}
}

func TestEngine_Run_ContextCancellation(t *testing.T) {
	proc := &bpmn.Process{
		ID:           "proc-cancel",
		Name:         "Cancel Test",
		StartEventID: "start-1",
		Elements: map[string]bpmn.Element{
			"start-1": {ID: "start-1", Type: bpmn.ElementTypeStartEvent, OutgoingFlows: []string{"flow-1"}},
			"end-1":   {ID: "end-1", Type: bpmn.ElementTypeEndEvent, IncomingFlows: []string{"flow-1"}},
		},
		Flows: map[string]bpmn.Flow{
			"flow-1": {ID: "flow-1", SourceRef: "start-1", TargetRef: "end-1"},
		},
	}

	registry := NewElementRegistry()
	registry.Register(bpmn.ElementTypeStartEvent, events.NewStartEvent)
	registry.Register(bpmn.ElementTypeEndEvent, events.NewEndEvent)

	store := memory.NewStore()
	logger, _ := observability.NewFromConfig("info", "text")
	eng := New(Config{WorkerCount: 1}, registry, store, logger, nil)

	instance := process.NewInstance(proc, nil)
	if err := store.CreateInstance(context.Background(), instance.ToRecord()); err != nil {
		t.Fatalf("create instance: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	err := eng.Run(ctx, instance)
	if err != context.Canceled {
		t.Errorf("expected context.Canceled error, got %v", err)
	}
}

func TestFailSafeManager_Timeout(t *testing.T) {
	fs := NewFailSafeManager(1*time.Millisecond, 100)
	time.Sleep(5 * time.Millisecond)

	err := fs.Check("elem-1")
	if err == nil {
		t.Fatal("expected timeout error")
	}

	timeoutErr, ok := err.(*ExecutionTimeoutError)
	if !ok {
		t.Fatalf("expected ExecutionTimeoutError, got %T", err)
	}
	if timeoutErr.Elapsed < timeoutErr.Limit {
		t.Errorf("expected elapsed >= limit")
	}
}

func TestFailSafeManager_LoopCount(t *testing.T) {
	fs := NewFailSafeManager(10*time.Second, 3)

	// Execute same element 3 times (should pass)
	for i := 0; i < 3; i++ {
		if err := fs.Check("elem-1"); err != nil {
			t.Fatalf("unexpected error on iteration %d: %v", i+1, err)
		}
	}

	// 4th execution should fail
	err := fs.Check("elem-1")
	if err == nil {
		t.Fatal("expected loop count error")
	}

	loopErr, ok := err.(*NestedLoopError)
	if !ok {
		t.Fatalf("expected NestedLoopError, got %T", err)
	}
	if loopErr.Count != 4 {
		t.Errorf("expected count 4, got %d", loopErr.Count)
	}
	if loopErr.Limit != 3 {
		t.Errorf("expected limit 3, got %d", loopErr.Limit)
	}
}

func TestFailSafeManager_DifferentElements(t *testing.T) {
	fs := NewFailSafeManager(10*time.Second, 2)

	// Execute different elements - should not trigger loop limit
	for i := 0; i < 10; i++ {
		elemID := "elem-" + string(rune('a'+i))
		if err := fs.Check(elemID); err != nil {
			t.Fatalf("unexpected error for %s: %v", elemID, err)
		}
	}

	if fs.TotalExecutions() != 10 {
		t.Errorf("expected 10 total executions, got %d", fs.TotalExecutions())
	}
}

func TestFlowRouter_Route(t *testing.T) {
	router := NewFlowRouter()

	proc := &bpmn.Process{
		Elements: map[string]bpmn.Element{
			"gw-1": {ID: "gw-1", OutgoingFlows: []string{"flow-a", "flow-b"}},
			"end-a": {ID: "end-a", Type: bpmn.ElementTypeEndEvent},
			"end-b": {ID: "end-b", Type: bpmn.ElementTypeEndEvent},
		},
		Flows: map[string]bpmn.Flow{
			"flow-a": {ID: "flow-a", SourceRef: "gw-1", TargetRef: "end-a"},
			"flow-b": {ID: "flow-b", SourceRef: "gw-1", TargetRef: "end-b"},
		},
	}

	flow := &store.FlowRecord{
		ElementID: "gw-1",
		ThreadID:  1,
	}

	result := ExecutionResult{
		Action:   ActionRoute,
		FlowData: flow,
	}

	next := router.Route(result, proc, 1)
	if len(next) != 2 {
		t.Errorf("expected 2 next flows, got %d", len(next))
	}
}

func TestFlowRouter_RouteWithFilters(t *testing.T) {
	router := NewFlowRouter()

	proc := &bpmn.Process{
		Elements: map[string]bpmn.Element{
			"gw-1": {ID: "gw-1", OutgoingFlows: []string{"flow-a", "flow-b", "flow-c"}},
			"end-a": {ID: "end-a", Type: bpmn.ElementTypeEndEvent},
			"end-b": {ID: "end-b", Type: bpmn.ElementTypeEndEvent},
			"end-c": {ID: "end-c", Type: bpmn.ElementTypeEndEvent},
		},
		Flows: map[string]bpmn.Flow{
			"flow-a": {ID: "flow-a", SourceRef: "gw-1", TargetRef: "end-a"},
			"flow-b": {ID: "flow-b", SourceRef: "gw-1", TargetRef: "end-b"},
			"flow-c": {ID: "flow-c", SourceRef: "gw-1", TargetRef: "end-c"},
		},
	}

	flow := &store.FlowRecord{
		ElementID: "gw-1",
		ThreadID:  1,
	}

	result := ExecutionResult{
		Action:      ActionRoute,
		FlowData:    flow,
		FlowFilters: []string{"flow-b"},
	}

	next := router.Route(result, proc, 1)
	if len(next) != 1 {
		t.Errorf("expected 1 next flow with filter, got %d", len(next))
	}
	if len(next) > 0 && next[0].FlowID != "flow-b" {
		t.Errorf("expected flow-b, got %s", next[0].FlowID)
	}
}

func TestElementRegistry_RegisterAndGet(t *testing.T) {
	reg := NewElementRegistry()

	reg.Register(bpmn.ElementTypeStartEvent, events.NewStartEvent)

	if !reg.Has(bpmn.ElementTypeStartEvent) {
		t.Error("expected registry to have startEvent")
	}

	elemDef := bpmn.Element{ID: "start-1", Type: bpmn.ElementTypeStartEvent, Name: "Start"}
	elem, err := reg.Get(elemDef)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if elem.ID() != "start-1" {
		t.Errorf("expected ID start-1, got %s", elem.ID())
	}
	if elem.Type() != bpmn.ElementTypeStartEvent {
		t.Errorf("expected type startEvent, got %s", elem.Type())
	}
}

func TestElementRegistry_GetUnknown(t *testing.T) {
	reg := NewElementRegistry()

	elemDef := bpmn.Element{ID: "unknown-1", Type: "unknown"}
	_, err := reg.Get(elemDef)
	if err == nil {
		t.Fatal("expected error for unknown element type")
	}
}

func TestExecutionContext(t *testing.T) {
	proc := &bpmn.Process{
		ID:           "proc-1",
		StartEventID: "start-1",
		Elements: map[string]bpmn.Element{
			"start-1": {ID: "start-1", Type: bpmn.ElementTypeStartEvent},
		},
	}

	instance := process.NewInstance(proc, map[string]any{"key": "value"})
	flow := &store.FlowRecord{ElementID: "start-1"}
	store := memory.NewStore()
	logger, _ := observability.NewFromConfig("info", "text")

	ctx := NewExecutionContext(context.Background(), instance, flow, store, logger)

	// Test variable access
	val, ok := ctx.GetVariable("key")
	if !ok || val != "value" {
		t.Errorf("expected variable value, got %v", val)
	}

	// Test set variable
	ctx.SetVariable("new", 42)
	val, ok = ctx.GetVariable("new")
	if !ok || val != 42 {
		t.Errorf("expected new variable to be 42, got %v", val)
	}

	// Test element access
	elem, ok := ctx.Element()
	if !ok {
		t.Fatal("expected element")
	}
	if elem.ID != "start-1" {
		t.Errorf("expected element ID start-1, got %s", elem.ID)
	}
}

func TestEngine_Run_TerminateEvent(t *testing.T) {
	proc := &bpmn.Process{
		ID:           "proc-term",
		Name:         "Terminate Process",
		StartEventID: "start-1",
		Elements: map[string]bpmn.Element{
			"start-1": {ID: "start-1", Type: bpmn.ElementTypeStartEvent, OutgoingFlows: []string{"flow-1"}},
			"term-1":  {ID: "term-1", Type: bpmn.ElementTypeTerminateEvent, IncomingFlows: []string{"flow-1"}},
		},
		Flows: map[string]bpmn.Flow{
			"flow-1": {ID: "flow-1", SourceRef: "start-1", TargetRef: "term-1"},
		},
	}

	registry := NewElementRegistry()
	registry.Register(bpmn.ElementTypeStartEvent, events.NewStartEvent)
	registry.Register(bpmn.ElementTypeTerminateEvent, events.NewTerminateEvent)

	s := memory.NewStore()
	logger, _ := observability.NewFromConfig("info", "text")
	eng := New(Config{WorkerCount: 1, MaxLoops: 100, ExecutionTimeout: 5 * time.Second}, registry, s, logger, nil)

	instance := process.NewInstance(proc, nil)
	if err := s.CreateInstance(context.Background(), instance.ToRecord()); err != nil {
		t.Fatalf("create instance: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := eng.Run(ctx, instance)
	if err == nil {
		t.Fatal("expected error from terminate event")
	}
	if !strings.Contains(err.Error(), "process terminated") {
		t.Errorf("expected process terminated error, got %v", err)
	}
	if instance.State != process.StateTerminated {
		t.Errorf("expected state TERMINATED, got %s", instance.State)
	}
}

func TestEngine_Run_UserTask(t *testing.T) {
	proc := &bpmn.Process{
		ID:           "proc-task",
		Name:         "User Task Process",
		StartEventID: "start-1",
		Elements: map[string]bpmn.Element{
			"start-1": {ID: "start-1", Type: bpmn.ElementTypeStartEvent, OutgoingFlows: []string{"flow-1"}},
			"task-1":  {ID: "task-1", Type: bpmn.ElementTypeUserTask, IncomingFlows: []string{"flow-1"}, OutgoingFlows: []string{"flow-2"}, Assignee: "user-1"},
			"end-1":   {ID: "end-1", Type: bpmn.ElementTypeEndEvent, IncomingFlows: []string{"flow-2"}},
		},
		Flows: map[string]bpmn.Flow{
			"flow-1": {ID: "flow-1", SourceRef: "start-1", TargetRef: "task-1"},
			"flow-2": {ID: "flow-2", SourceRef: "task-1", TargetRef: "end-1"},
		},
	}

	registry := NewElementRegistry()
	registry.Register(bpmn.ElementTypeStartEvent, events.NewStartEvent)
	registry.Register(bpmn.ElementTypeEndEvent, events.NewEndEvent)
	registry.Register(bpmn.ElementTypeUserTask, activities.NewUserTask)

	s := memory.NewStore()
	logger, _ := observability.NewFromConfig("info", "text")
	eng := New(Config{WorkerCount: 1, MaxLoops: 100, ExecutionTimeout: 5 * time.Second}, registry, s, logger, nil)

	instance := process.NewInstance(proc, nil)
	if err := s.CreateInstance(context.Background(), instance.ToRecord()); err != nil {
		t.Fatalf("create instance: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := eng.Run(ctx, instance)
	if err != nil {
		t.Fatalf("engine run failed: %v", err)
	}
	if instance.State != process.StateWaiting {
		t.Errorf("expected state WAITING, got %s", instance.State)
	}
}

func TestEngine_Run_ScriptTask(t *testing.T) {
	proc := &bpmn.Process{
		ID:           "proc-script",
		Name:         "Script Task Process",
		StartEventID: "start-1",
		Elements: map[string]bpmn.Element{
			"start-1":  {ID: "start-1", Type: bpmn.ElementTypeStartEvent, OutgoingFlows: []string{"flow-1"}},
			"script-1": {ID: "script-1", Type: bpmn.ElementTypeScriptTask, IncomingFlows: []string{"flow-1"}, OutgoingFlows: []string{"flow-2"}},
			"end-1":    {ID: "end-1", Type: bpmn.ElementTypeEndEvent, IncomingFlows: []string{"flow-2"}},
		},
		Flows: map[string]bpmn.Flow{
			"flow-1": {ID: "flow-1", SourceRef: "start-1", TargetRef: "script-1"},
			"flow-2": {ID: "flow-2", SourceRef: "script-1", TargetRef: "end-1"},
		},
	}

	registry := NewElementRegistry()
	registry.Register(bpmn.ElementTypeStartEvent, events.NewStartEvent)
	registry.Register(bpmn.ElementTypeEndEvent, events.NewEndEvent)
	registry.Register(bpmn.ElementTypeScriptTask, activities.NewScriptTask)

	s := memory.NewStore()
	logger, _ := observability.NewFromConfig("info", "text")
	eng := New(Config{WorkerCount: 1, MaxLoops: 100, ExecutionTimeout: 5 * time.Second}, registry, s, logger, nil)

	instance := process.NewInstance(proc, nil)
	if err := s.CreateInstance(context.Background(), instance.ToRecord()); err != nil {
		t.Fatalf("create instance: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := eng.Run(ctx, instance)
	if err != nil {
		t.Fatalf("engine run failed: %v", err)
	}
	if instance.State != process.StateCompleted {
		t.Errorf("expected state COMPLETED, got %s", instance.State)
	}
}

func TestEngine_Run_ServiceTask(t *testing.T) {
	proc := &bpmn.Process{
		ID:           "proc-svc",
		Name:         "Service Task Process",
		StartEventID: "start-1",
		Elements: map[string]bpmn.Element{
			"start-1": {ID: "start-1", Type: bpmn.ElementTypeStartEvent, OutgoingFlows: []string{"flow-1"}},
			"svc-1":   {ID: "svc-1", Type: bpmn.ElementTypeServiceTask, IncomingFlows: []string{"flow-1"}, OutgoingFlows: []string{"flow-2"}},
			"end-1":   {ID: "end-1", Type: bpmn.ElementTypeEndEvent, IncomingFlows: []string{"flow-2"}},
		},
		Flows: map[string]bpmn.Flow{
			"flow-1": {ID: "flow-1", SourceRef: "start-1", TargetRef: "svc-1"},
			"flow-2": {ID: "flow-2", SourceRef: "svc-1", TargetRef: "end-1"},
		},
	}

	registry := NewElementRegistry()
	registry.Register(bpmn.ElementTypeStartEvent, events.NewStartEvent)
	registry.Register(bpmn.ElementTypeEndEvent, events.NewEndEvent)
	registry.Register(bpmn.ElementTypeServiceTask, activities.NewServiceTask)

	s := memory.NewStore()
	logger, _ := observability.NewFromConfig("info", "text")
	eng := New(Config{WorkerCount: 1, MaxLoops: 100, ExecutionTimeout: 5 * time.Second}, registry, s, logger, nil)

	instance := process.NewInstance(proc, nil)
	if err := s.CreateInstance(context.Background(), instance.ToRecord()); err != nil {
		t.Fatalf("create instance: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := eng.Run(ctx, instance)
	if err != nil {
		t.Fatalf("engine run failed: %v", err)
	}
	if instance.State != process.StateCompleted {
		t.Errorf("expected state COMPLETED, got %s", instance.State)
	}
}

func TestEngine_Run_ExclusiveGatewayWithConditions(t *testing.T) {
	proc := &bpmn.Process{
		ID:           "proc-cond",
		Name:         "Conditional",
		StartEventID: "start-1",
		Elements: map[string]bpmn.Element{
			"start-1": {ID: "start-1", Type: bpmn.ElementTypeStartEvent, OutgoingFlows: []string{"flow-1"}},
			"gw-1": {ID: "gw-1", Type: bpmn.ElementTypeExclusiveGateway,
				IncomingFlows: []string{"flow-1"}, OutgoingFlows: []string{"flow-2", "flow-3"},
				DefaultFlowID: "flow-3"},
			"end-app": {ID: "end-app", Type: bpmn.ElementTypeEndEvent, IncomingFlows: []string{"flow-2"}},
			"end-rej": {ID: "end-rej", Type: bpmn.ElementTypeEndEvent, IncomingFlows: []string{"flow-3"}},
		},
		Flows: map[string]bpmn.Flow{
			"flow-1": {ID: "flow-1", SourceRef: "start-1", TargetRef: "gw-1"},
			"flow-2": {ID: "flow-2", SourceRef: "gw-1", TargetRef: "end-app"},
			"flow-3": {ID: "flow-3", SourceRef: "gw-1", TargetRef: "end-rej"},
		},
	}

	registry := NewElementRegistry()
	registry.Register(bpmn.ElementTypeStartEvent, events.NewStartEvent)
	registry.Register(bpmn.ElementTypeEndEvent, events.NewEndEvent)
	registry.Register(bpmn.ElementTypeExclusiveGateway, gateways.NewExclusiveGateway)

	s := memory.NewStore()
	logger, _ := observability.NewFromConfig("info", "text")
	eng := New(Config{WorkerCount: 1, MaxLoops: 100, ExecutionTimeout: 5 * time.Second}, registry, s, logger, nil)

	instance := process.NewInstance(proc, nil)
	instance.SetVariable("approved", true)
	if err := s.CreateInstance(context.Background(), instance.ToRecord()); err != nil {
		t.Fatalf("create instance: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := eng.Run(ctx, instance)
	if err != nil {
		t.Fatalf("engine run failed: %v", err)
	}
	if instance.State != process.StateCompleted {
		t.Errorf("expected state COMPLETED, got %s", instance.State)
	}
}

func TestEngine_Run_InclusiveGateway(t *testing.T) {
	proc := &bpmn.Process{
		ID:           "proc-inc",
		Name:         "Inclusive",
		StartEventID: "start-1",
		Elements: map[string]bpmn.Element{
			"start-1": {ID: "start-1", Type: bpmn.ElementTypeStartEvent, OutgoingFlows: []string{"flow-1"}},
			"gw-inc":  {ID: "gw-inc", Type: bpmn.ElementTypeInclusiveGateway, IncomingFlows: []string{"flow-1"}, OutgoingFlows: []string{"flow-2", "flow-3"}},
			"end-a":   {ID: "end-a", Type: bpmn.ElementTypeEndEvent, IncomingFlows: []string{"flow-2"}},
			"end-b":   {ID: "end-b", Type: bpmn.ElementTypeEndEvent, IncomingFlows: []string{"flow-3"}},
		},
		Flows: map[string]bpmn.Flow{
			"flow-1": {ID: "flow-1", SourceRef: "start-1", TargetRef: "gw-inc"},
			"flow-2": {ID: "flow-2", SourceRef: "gw-inc", TargetRef: "end-a"},
			"flow-3": {ID: "flow-3", SourceRef: "gw-inc", TargetRef: "end-b"},
		},
	}

	registry := NewElementRegistry()
	registry.Register(bpmn.ElementTypeStartEvent, events.NewStartEvent)
	registry.Register(bpmn.ElementTypeEndEvent, events.NewEndEvent)
	registry.Register(bpmn.ElementTypeInclusiveGateway, gateways.NewInclusiveGateway)

	s := memory.NewStore()
	logger, _ := observability.NewFromConfig("info", "text")
	eng := New(Config{WorkerCount: 1, MaxLoops: 100, ExecutionTimeout: 5 * time.Second}, registry, s, logger, nil)

	instance := process.NewInstance(proc, nil)
	if err := s.CreateInstance(context.Background(), instance.ToRecord()); err != nil {
		t.Fatalf("create instance: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := eng.Run(ctx, instance)
	if err != nil {
		t.Fatalf("engine run failed: %v", err)
	}
	if instance.State != process.StateCompleted {
		t.Errorf("expected state COMPLETED, got %s", instance.State)
	}
}

func TestEngine_Run_MessageThrowEvent(t *testing.T) {
	proc := &bpmn.Process{
		ID:           "proc-msg",
		Name:         "Message",
		StartEventID: "start-1",
		Elements: map[string]bpmn.Element{
			"start-1": {ID: "start-1", Type: bpmn.ElementTypeStartEvent, OutgoingFlows: []string{"flow-1"}},
			"msg-1":   {ID: "msg-1", Type: bpmn.ElementTypeMessageThrow, IncomingFlows: []string{"flow-1"}, OutgoingFlows: []string{"flow-2"}},
			"end-1":   {ID: "end-1", Type: bpmn.ElementTypeEndEvent, IncomingFlows: []string{"flow-2"}},
		},
		Flows: map[string]bpmn.Flow{
			"flow-1": {ID: "flow-1", SourceRef: "start-1", TargetRef: "msg-1"},
			"flow-2": {ID: "flow-2", SourceRef: "msg-1", TargetRef: "end-1"},
		},
	}

	registry := NewElementRegistry()
	registry.Register(bpmn.ElementTypeStartEvent, events.NewStartEvent)
	registry.Register(bpmn.ElementTypeEndEvent, events.NewEndEvent)
	registry.Register(bpmn.ElementTypeMessageThrow, events.NewMessageThrowEvent)

	s := memory.NewStore()
	logger, _ := observability.NewFromConfig("info", "text")
	eng := New(Config{WorkerCount: 1, MaxLoops: 100, ExecutionTimeout: 5 * time.Second}, registry, s, logger, nil)

	instance := process.NewInstance(proc, nil)
	if err := s.CreateInstance(context.Background(), instance.ToRecord()); err != nil {
		t.Fatalf("create instance: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := eng.Run(ctx, instance)
	if err != nil {
		t.Fatalf("engine run failed: %v", err)
	}
	if instance.State != process.StateCompleted {
		t.Errorf("expected state COMPLETED, got %s", instance.State)
	}
}

func TestEngine_Run_EventBasedGateway(t *testing.T) {
	proc := &bpmn.Process{
		ID:           "proc-eb",
		Name:         "Event Based",
		StartEventID: "start-1",
		Elements: map[string]bpmn.Element{
			"start-1": {ID: "start-1", Type: bpmn.ElementTypeStartEvent, OutgoingFlows: []string{"flow-1"}},
			"gw-eb":   {ID: "gw-eb", Type: bpmn.ElementTypeEventBasedGateway, IncomingFlows: []string{"flow-1"}, OutgoingFlows: []string{"flow-2", "flow-3"}},
			"end-1":   {ID: "end-1", Type: bpmn.ElementTypeEndEvent, IncomingFlows: []string{"flow-2"}},
			"end-2":   {ID: "end-2", Type: bpmn.ElementTypeEndEvent, IncomingFlows: []string{"flow-3"}},
		},
		Flows: map[string]bpmn.Flow{
			"flow-1": {ID: "flow-1", SourceRef: "start-1", TargetRef: "gw-eb"},
			"flow-2": {ID: "flow-2", SourceRef: "gw-eb", TargetRef: "end-1"},
			"flow-3": {ID: "flow-3", SourceRef: "gw-eb", TargetRef: "end-2"},
		},
	}

	registry := NewElementRegistry()
	registry.Register(bpmn.ElementTypeStartEvent, events.NewStartEvent)
	registry.Register(bpmn.ElementTypeEndEvent, events.NewEndEvent)
	registry.Register(bpmn.ElementTypeEventBasedGateway, gateways.NewEventBasedGateway)

	s := memory.NewStore()
	logger, _ := observability.NewFromConfig("info", "text")
	eng := New(Config{WorkerCount: 1, MaxLoops: 100, ExecutionTimeout: 5 * time.Second}, registry, s, logger, nil)

	instance := process.NewInstance(proc, nil)
	if err := s.CreateInstance(context.Background(), instance.ToRecord()); err != nil {
		t.Fatalf("create instance: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := eng.Run(ctx, instance)
	if err != nil {
		t.Fatalf("engine run failed: %v", err)
	}
	if instance.State != process.StateWaiting {
		t.Errorf("expected state WAITING for event-based gateway, got %s", instance.State)
	}
}

func TestEngine_Run_UserTaskFlow(t *testing.T) {
	proc := &bpmn.Process{
		ID:           "proc-flow",
		Name:         "User task flow",
		StartEventID: "start-1",
		Elements: map[string]bpmn.Element{
			"start-1": {ID: "start-1", Type: bpmn.ElementTypeStartEvent, OutgoingFlows: []string{"flow-1"}},
			"task-1":  {ID: "task-1", Type: bpmn.ElementTypeUserTask, IncomingFlows: []string{"flow-1"}, OutgoingFlows: []string{"flow-2"}, Assignee: "user-1"},
			"end-1":   {ID: "end-1", Type: bpmn.ElementTypeEndEvent, IncomingFlows: []string{"flow-2"}},
		},
		Flows: map[string]bpmn.Flow{
			"flow-1": {ID: "flow-1", SourceRef: "start-1", TargetRef: "task-1"},
			"flow-2": {ID: "flow-2", SourceRef: "task-1", TargetRef: "end-1"},
		},
	}

	registry := NewElementRegistry()
	registry.Register(bpmn.ElementTypeStartEvent, events.NewStartEvent)
	registry.Register(bpmn.ElementTypeEndEvent, events.NewEndEvent)
	registry.Register(bpmn.ElementTypeUserTask, activities.NewUserTask)

	s := memory.NewStore()
	logger, _ := observability.NewFromConfig("info", "text")
	eng := New(Config{WorkerCount: 1, MaxLoops: 100, ExecutionTimeout: 5 * time.Second}, registry, s, logger, nil)

	instance := process.NewInstance(proc, nil)
	if err := s.CreateInstance(context.Background(), instance.ToRecord()); err != nil {
		t.Fatalf("create instance: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := eng.Run(ctx, instance)
	if err != nil {
		t.Fatalf("engine run failed: %v", err)
	}
	if instance.State != process.StateWaiting {
		t.Errorf("expected WAITING after user task, got %s", instance.State)
	}

	// Verify that the user task assigned variables were set
	assignee, ok := instance.GetVariable("task_assignee")
	if !ok || assignee != "user-1" {
		t.Errorf("expected task_assignee=user-1, got %v", assignee)
	}
}

func TestEngine_UnregisteredElement(t *testing.T) {
	proc := &bpmn.Process{
		ID:           "proc-unk",
		Name:         "Unknown Element",
		StartEventID: "start-1",
		Elements: map[string]bpmn.Element{
			"start-1": {ID: "start-1", Type: bpmn.ElementTypeStartEvent, OutgoingFlows: []string{"flow-1"}},
			"unk-1":   {ID: "unk-1", Type: "unknownType", IncomingFlows: []string{"flow-1"}},
		},
		Flows: map[string]bpmn.Flow{
			"flow-1": {ID: "flow-1", SourceRef: "start-1", TargetRef: "unk-1"},
		},
	}

	registry := NewElementRegistry()
	registry.Register(bpmn.ElementTypeStartEvent, events.NewStartEvent)

	s := memory.NewStore()
	logger, _ := observability.NewFromConfig("info", "text")
	eng := New(Config{WorkerCount: 1, MaxLoops: 100, ExecutionTimeout: 5 * time.Second}, registry, s, logger, nil)

	instance := process.NewInstance(proc, nil)
	if err := s.CreateInstance(context.Background(), instance.ToRecord()); err != nil {
		t.Fatalf("create instance: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := eng.Run(ctx, instance)
	if err == nil {
		t.Fatal("expected error for unregistered element")
	}
}

func TestEngine_Run_TimerEvent_NoValue(t *testing.T) {
	proc := &bpmn.Process{
		ID:           "proc-timer",
		Name:         "Timer",
		StartEventID: "start-1",
		Elements: map[string]bpmn.Element{
			"start-1": {ID: "start-1", Type: bpmn.ElementTypeStartEvent, OutgoingFlows: []string{"flow-1"}},
			"timer-1": {ID: "timer-1", Type: bpmn.ElementTypeTimerEvent, IncomingFlows: []string{"flow-1"}, OutgoingFlows: []string{"flow-2"}},
			"end-1":   {ID: "end-1", Type: bpmn.ElementTypeEndEvent, IncomingFlows: []string{"flow-2"}},
		},
		Flows: map[string]bpmn.Flow{
			"flow-1": {ID: "flow-1", SourceRef: "start-1", TargetRef: "timer-1"},
			"flow-2": {ID: "flow-2", SourceRef: "timer-1", TargetRef: "end-1"},
		},
	}

	registry := NewElementRegistry()
	registry.Register(bpmn.ElementTypeStartEvent, events.NewStartEvent)
	registry.Register(bpmn.ElementTypeEndEvent, events.NewEndEvent)
	registry.Register(bpmn.ElementTypeTimerEvent, events.NewTimerEvent)

	s := memory.NewStore()
	logger, _ := observability.NewFromConfig("info", "text")
	eng := New(Config{WorkerCount: 1, MaxLoops: 100, ExecutionTimeout: 5 * time.Second}, registry, s, logger, nil)

	instance := process.NewInstance(proc, nil)
	if err := s.CreateInstance(context.Background(), instance.ToRecord()); err != nil {
		t.Fatalf("create instance: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := eng.Run(ctx, instance)
	if err != nil {
		t.Fatalf("engine run failed: %v", err)
	}
	if instance.State != process.StateCompleted {
		t.Errorf("expected state COMPLETED, got %s", instance.State)
	}
}

func TestEngine_ActionConstants(t *testing.T) {
	if ActionRoute != "ROUTE" {
		t.Errorf("expected ROUTE, got %s", ActionRoute)
	}
	if ActionWait != "WAIT" {
		t.Errorf("expected WAIT, got %s", ActionWait)
	}
	if ActionForm != "FORM" {
		t.Errorf("expected FORM, got %s", ActionForm)
	}
	if ActionError != "ERROR" {
		t.Errorf("expected ERROR, got %s", ActionError)
	}
	if ActionComplete != "COMPLETE" {
		t.Errorf("expected COMPLETE, got %s", ActionComplete)
	}
	if ActionSkip != "SKIP" {
		t.Errorf("expected SKIP, got %s", ActionSkip)
	}
	if ActionQueue != "QUEUE" {
		t.Errorf("expected QUEUE, got %s", ActionQueue)
	}
	if ActionTerminate != "TERMINATE" {
		t.Errorf("expected TERMINATE, got %s", ActionTerminate)
	}
}

func TestNewResult(t *testing.T) {
	flow := &store.FlowRecord{ElementID: "test"}
	result := NewResult(ActionRoute, flow)
	if result.Action != ActionRoute {
		t.Errorf("expected ActionRoute, got %s", result.Action)
	}
	if result.FlowData != flow {
		t.Errorf("expected same flow reference")
	}
}

func TestNewResultWithFilters(t *testing.T) {
	flow := &store.FlowRecord{ElementID: "test"}
	filters := []string{"flow-1", "flow-2"}
	result := NewResultWithFilters(ActionRoute, flow, filters)
	if result.Action != ActionRoute {
		t.Errorf("expected ActionRoute, got %s", result.Action)
	}
	if len(result.FlowFilters) != 2 {
		t.Errorf("expected 2 filters, got %d", len(result.FlowFilters))
	}
}

func TestNewErrorResult(t *testing.T) {
	flow := &store.FlowRecord{ElementID: "test"}
	err := assertAnError("test error")
	result := NewErrorResult(flow, err)
	if result.Action != ActionError {
		t.Errorf("expected ActionError, got %s", result.Action)
	}
	if result.Error == nil {
		t.Fatal("expected error")
	}
	if result.Error.Error() != "test error" {
		t.Errorf("expected 'test error', got %s", result.Error.Error())
	}
}

type assertAnError string

func (e assertAnError) Error() string {
	return string(e)
}
