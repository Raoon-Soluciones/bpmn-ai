package engine

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/Raoon-Soluciones/bpmn-ai/internal/element"
	"github.com/Raoon-Soluciones/bpmn-ai/internal/element/gateways"
	"github.com/Raoon-Soluciones/bpmn-ai/internal/observability"
	"github.com/Raoon-Soluciones/bpmn-ai/internal/process"
	"github.com/Raoon-Soluciones/bpmn-ai/internal/queue"
	"github.com/Raoon-Soluciones/bpmn-ai/pkg/bpmn"
	"github.com/Raoon-Soluciones/bpmn-ai/pkg/store"
)

// Compile-time check that *engine implements Engine.
var _ Engine = (*engine)(nil)

// Config holds engine configuration.
type Config struct {
	WorkerCount      int
	MaxLoops         int
	ExecutionTimeout time.Duration
}

// Engine defines the public API for the BPMN execution engine.
type Engine interface {
	// Run executes a process instance from its start event.
	Run(ctx context.Context, instance *process.Instance) error

	// Continue resumes execution after a task is completed.
	Continue(ctx context.Context, instanceID string, completedFlowID string, variables map[string]any) error

	// SendMessage delivers a message to a waiting MessageCatch event.
	SendMessage(ctx context.Context, instanceID string, messageRef string, variables map[string]any) error

	// Registry returns the element registry.
	Registry() *ElementRegistry

	// WithDispatcher sets the event dispatcher.
	WithDispatcher(d *observability.Dispatcher) Engine

	// JobHandler returns the queue job handler for engine-managed jobs.
	JobHandler() queue.JobHandler
}

// engine is the BPMN execution engine with an iterative loop.
type engine struct {
	config     Config
	registry   *ElementRegistry
	router     *FlowRouter
	store      EngineStore
	logger     *observability.Logger
	queue      *queue.WorkerPool
	dispatcher *observability.Dispatcher
}

// New creates a new engine.
func New(cfg Config, registry *ElementRegistry, s EngineStore, logger *observability.Logger, q *queue.WorkerPool) Engine {
	if cfg.WorkerCount < 1 {
		cfg.WorkerCount = 1
	}
	if cfg.MaxLoops < 1 {
		cfg.MaxLoops = 100
	}
	if cfg.ExecutionTimeout < 1 {
		cfg.ExecutionTimeout = 30 * time.Second
	}

	return &engine{
		config:     cfg,
		registry:   registry,
		router:     NewFlowRouter(),
		store:      s,
		logger:     logger,
		queue:      q,
		dispatcher: observability.NewDispatcher(),
	}
}

// Registry returns the element registry.
func (e *engine) Registry() *ElementRegistry {
	return e.registry
}

// WithDispatcher sets the event dispatcher for the engine.
func (e *engine) WithDispatcher(d *observability.Dispatcher) Engine {
	e.dispatcher = d
	return e
}

// workItem represents a unit of work for the engine.
type workItem struct {
	flow     *store.FlowRecord
	threadID int
}

// Run executes a process instance from its start event.
func (e *engine) Run(ctx context.Context, instance *process.Instance) error {
	if instance.Process.StartEventID == "" {
		return fmt.Errorf("process %s has no start event", instance.ProcessID)
	}

	if instance.State != process.StateInProgress {
		if err := instance.Transition(process.StateInProgress); err != nil {
			return fmt.Errorf("transition to in_progress: %w", err)
		}
	}

	initialFlow := &store.FlowRecord{
		InstanceID:  instance.ID,
		ElementID:   instance.Process.StartEventID,
		ElementType: bpmn.ElementTypeStartEvent,
		ThreadID:    1,
		Status:      store.FlowStatusActive,
	}
	if err := e.store.CreateFlow(ctx, initialFlow); err != nil {
		return fmt.Errorf("create initial flow: %w", err)
	}

	thread := process.NewThread(instance.ID, 1, nil, initialFlow.ID)
	threadRec := &store.ThreadRecord{
		InstanceID:  thread.InstanceID,
		ThreadIndex: thread.ThreadIndex,
		FlowID:      thread.CurrentFlowID,
		Status:      thread.State,
		CreatedAt:   thread.CreatedAt,
	}
	if err := e.store.CreateThread(ctx, threadRec); err != nil {
		return fmt.Errorf("create thread: %w", err)
	}

	e.dispatcher.DispatchAsync(observability.Event{
		Type:      observability.EventProcessStarted,
		Timestamp: time.Now(),
		Payload: map[string]any{
			"instance_id":  instance.ID,
			"process_id":   instance.ProcessID,
			"process_name": instance.Process.Name,
			"element_id":   instance.Process.StartEventID,
			"element_type": string(bpmn.ElementTypeStartEvent),
			"action":       string(ActionRoute),
			"thread_id":    1,
			"from_state":   string(process.StateCreated),
			"to_state":     string(process.StateInProgress),
		},
	})

	return e.runLoop(ctx, instance, []workItem{{flow: initialFlow, threadID: 1}})
}

