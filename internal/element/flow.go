package element

// Flow represents a BPMN sequence flow.
type Flow interface {
	Element

	// SourceRef returns the source element ID.
	SourceRef() string

	// TargetRef returns the target element ID.
	TargetRef() string

	// Condition returns the condition expression (if any).
	Condition() string

	// IsDefault returns true if this is the default flow.
	IsDefault() bool
}
