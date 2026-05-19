package process

import (
	"time"

	"github.com/google/uuid"
	"github.com/Raoon-Soluciones/bpmn-ai/pkg/bpmn"
	"github.com/Raoon-Soluciones/bpmn-ai/pkg/store"
)

// State represents the state of a process instance.
type State string

const (
	StateCreated    State = "CREATED"
	StateInProgress State = "IN_PROGRESS"
	StateWaiting    State = "WAITING"
	StateSuspended  State = "SUSPENDED"
	StateCompleted  State = "COMPLETED"
	StateError      State = "ERROR"
	StateTerminated State = "TERMINATED"
)

// ValidTransitions defines allowed state transitions.
var ValidTransitions = map[State][]State{
	StateCreated:    {StateInProgress, StateError},
	StateInProgress: {StateWaiting, StateSuspended, StateCompleted, StateError, StateTerminated},
	StateWaiting:    {StateInProgress, StateError, StateTerminated},
	StateSuspended:  {StateInProgress, StateError},
	StateError:      {StateInProgress},
	StateCompleted:  {},
	StateTerminated: {},
}

// IsValidTransition returns true if the transition from current to next is allowed.
func IsValidTransition(current, next State) bool {
	allowed, ok := ValidTransitions[current]
	if !ok {
		return false
	}
	for _, s := range allowed {
		if s == next {
			return true
		}
	}
	return false
}

// Instance represents a running process instance (case).
type Instance struct {
	ID         string
	ProcessID  string
	Process    *bpmn.Process
	Title      string
	State      State
	Variables  map[string]any
	PIN        string
	Threads    []*Thread
	StartedAt  time.Time
	UpdatedAt  time.Time
	FinishedAt *time.Time
}

// NewInstance creates a new process instance from a BPMN process definition.
func NewInstance(proc *bpmn.Process, variables map[string]any) *Instance {
	now := time.Now()
	if variables == nil {
		variables = make(map[string]any)
	}
	return &Instance{
		ID:        uuid.New().String(),
		ProcessID: proc.ID,
		Process:   proc,
		State:     StateCreated,
		Variables: variables,
		PIN:       "0000",
		Threads:   make([]*Thread, 0),
		StartedAt: now,
		UpdatedAt: now,
	}
}

// GetID returns the instance ID.
func (i *Instance) GetID() string {
	return i.ID
}

// GetState returns the current instance state.
func (i *Instance) GetState() string {
	return string(i.State)
}

// Transition attempts to change the instance state.
func (i *Instance) Transition(next State) error {
	if !IsValidTransition(i.State, next) {
		return &InvalidTransitionError{
			From: i.State,
			To:   next,
		}
	}
	i.State = next
	i.UpdatedAt = time.Now()
	if next == StateCompleted || next == StateTerminated {
		now := time.Now()
		i.FinishedAt = &now
	}
	return nil
}

// GetVariable returns a process variable by key.
func (i *Instance) GetVariable(key string) (any, bool) {
	val, ok := i.Variables[key]
	return val, ok
}

// SetVariable sets a process variable.
func (i *Instance) SetVariable(key string, value any) {
	i.Variables[key] = value
	i.UpdatedAt = time.Now()
}

// ToRecord converts the instance to a store record.
func (i *Instance) ToRecord() *store.InstanceRecord {
	return &store.InstanceRecord{
		ID:        i.ID,
		ProcessID: i.ProcessID,
		Title:     i.Title,
		Status:    store.InstanceStatus(i.State),
		Variables: i.Variables,
		PIN:       i.PIN,
		StartedAt: i.StartedAt,
		UpdatedAt: i.UpdatedAt,
		FinishedAt: i.FinishedAt,
	}
}

// Thread represents an execution thread within a process instance.
type Thread struct {
	ID            int
	InstanceID    string
	ThreadIndex   int
	ParentIndex   *int
	CurrentFlowID string
	State         string
	CreatedAt     time.Time
}

// NewThread creates a new execution thread.
func NewThread(instanceID string, threadIndex int, parentIndex *int, flowID string) *Thread {
	return &Thread{
		InstanceID:    instanceID,
		ThreadIndex:   threadIndex,
		ParentIndex:   parentIndex,
		CurrentFlowID: flowID,
		State:         "ACTIVE",
		CreatedAt:     time.Now(),
	}
}

// InvalidTransitionError is returned when a state transition is not allowed.
type InvalidTransitionError struct {
	From State
	To   State
}

func (e *InvalidTransitionError) Error() string {
	return "invalid state transition: " + string(e.From) + " -> " + string(e.To)
}