// Continue resumes execution of a process instance after a task is completed.
func (e *engine) Continue(ctx context.Context, instanceID string, completedFlowID string, variables map[string]any) error {
	instRec, err := e.store.GetInstance(ctx, instanceID)
	if err != nil {
		return fmt.Errorf("get instance: %w", err)
	}

	proc, err := e.store.GetProcess(ctx, instRec.ProcessID)
	if err != nil {
		return fmt.Errorf("get process: %w", err)
	}

	instance := process.NewInstance(proc, instRec.Variables)
	instance.ID = instRec.ID
	instance.State = process.State(instRec.Status)
	instance.Title = instRec.Title
	instance.StartedAt = instRec.StartedAt
	instance.UpdatedAt = instRec.UpdatedAt
	instance.FinishedAt = instRec.FinishedAt

	for k, v := range variables {
		instance.SetVariable(k, v)
	}

	if err := instance.Transition(process.StateInProgress); err != nil {
		return fmt.Errorf("transition to in_progress: %w", err)
	}

	flow, err := e.store.GetFlow(ctx, completedFlowID)
	if err != nil {
		return fmt.Errorf("get completed flow: %w", err)
	}

	// Check EventBasedGateway resolution — only the first event branch proceeds
	if !gateways.CheckAndResolve(execCtxForGateway(instance), flow.ElementID) {
		e.logger.Info("event-based gateway branch skipped (already resolved)",
			"instance_id", instanceID,
			"element_id", flow.ElementID,
		)
		return e.finalizeInstance(ctx, instance)
	}

	result := element.ExecutionResult{
		Action:   element.ActionRoute,
		FlowData: flow,
	}
	nextFlows := e.router.Route(result, proc, flow.ThreadID)

	if len(nextFlows) == 0 {
		if isEndEvent(flow.ElementID, proc) {
			instance.TryComplete()
		}
		return e.finalizeInstance(ctx, instance)
	}

	var initialItems []workItem
	for _, nf := range nextFlows {
		flowRec := CreateFlowRecord(instance.ID, nf.ElementID, nf.ElementType, nf.ThreadID, flow.ID)
		if err := e.store.CreateFlow(ctx, flowRec); err != nil {
			return fmt.Errorf("create flow: %w", err)
		}
		initialItems = append(initialItems, workItem{flow: flowRec, threadID: nf.ThreadID})
	}

	return e.runLoop(ctx, instance, initialItems)
}

