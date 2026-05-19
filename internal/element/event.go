package element

import "github.com/organization/bpmn-engine/pkg/bpmn"

// Event represents a BPMN event.
type Event interface {
	Element

	// EventDefinition returns the event definition.
	EventDefinition() bpmn.EventDefinition
}
