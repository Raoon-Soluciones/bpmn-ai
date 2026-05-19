package observability

import (
	"sync"
	"time"
)

// Event represents a domain event emitted by the engine.
type Event struct {
	Type      string
	Timestamp time.Time
	Payload   map[string]any
}

// EventHandler processes an event.
type EventHandler func(event Event)

// Dispatcher distributes events to registered handlers.
type Dispatcher struct {
	mu       sync.RWMutex
	handlers map[string][]EventHandler
}

// NewDispatcher creates a new event dispatcher.
func NewDispatcher() *Dispatcher {
	return &Dispatcher{
		handlers: make(map[string][]EventHandler),
	}
}

// Register adds a handler for a specific event type.
func (d *Dispatcher) Register(eventType string, handler EventHandler) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.handlers[eventType] = append(d.handlers[eventType], handler)
}

// Dispatch sends an event to all registered handlers for its type.
func (d *Dispatcher) Dispatch(event Event) {
	d.mu.RLock()
	handlers := d.handlers[event.Type]
	d.mu.RUnlock()

	for _, h := range handlers {
		h(event)
	}
}

// DispatchAsync sends an event to handlers in a goroutine.
func (d *Dispatcher) DispatchAsync(event Event) {
	d.mu.RLock()
	handlers := d.handlers[event.Type]
	d.mu.RUnlock()

	for _, h := range handlers {
		go h(event)
	}
}

// Event types for the BPMN engine.
const (
	EventProcessStarted    = "process.started"
	EventProcessCompleted  = "process.completed"
	EventProcessTerminated = "process.terminated"
	EventProcessError      = "process.error"
	EventElementExecuted   = "element.executed"
	EventElementError      = "element.error"
	EventTaskClaimed       = "task.claimed"
	EventTaskCompleted     = "task.completed"
	EventJobQueued         = "job.queued"
	EventJobCompleted      = "job.completed"
	EventJobFailed         = "job.failed"
	EventJobDead           = "job.dead"
)