// SendMessage delivers a message to a waiting MessageCatch event and resumes the process.
func (e *engine) SendMessage(ctx context.Context, instanceID string, messageRef string, variables map[string]any) error {
	instRec, err := e.store.GetInstance(ctx, instanceID)
	if err != nil {
		return fmt.Errorf("get instance: %w", err)
	}

	proc, err := e.store.GetProcess(ctx, instRec.ProcessID)
	if err != nil {
		return fmt.Errorf("get process: %w", err)
	}

	flows, err := e.store.GetFlowsByInstance(ctx, instanceID)
	if err != nil {
		return fmt.Errorf("get instance flows: %w", err)
	}

	// Find the waiting flow whose element is a MessageCatch expecting this messageRef
	for _, f := range flows {
		if f.Status != store.FlowStatusActive {
			continue
		}
		elem, ok := proc.Elements[f.ElementID]
		if !ok || elem.Type != bpmn.ElementTypeMessageCatch {
			continue
		}
		if elem.EventDefinition.MessageRef == messageRef {
			f.Status = store.FlowStatusCompleted
			now := time.Now()
			f.FinishedAt = &now
			if err := e.store.UpdateFlow(ctx, f); err != nil {
				return fmt.Errorf("update flow: %w", err)
			}
			return e.Continue(ctx, instanceID, f.ID, variables)
		}
	}

	return fmt.Errorf("no waiting MessageCatch found for messageRef %s in instance %s", messageRef, instanceID)
}

// runLoop executes the engine worker pool and processes results.
func (e *engine) runLoop(ctx context.Context, instance *process.Instance, initialFlows []workItem) error {
	execCtx, cancel := context.WithTimeout(ctx, e.config.ExecutionTimeout)
	defer cancel()

	failsafe := NewFailSafeManager(e.config.ExecutionTimeout, e.config.MaxLoops)

	workCh := make(chan workItem, 1024)
	resultCh := make(chan ExecutionResult, 1024)
	errCh := make(chan error, e.config.WorkerCount)

	var pendingMu sync.Mutex
	var pending int = len(initialFlows)

	var wg sync.WaitGroup
	for i := 0; i < e.config.WorkerCount; i++ {
		wg.Add(1)
		go e.worker(execCtx, instance, workCh, resultCh, &wg)
	}

	for _, item := range initialFlows {
		workCh <- item
	}

	for {
		select {
		case <-execCtx.Done():
			close(workCh)
			wg.Wait()
			close(resultCh)
			e.dispatcher.Drain()
			if execCtx.Err() == context.DeadlineExceeded {
				instance.Transition(process.StateError)
				return &ExecutionTimeoutError{
					Elapsed: e.config.ExecutionTimeout,
					Limit:   e.config.ExecutionTimeout,
				}
			}
			return execCtx.Err()

		case result, ok := <-resultCh:
			if !ok {
				wg.Wait()
				e.dispatcher.Drain()
				return e.finalizeInstance(ctx, instance)
			}

			pendingMu.Lock()
			pending--
			pendingMu.Unlock()

			if result.FlowData != nil {
				if err := failsafe.Check(result.FlowData.ElementID); err != nil {
					instance.Transition(process.StateError)
					close(workCh)
					wg.Wait()
					close(resultCh)
					e.dispatcher.Drain()
					return err
				}
			}

			if err := e.handleResult(execCtx, instance, result, workCh, errCh, &pendingMu, &pending); err != nil {
				e.dispatcher.Drain()
				return err
			}

			if result.Action == element.ActionError {
				err := <-errCh
				e.dispatcher.Drain()
				return err
			}

			pendingMu.Lock()
			allDone := pending == 0
			pendingMu.Unlock()
			if allDone {
				close(workCh)
				wg.Wait()
				close(resultCh)
				e.dispatcher.Drain()
				return e.finalizeInstance(ctx, instance)
			}

		case err := <-errCh:
			e.dispatcher.Drain()
			return err
		}
	}
}

// worker processes elements from the work queue.
func (e *engine) worker(ctx context.Context, instance *process.Instance, workCh <-chan workItem, resultCh chan<- ExecutionResult, wg *sync.WaitGroup) {
	defer wg.Done()

	for item := range workCh {
		select {
		case <-ctx.Done():
			return
		default:
			result := e.executeElement(ctx, instance, item.flow, item.threadID)
			resultCh <- result
		}
	}
}

