package bpmn

import "time"

// ElementType represents the type of a BPMN element.
type ElementType string

const (
	ElementTypeStartEvent      ElementType = "startEvent"
	ElementTypeEndEvent        ElementType = "endEvent"
	ElementTypeTerminateEvent  ElementType = "terminateEvent"
	ElementTypeTimerEvent      ElementType = "timerEvent"
	ElementTypeMessageThrow    ElementType = "messageThrow"
	ElementTypeMessageCatch    ElementType = "messageCatch"
	ElementTypeExclusiveGateway ElementType = "exclusiveGateway"
	ElementTypeParallelGateway  ElementType = "parallelGateway"
	ElementTypeInclusiveGateway ElementType = "inclusiveGateway"
	ElementTypeEventBasedGateway ElementType = "eventBasedGateway"
	ElementTypeUserTask        ElementType = "userTask"
	ElementTypeScriptTask      ElementType = "scriptTask"
	ElementTypeServiceTask     ElementType = "serviceTask"
	ElementTypeSequenceFlow    ElementType = "sequenceFlow"
)

// Process represents a parsed BPMN 2.0 process definition.
type Process struct {
	ID              string
	Name            string
	Version         int
	StartEventID    string
	Elements        map[string]Element
	Flows           map[string]Flow
	TargetNamespace string
	CreatedAt       time.Time
}

// Element represents any BPMN element (event, gateway, activity).
type Element struct {
	ID            string
	Name          string
	Type          ElementType
	IncomingFlows []string
	OutgoingFlows []string
	Incoming      []string // raw incoming element IDs from XML (used for wiring)
	Outgoing      []string // raw outgoing element IDs from XML (used for wiring)

	// Event-specific fields
	EventDefinition EventDefinition

	// Gateway-specific fields
	GatewayType      GatewayType
	GatewayDirection GatewayDirection
	DefaultFlowID    string
	Conditions       map[string]string // flowID -> expression

	// Activity-specific fields
	TaskType       TaskType
	Assignee       string
	CandidateUsers []string
	CandidateGroups []string
	Duration       string // ISO 8601 duration for timer tasks

	// Extension attributes
	ExtensionData map[string]string
}

// Flow represents a sequence flow between two elements.
type Flow struct {
	ID           string
	Name         string
	SourceRef    string
	TargetRef    string
	Condition    string // expression for conditional flows
	IsDefault    bool
}

// EventDefinition holds event-specific configuration.
type EventDefinition struct {
	Type       EventType
	TimerType  TimerType // cron, duration, date
	TimerValue string    // cron expression, ISO 8601 duration, or date
	MessageRef string    // message reference for message events
}

// EventType represents the type of BPMN event.
type EventType string

const (
	EventTypeNone      EventType = "none"
	EventTypeTimer     EventType = "timer"
	EventTypeMessage   EventType = "message"
	EventTypeTerminate EventType = "terminate"
)

// TimerType represents how a timer is defined.
type TimerType string

const (
	TimerTypeDuration TimerType = "duration" // ISO 8601 duration
	TimerTypeDate     TimerType = "date"     // ISO 8601 date
	TimerTypeCycle    TimerType = "cycle"    // cron expression
)

// GatewayType represents the type of BPMN gateway.
type GatewayType string

const (
	GatewayTypeExclusive  GatewayType = "exclusive"
	GatewayTypeParallel   GatewayType = "parallel"
	GatewayTypeInclusive  GatewayType = "inclusive"
	GatewayTypeEventBased GatewayType = "eventBased"
)

// GatewayDirection represents the direction of a gateway.
type GatewayDirection string

const (
	GatewayDirectionDiverging   GatewayDirection = "diverging"
	GatewayDirectionConverging  GatewayDirection = "converging"
	GatewayDirectionMixed       GatewayDirection = "mixed"
)

// TaskType represents the type of BPMN task.
type TaskType string

const (
	TaskTypeUser    TaskType = "user"
	TaskTypeScript  TaskType = "script"
	TaskTypeService TaskType = "service"
)

// ScriptType represents the type of script task.
type ScriptType string

const (
	ScriptTypeBusinessRule  ScriptType = "business_rule"
	ScriptTypeChangeField   ScriptType = "change_field"
	ScriptTypeAssignTeam    ScriptType = "assign_team"
	ScriptTypeAssignUser    ScriptType = "assign_user"
	ScriptTypeAddRelated    ScriptType = "add_related"
)
