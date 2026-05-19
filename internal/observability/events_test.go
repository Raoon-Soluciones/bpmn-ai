package observability

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestDispatcher_RegisterAndDispatch(t *testing.T) {
	d := NewDispatcher()

	var received int32
	d.Register(EventElementExecuted, func(event Event) {
		atomic.AddInt32(&received, 1)
	})

	d.Dispatch(Event{
		Type:      EventElementExecuted,
		Timestamp: time.Now(),
		Payload:   map[string]any{"element": "start-1"},
	})

	if atomic.LoadInt32(&received) != 1 {
		t.Errorf("expected 1 event received, got %d", received)
	}
}

func TestDispatcher_MultipleHandlers(t *testing.T) {
	d := NewDispatcher()

	var count1, count2 int32
	d.Register(EventElementExecuted, func(event Event) {
		atomic.AddInt32(&count1, 1)
	})
	d.Register(EventElementExecuted, func(event Event) {
		atomic.AddInt32(&count2, 1)
	})

	d.Dispatch(Event{Type: EventElementExecuted})

	if atomic.LoadInt32(&count1) != 1 {
		t.Errorf("expected handler1 to receive 1 event, got %d", count1)
	}
	if atomic.LoadInt32(&count2) != 1 {
		t.Errorf("expected handler2 to receive 1 event, got %d", count2)
	}
}

func TestDispatcher_DifferentEventTypes(t *testing.T) {
	d := NewDispatcher()

	var started, completed int32
	d.Register(EventProcessStarted, func(event Event) {
		atomic.AddInt32(&started, 1)
	})
	d.Register(EventProcessCompleted, func(event Event) {
		atomic.AddInt32(&completed, 1)
	})

	d.Dispatch(Event{Type: EventProcessStarted})
	d.Dispatch(Event{Type: EventProcessCompleted})
	d.Dispatch(Event{Type: EventProcessStarted})

	if atomic.LoadInt32(&started) != 2 {
		t.Errorf("expected 2 started events, got %d", started)
	}
	if atomic.LoadInt32(&completed) != 1 {
		t.Errorf("expected 1 completed event, got %d", completed)
	}
}

func TestDispatcher_DispatchAsync(t *testing.T) {
	d := NewDispatcher()

	var wg sync.WaitGroup
	var received int32
	d.Register(EventElementExecuted, func(event Event) {
		atomic.AddInt32(&received, 1)
		wg.Done()
	})

	wg.Add(3)
	for i := 0; i < 3; i++ {
		d.DispatchAsync(Event{Type: EventElementExecuted})
	}

	wg.Wait()

	if atomic.LoadInt32(&received) != 3 {
		t.Errorf("expected 3 async events, got %d", received)
	}
}

func TestDispatcher_NoHandlerForType(t *testing.T) {
	d := NewDispatcher()

	d.Dispatch(Event{Type: "nonexistent"})
}

func TestEventConstants(t *testing.T) {
	expected := map[string]string{
		"process.started":    EventProcessStarted,
		"process.completed":  EventProcessCompleted,
		"process.terminated": EventProcessTerminated,
		"process.error":      EventProcessError,
		"element.executed":   EventElementExecuted,
		"element.error":      EventElementError,
		"task.claimed":       EventTaskClaimed,
		"task.completed":     EventTaskCompleted,
		"job.queued":         EventJobQueued,
		"job.completed":      EventJobCompleted,
		"job.failed":         EventJobFailed,
		"job.dead":           EventJobDead,
	}

	for want, got := range expected {
		if got != want {
			t.Errorf("expected event constant %s, got %s", want, got)
		}
	}
}