// executeElement executes a single BPMN element.
func (e *engine) executeElement(ctx context.Context, instance *process.Instance, flow *store.FlowRecord, threadID int) ExecutionResult {
	startTime := time.Now()

	// Get the BPMN element definition
	elemDef, ok := instance.Process.Elements[flow.ElementID]
	if !ok {
		return ExecutionResult{
			Action:   ActionError,
			FlowData: flow,
			Error:    fmt.Errorf("element %s not found in process", flow.ElementID),
		}
	}

	// Get the executable element from registry
	execElement, err := e.registry.Get(elemDef)
	if err != nil {
		return ExecutionResult{
			Action:   ActionError,
			FlowData: flow,
			Error:    fmt.Errorf("get element %s: %w", flow.ElementID, err),
		}
	}

	// Create execution context
	execCtx := NewExecutionContext(ctx, instance, flow, e.store, e.logger)

	// Execute the element
	result := execElement.Execute(ctx, execCtx)

	// Calculate duration
	result.DurationMs = int(time.Since(startTime).Milliseconds())

	// Update flow record with duration
	if result.FlowData != nil {
		now := time.Now()
		result.FlowData.FinishedAt = &now
		d := result.DurationMs
		result.FlowData.DurationMs = &d
	}

	// Emit element executed event
	payload := map[string]any{
		"instance_id":   instance.ID,
		"process_id":    instance.ProcessID,
		"process_name":  instance.Process.Name,
		"element_id":    flow.ElementID,
		"element_name":  elemDef.Name,
		"element_type":  string(flow.ElementType),
		"action":        string(result.Action),
		"thread_id":     threadID,
		"duration_ms":   result.DurationMs,
	}

	if len(result.FlowFilters) > 0 {
		payload["flow_filters"] = result.FlowFilters
	}

	if len(instance.Variables) > 0 {
		payload["variables"] = cloneVariables(instance.Variables)
	}

	eventType := observability.EventElementExecuted
	if result.Action == ActionError {
		eventType = observability.EventElementError
		if result.Error != nil {
			payload["error"] = result.Error.Error()
		}
	}

	e.dispatcher.DispatchAsync(observability.Event{
		Type:      eventType,
		Timestamp: time.Now(),
		Payload:   payload,
	})

	return result
}

func cloneVariables(vars map[string]any) map[string]any {
	c := make(map[string]any, len(vars))
	for k, v := range vars {
		c[k] = v
	}
	return c
}

