package engine

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/Raoon-Soluciones/bpmn-ai/internal/element"
	"github.com/Raoon-Soluciones/bpmn-ai/internal/observability"
	"github.com/Raoon-Soluciones/bpmn-ai/internal/process"
	"github.com/Raoon-Soluciones/bpmn-ai/internal/queue"
	"github.com/Raoon-Soluciones/bpmn-ai/pkg/bpmn"
	"github.com/Raoon-Soluciones/bpmn-ai/pkg/store"
)

// Config holds engine configuration.
type Config struct {
	WorkerCount      int
	MaxLoops         int
	ExecutionTimeout time.Duration
}

// Engine is the BPMN execution engine with an iterative loop.
type Engine struct {
	config   Config
	registry *ElementRegistry
	router   *FlowRouter
	store    store.Store
	logger   *observability.Logger
	queue    *queue.WorkerPool
}

// New creates a new engine.
func New(cfg Config, registry *ElementRegistry, s store.Store, logger *observability.Logger, q *queue.WorkerPool) *Engine {
	if cfg.WorkerCount < 1 {
		cfg.WorkerCount = 1
	}
	if cfg.MaxLoops < 1 {
		cfg.MaxLoops = 100
	}
	if cfg.ExecutionTimeout < 1 {
		cfg.ExecutionTimeout = 30 * time.Second
	}

	return &Engine{
		config:   cfg,
		registry: registry,
		router:   NewFlowRouter(),
		store:    s,
		logger:   logger,
		queue:    q,
	}
}

// workItem represents a unit of work for the engine.
type workItem struct {
	flow     *store.FlowRecord
	threadID int
}

// Run executes a process instance from its start event.
func (e *Engine) Run(ctx context.Context, instance *process.Instance) error {
	if instance.Process.StartEventID == "" {
		return fmt.Errorf("process %s has no start event", instance.ProcessID)
	}

	// Transition to IN_PROGRESS if not already
	if instance.State != process.StateInProgress {
		if err := instance.Transition(process.StateInProgress); err != nil {
			return fmt.Errorf("transition to in_progress: %w", err)
		}
	}

	// Create initial flow record
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

	// Create initial thread
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

	// Initialize fail-safe manager
	failsafe := NewFailSafeManager(e.config.ExecutionTimeout, e.config.MaxLoops)

	// Work queue
	workCh := make(chan workItem, 1024)
	resultCh := make(chan ExecutionResult, 1024)
	errCh := make(chan error, 1)

	// Track active work to know when to stop
	var pendingMu sync.Mutex
	var pending int = 1 // initial flow
	done := make(chan struct{})
	var doneOnce sync.Once

	// Start workers
	var wg sync.WaitGroup
	for i := 0; i < e.config.WorkerCount; i++ {
		wg.Add(1)
		go e.worker(ctx, instance, workCh, resultCh, &wg)
	}

	// Enqueue initial flow
	workCh <- workItem{flow: initialFlow, threadID: 1}

	// Signal when all work is done
	go func() {
		for {
			pendingMu.Lock()
			p := pending
			pendingMu.Unlock()
			if p == 0 {
				doneOnce.Do(func() {
					close(workCh)
					close(done)
				})
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
	}()

	// Process results
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case <-done:
			// All work is done
			wg.Wait()
			close(resultCh)
			return e.finalizeInstance(ctx, instance)

		case result, ok := <-resultCh:
			if !ok {
				return e.finalizeInstance(ctx, instance)
			}

			// Decrement pending for completed item
			pendingMu.Lock()
			pending--
			pendingMu.Unlock()

			// Check fail-safes
			if result.FlowData != nil {
				if err := failsafe.Check(result.FlowData.ElementID); err != nil {
					instance.Transition(process.StateError)
					return err
				}
			}

			// Handle result
			if err := e.handleResult(ctx, instance, result, workCh, errCh, &pendingMu, &pending); err != nil {
				return err
			}

		case err := <-errCh:
			return err
		}
	}
}

