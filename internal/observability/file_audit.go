package observability

import (
	"context"
	"encoding/json"
	"os"
	"sync"
	"time"

	"github.com/google/uuid"
)

type FileAuditWriter struct {
	file    *os.File
	mu      sync.Mutex
	logger  *Logger
	enabled bool
}

func NewFileAuditWriter(path string, enabled bool, logger *Logger) (*FileAuditWriter, error) {
	if !enabled {
		return &FileAuditWriter{enabled: false, logger: logger}, nil
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}

	return &FileAuditWriter{
		file:    f,
		enabled: true,
		logger:  logger,
	}, nil
}

func (w *FileAuditWriter) WriteEntry(ctx context.Context, entry *AuditEntry) error {
	if !w.enabled || w.file == nil {
		return nil
	}

	entry.ID = uuid.New().String()
	entry.Timestamp = time.Now()

	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	if _, err := w.file.Write(data); err != nil {
		return err
	}
	if _, err := w.file.Write([]byte("\n")); err != nil {
		return err
	}
	return nil
}

func (w *FileAuditWriter) HandleEvent(event Event) {
	entry := &AuditEntry{
		InstanceID:  extractString(event.Payload, "instance_id"),
		ProcessID:   extractString(event.Payload, "process_id"),
		ElementID:   extractString(event.Payload, "element_id"),
		ElementType: extractString(event.Payload, "element_type"),
		Action:      extractString(event.Payload, "action"),
		EventType:   event.Type,
		ThreadID:    extractInt(event.Payload, "thread_id"),
		FromState:   extractString(event.Payload, "from_state"),
		ToState:     extractString(event.Payload, "to_state"),
		ErrorMessage: extractString(event.Payload, "error"),
		Payload:     event.Payload,
	}

	if pi, ok := event.Payload["parent_index"]; ok {
		if v, ok := pi.(int); ok {
			entry.ParentIndex = &v
		}
	}

	if err := w.WriteEntry(context.Background(), entry); err != nil {
		if w.logger != nil {
			w.logger.Error("failed to write audit entry", "error", err, "event_type", event.Type)
		}
	}
}

func (w *FileAuditWriter) Close() error {
	if w.file != nil {
		return w.file.Close()
	}
	return nil
}

func extractString(payload map[string]any, key string) string {
	if payload == nil {
		return ""
	}
	v, ok := payload[key]
	if !ok {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return s
}

func extractInt(payload map[string]any, key string) int {
	if payload == nil {
		return 0
	}
	v, ok := payload[key]
	if !ok {
		return 0
	}
	switch n := v.(type) {
	case int:
		return n
	case float64:
		return int(n)
	default:
		return 0
	}
}
