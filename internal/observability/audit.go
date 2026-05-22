package observability

import "time"

type AuditEntry struct {
	ID           string         `json:"id"`
	InstanceID   string         `json:"instance_id"`
	ProcessID    string         `json:"process_id"`
	ElementID    string         `json:"element_id,omitempty"`
	ElementName  string         `json:"element_name,omitempty"`
	ElementType  string         `json:"element_type,omitempty"`
	Action       string         `json:"action,omitempty"`
	EventType    string         `json:"event_type"`
	Timestamp    time.Time      `json:"timestamp"`
	ThreadID     int            `json:"thread_id,omitempty"`
	ParentIndex  *int           `json:"parent_index,omitempty"`
	FromState    string         `json:"from_state,omitempty"`
	ToState      string         `json:"to_state,omitempty"`
	ErrorMessage string         `json:"error_message,omitempty"`
	Payload      map[string]any `json:"payload,omitempty"`
}

type AuditConfig struct {
	Enabled bool
	Dir     string
}