// worker processes elements from the work queue.
func (e *Engine) worker(ctx context.Context, instance *process.Instance, workCh <-chan workItem, resultCh chan<- ExecutionResult, wg *sync.WaitGroup) {
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
func (e *Engine) executeElement(ctx context.Context, instance *process.Instance, flow *store.FlowRecord, threadID int) ExecutionResult {
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

	return result
}

// handleResult processes an execution result and enqueues next flows.
func (e *Engine) handleResult(
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
			parentThreadIdx := result.FlowData.ThreadID
			for i, nf := range nextFlows {
				threadIdx := parentThreadIdx*10 + i + 1
				parentIdx := parentThreadIdx

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

				flowRecord := CreateFlowRecord(
					instance.ID,
					nf.ElementID,
					nf.ElementType,
					threadIdx,
					result.FlowData.ID,
				)
				if err := e.store.CreateFlow(ctx, flowRecord); err != nil {
					return fmt.Errorf("create flow: %w", err)
				}

				pendingMu.Lock()
				*pending++
				pendingMu.Unlock()
				select {
				case workCh <- workItem{flow: flowRecord, threadID: threadIdx}:
				case <-ctx.Done():
					return ctx.Err()
				}
			}
		} else if len(nextFlows) == 1 {
			nf := nextFlows[0]
			flowRecord := CreateFlowRecord(
				instance.ID,
				nf.ElementID,
				nf.ElementType,
				result.FlowData.ThreadID,
				result.FlowData.ID,
			)
			if err := e.store.CreateFlow(ctx, flowRecord); err != nil {
				return fmt.Errorf("create flow: %w", err)
			}

			pendingMu.Lock()
			*pending++
			pendingMu.Unlock()
			select {
			case workCh <- workItem{flow: flowRecord, threadID: nf.ThreadID}:
			case <-ctx.Done():
				return ctx.Err()
			}
		} else {
			// No next flows - check if this is an end event
			if isEndEvent(result.FlowData.ElementID, instance.Process) {
				if err := instance.Transition(process.StateCompleted); err != nil {
					e.logger.Error("failed to transition instance", "error", err)
				}
			}
		}

	case element.ActionWait, element.ActionForm:
		if err := instance.Transition(process.StateWaiting); err != nil {
			e.logger.Error("failed to suspend instance", "error", err)
		}

	case element.ActionError:
		if err := instance.Transition(process.StateError); err != nil {
			e.logger.Error("failed to transition instance to error", "error", err)
		}
		if result.Error != nil {
			errCh <- result.Error
		}

	case element.ActionComplete:
		if isEndEvent(result.FlowData.ElementID, instance.Process) {
			if err := instance.Transition(process.StateCompleted); err != nil {
				e.logger.Error("failed to complete instance", "error", err)
			}
		}

	case element.ActionTerminate:
		if err := instance.Transition(process.StateTerminated); err != nil {
			e.logger.Error("failed to terminate instance", "error", err)
		}
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
			job.ID = "job-" + job.InstanceID
			job.Status = store.JobStatusPending
			job.ScheduledAt = time.Now()
			job.MaxRetries = 3
			_ = e.store.CreateJob(ctx, job)
		}
		// Route to next flows immediately while queue is processed asynchronously
		nextFlows := e.router.Route(result, instance.Process, result.FlowData.ThreadID)
		for _, nf := range nextFlows {
			flowRecord := CreateFlowRecord(
				instance.ID,
				nf.ElementID,
				nf.ElementType,
				result.FlowData.ThreadID,
				result.FlowData.ID,
			)
			if err := e.store.CreateFlow(ctx, flowRecord); err != nil {
				return fmt.Errorf("create flow: %w", err)
			}
			pendingMu.Lock()
			*pending++
			pendingMu.Unlock()
			select {
			case workCh <- workItem{flow: flowRecord, threadID: nf.ThreadID}:
			case <-ctx.Done():
				return ctx.Err()
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
func (e *Engine) finalizeInstance(ctx context.Context, instance *process.Instance) error {
	return e.store.UpdateInstance(ctx, instance.ToRecord())
}

func isEndEvent(elementID string, proc *bpmn.Process) bool {
	elem, ok := proc.Elements[elementID]
	if !ok {
		return false
	}
	return elem.Type == bpmn.ElementTypeEndEvent || elem.Type == bpmn.ElementTypeTerminateEvent
}