// enqueueFlow creates a flow record, persists it, and enqueues it for execution.
func (e *engine) enqueueFlow(
	ctx context.Context,
	instanceID string,
	next NextFlow,
	threadID int,
	prevFlowID string,
	pendingMu *sync.Mutex,
	pending *int,
	workCh chan<- workItem,
) error {
	flowRecord := CreateFlowRecord(instanceID, next.ElementID, next.ElementType, threadID, prevFlowID)
	if err := e.store.CreateFlow(ctx, flowRecord); err != nil {
		return fmt.Errorf("create flow: %w", err)
	}
	pendingMu.Lock()
	*pending++
	pendingMu.Unlock()
	select {
	case workCh <- workItem{flow: flowRecord, threadID: next.ThreadID}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// handleResult processes an execution result and enqueues next flows.
func (e *engine) handleResult(
	ctx context.Context,
	instance *process.Instance,
	result ExecutionResult,
	workCh chan<- workItem,
	errCh chan<- error,
	pendingMu *sync.Mutex,
	pending *int,
) error {
	switch result.Action {
	case element.ActionRoute:
		nextFlows := e.router.Route(result, instance.Process, result.FlowData.ThreadID)

		if len(nextFlows) > 1 {
			parentIdx := result.FlowData.ThreadID
			for _, nf := range nextFlows {
				threadIdx := instance.NextThreadID()

				thread := process.NewThread(instance.ID, threadIdx, &parentIdx, nf.FlowID)
				threadRec := &store.ThreadRecord{
					InstanceID:  thread.InstanceID,
					ThreadIndex: thread.ThreadIndex,
					ParentIndex: thread.ParentIndex,
					FlowID:      thread.CurrentFlowID,
					Status:      thread.State,
					CreatedAt:   thread.CreatedAt,
				}
				if err := e.store.CreateThread(ctx, threadRec); err != nil {
					return fmt.Errorf("create thread: %w", err)
				}

				if err := e.enqueueFlow(ctx, instance.ID, nf, threadIdx, result.FlowData.ID, pendingMu, pending, workCh); err != nil {
					return err
				}
			}
		} else if len(nextFlows) == 1 {
			nf := nextFlows[0]
			if err := e.enqueueFlow(ctx, instance.ID, nf, result.FlowData.ThreadID, result.FlowData.ID, pendingMu, pending, workCh); err != nil {
				return err
			}
		} else {
			// No next flows - check if this is an end event
			if isEndEvent(result.FlowData.ElementID, instance.Process) {
				instance.TryComplete()
			}
		}

	case element.ActionWait, element.ActionForm:
		if err := instance.Transition(process.StateWaiting); err != nil {
			return fmt.Errorf("transition to waiting: %w", err)
		}

		// Schedule auto-continue if the element specified a continuation time (e.g., TimerEvent)
		if result.ContinueAt != nil && !result.ContinueAt.IsZero() {
			job := &store.JobRecord{
				InstanceID:  instance.ID,
				FlowID:      result.FlowData.ID,
				Type:        "timer_continue",
				ScheduledAt: *result.ContinueAt,
				Payload: map[string]any{
					"element_id":  result.FlowData.ElementID,
					"instance_id": instance.ID,
					"flow_id":     result.FlowData.ID,
				},
			}
			if e.queue != nil {
				if err := e.queue.Enqueue(ctx, job); err != nil {
					e.logger.Error("failed to enqueue timer continuation job", "error", err)
				}
			} else {
				_ = e.store.CreateJob(ctx, job)
			}
			e.dispatcher.DispatchAsync(observability.Event{
				Type:      observability.EventJobQueued,
				Timestamp: time.Now(),
				Payload: map[string]any{
					"instance_id": instance.ID,
					"element_id":  result.FlowData.ElementID,
					"job_id":      job.ID,
					"job_type":    "timer_continue",
				},
			})
		}

		e.dispatcher.DispatchAsync(observability.Event{
			Type:      observability.EventProcessCompleted,
			Timestamp: time.Now(),
			Payload: map[string]any{
				"instance_id": instance.ID,
				"process_id":  instance.ProcessID,
				"from_state":  string(process.StateInProgress),
				"to_state":    string(process.StateWaiting),
			},
		})

	case element.ActionError:
		fromState := instance.State
		if err := instance.Transition(process.StateError); err != nil {
			return fmt.Errorf("transition to error: %w", err)
		}
		e.dispatcher.DispatchAsync(observability.Event{
			Type:      observability.EventProcessError,
			Timestamp: time.Now(),
			Payload: map[string]any{
				"instance_id": instance.ID,
				"process_id":  instance.ProcessID,
				"from_state":  string(fromState),
				"to_state":    string(process.StateError),
				"error":       result.Error.Error(),
			},
		})
		if result.Error != nil {
			errCh <- result.Error
		}

	case element.ActionComplete:
		if isEndEvent(result.FlowData.ElementID, instance.Process) {
			if instance.TryComplete() {
				e.dispatcher.DispatchAsync(observability.Event{
					Type:      observability.EventProcessCompleted,
					Timestamp: time.Now(),
					Payload: map[string]any{
						"instance_id": instance.ID,
						"process_id":  instance.ProcessID,
						"from_state":  string(process.StateInProgress),
						"to_state":    string(process.StateCompleted),
					},
				})
			}
		}

	case element.ActionTerminate:
		if err := instance.Transition(process.StateTerminated); err != nil {
			return fmt.Errorf("transition to terminated: %w", err)
		}
		e.dispatcher.DispatchAsync(observability.Event{
			Type:      observability.EventProcessTerminated,
			Timestamp: time.Now(),
			Payload: map[string]any{
				"instance_id": instance.ID,
				"process_id":  instance.ProcessID,
				"from_state":  string(process.StateInProgress),
				"to_state":    string(process.StateTerminated),
			},
		})
		// Close all active threads
		threads, err := e.store.GetThreadsByInstance(ctx, instance.ID)
		if err == nil {
			for _, t := range threads {
				_ = e.store.CloseThread(ctx, instance.ID, t.ThreadIndex)
			}
		}
		return fmt.Errorf("process terminated by element %s", result.FlowData.ElementID)

	case element.ActionQueue:
		e.logger.Info("element queued for async execution",
			"element_id", result.FlowData.ElementID,
			"element_type", result.FlowData.ElementType,
		)
		// Create a job for async processing
		job := &store.JobRecord{
			InstanceID: instance.ID,
			FlowID:     result.FlowData.ID,
			Type:       string(result.FlowData.ElementType),
			Payload: map[string]any{
				"element_id": result.FlowData.ElementID,
				"thread_id":  result.FlowData.ThreadID,
			},
		}
		if e.queue != nil {
			if err := e.queue.Enqueue(ctx, job); err != nil {
				e.logger.Error("failed to enqueue job", "error", err)
			}
		} else {
			job.ID = uuid.New().String()
			job.Status = store.JobStatusPending
			job.ScheduledAt = time.Now()
			job.MaxRetries = 3
			_ = e.store.CreateJob(ctx, job)
		}
		e.dispatcher.DispatchAsync(observability.Event{
			Type:      observability.EventJobQueued,
			Timestamp: time.Now(),
			Payload: map[string]any{
				"instance_id": instance.ID,
				"process_id":  instance.ProcessID,
				"element_id":  result.FlowData.ElementID,
				"job_id":      job.ID,
				"job_type":    job.Type,
			},
		})
		// Route to next flows immediately while queue is processed asynchronously
		nextFlows := e.router.Route(result, instance.Process, result.FlowData.ThreadID)
		for _, nf := range nextFlows {
			if err := e.enqueueFlow(ctx, instance.ID, nf, result.FlowData.ThreadID, result.FlowData.ID, pendingMu, pending, workCh); err != nil {
				return err
			}
		}

	case element.ActionSkip:
		// Element skipped, no action needed
	}

	// Log execution
	if result.FlowData != nil {
		_ = e.store.LogExecution(ctx, &store.ExecutionLogEntry{
			InstanceID:  instance.ID,
			ElementID:   result.FlowData.ElementID,
			ElementType: result.FlowData.ElementType,
			Action:      string(result.Action),
			DurationMs:  result.DurationMs,
		})
	}

	return nil
}

// finalizeInstance updates the instance record in the store.
func (e *engine) finalizeInstance(ctx context.Context, instance *process.Instance) error {
	return e.store.UpdateInstance(ctx, instance.ToRecord())
}

// execCtxForGateway creates a minimal execution context for gateway checks.
func execCtxForGateway(inst *process.Instance) element.ExecutionContext {
	return &gatewayExecCtx{instance: inst}
}

type gatewayExecCtx struct {
	instance *process.Instance
}

func (c *gatewayExecCtx) Instance() element.Instance       { return c.instance }
func (c *gatewayExecCtx) Flow() *store.FlowRecord           { return nil }
func (c *gatewayExecCtx) GetVariable(key string) (any, bool) { return c.instance.GetVariable(key) }
func (c *gatewayExecCtx) SetVariable(key string, value any)  { c.instance.SetVariable(key, value) }
func (c *gatewayExecCtx) Store() element.ElementStore        { return nil }
func (c *gatewayExecCtx) Element() (bpmn.Element, bool)      { return bpmn.Element{}, false }

func isEndEvent(elementID string, proc *bpmn.Process) bool {
	elem, ok := proc.Elements[elementID]
	if !ok {
		return false
	}
	return elem.Type == bpmn.ElementTypeEndEvent || elem.Type == bpmn.ElementTypeTerminateEvent
}

// JobHandler returns the queue job handler that processes engine-managed jobs
// (e.g., timer_continue for scheduled TimerEvent resumption).
func (e *engine) JobHandler() queue.JobHandler {
	return func(ctx context.Context, job *store.JobRecord) error {
		switch job.Type {
		case "timer_continue":
			return e.Continue(ctx, job.InstanceID, job.FlowID, nil)
		default:
			e.logger.Info("unknown job type in engine handler", "type", job.Type)
			return nil
		}
	}
}
