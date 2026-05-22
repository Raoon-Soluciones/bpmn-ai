package observability

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestFileAuditWriter_WriteProcessStarted(t *testing.T) {
	dir := t.TempDir()

	logger, _ := NewFromConfig("error", "text")
	w, err := NewFileAuditWriter(dir, true, logger)
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}
	defer w.Close()

	w.HandleEvent(Event{
		Type:      EventProcessStarted,
		Timestamp: time.Now(),
		Payload: map[string]any{
			"instance_id":  "inst-1",
			"process_id":   "proc-1",
			"process_name": "Test Process",
		},
	})

	path := filepath.Join(dir, "audit_inst-1.log")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read audit file: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "BPMN Execution Audit") {
		t.Error("expected header in audit file")
	}
	if !strings.Contains(content, "Test Process") {
		t.Error("expected process name in audit file")
	}
	if !strings.Contains(content, "proc-1") {
		t.Error("expected process id in audit file")
	}
}

func TestFileAuditWriter_WriteElementExecuted(t *testing.T) {
	dir := t.TempDir()

	logger, _ := NewFromConfig("error", "text")
	w, err := NewFileAuditWriter(dir, true, logger)
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}
	defer w.Close()

	w.HandleEvent(Event{
		Type: EventElementExecuted,
		Payload: map[string]any{
			"instance_id":  "inst-1",
			"process_id":   "proc-1",
			"element_id":   "elem-1",
			"element_name": "Start Event",
			"element_type": "startEvent",
			"action":       "ROUTE",
			"thread_id":    1,
			"duration_ms":  2,
		},
	})

	path := filepath.Join(dir, "audit_inst-1.log")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read audit file: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "elem-1") {
		t.Error("expected element id in audit file")
	}
	if !strings.Contains(content, "Start Event") {
		t.Error("expected element name in audit file")
	}
	if !strings.Contains(content, "startEvent") {
		t.Error("expected element type in audit file")
	}
	if !strings.Contains(content, "ROUTE") {
		t.Error("expected action in audit file")
	}
	if !strings.Contains(content, "2ms") {
		t.Error("expected duration in audit file")
	}
}

func TestFileAuditWriter_ElementError(t *testing.T) {
	dir := t.TempDir()

	logger, _ := NewFromConfig("error", "text")
	w, err := NewFileAuditWriter(dir, true, logger)
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}
	defer w.Close()

	w.HandleEvent(Event{
		Type: EventElementError,
		Payload: map[string]any{
			"instance_id":  "inst-1",
			"element_id":   "task-1",
			"element_name": "Service Call",
			"element_type": "serviceTask",
			"thread_id":    1,
			"error":        "connection refused",
		},
	})

	path := filepath.Join(dir, "audit_inst-1.log")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read audit file: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "ERROR") {
		t.Error("expected ERROR marker in audit file")
	}
	if !strings.Contains(content, "connection refused") {
		t.Error("expected error message in audit file")
	}
}

func TestFileAuditWriter_Disabled(t *testing.T) {
	dir := t.TempDir()

	logger, _ := NewFromConfig("error", "text")
	w, err := NewFileAuditWriter(dir, false, logger)
	if err != nil {
		t.Fatalf("failed to create disabled writer: %v", err)
	}
	defer w.Close()

	w.HandleEvent(Event{
		Type: EventElementExecuted,
		Payload: map[string]any{
			"instance_id":  "inst-1",
			"element_id":   "elem-1",
			"element_type": "startEvent",
		},
	})

	path := filepath.Join(dir, "audit_inst-1.log")
	if _, err := os.Stat(path); err == nil {
		t.Error("expected no audit file when disabled")
	}
}

func TestFileAuditWriter_ConcurrentWrites(t *testing.T) {
	dir := t.TempDir()

	logger, _ := NewFromConfig("error", "text")
	w, err := NewFileAuditWriter(dir, true, logger)
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}
	defer w.Close()

	var wg sync.WaitGroup
	n := 20

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			w.HandleEvent(Event{
				Type: EventElementExecuted,
				Payload: map[string]any{
					"instance_id":  "inst-1",
					"element_id":   "elem-1",
					"element_type": "startEvent",
					"action":       "ROUTE",
					"thread_id":    i,
				},
			})
		}(i)
	}

	wg.Wait()

	path := filepath.Join(dir, "audit_inst-1.log")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read audit file: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "Thread") {
		t.Error("expected concurrent element entries in audit file")
	}
	if len(content) < 50 {
		t.Error("expected substantial content from concurrent writes")
	}
}

func TestFileAuditWriter_ProcessCompleted(t *testing.T) {
	dir := t.TempDir()

	logger, _ := NewFromConfig("error", "text")
	w, err := NewFileAuditWriter(dir, true, logger)
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}
	defer w.Close()

	w.HandleEvent(Event{
		Type: EventProcessStarted,
		Payload: map[string]any{
			"instance_id":  "inst-1",
			"process_id":   "proc-1",
			"process_name": "Test",
		},
	})

	w.HandleEvent(Event{
		Type: EventElementExecuted,
		Payload: map[string]any{
			"instance_id":  "inst-1",
			"element_id":   "start-1",
			"element_type": "startEvent",
			"action":       "ROUTE",
			"thread_id":    1,
		},
	})

	w.HandleEvent(Event{
		Type: EventProcessCompleted,
		Payload: map[string]any{
			"instance_id": "inst-1",
		},
	})

	path := filepath.Join(dir, "audit_inst-1.log")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read audit file: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "COMPLETED") {
		t.Error("expected COMPLETED in audit footer")
	}
	if !strings.Contains(content, "Elements: 1") {
		t.Error("expected element count in audit footer")
	}
}

func TestFileAuditWriter_ProcessTerminated(t *testing.T) {
	dir := t.TempDir()

	logger, _ := NewFromConfig("error", "text")
	w, err := NewFileAuditWriter(dir, true, logger)
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}
	defer w.Close()

	w.HandleEvent(Event{
		Type: EventProcessStarted,
		Payload: map[string]any{
			"instance_id":  "inst-1",
			"process_id":   "proc-1",
			"process_name": "Test",
		},
	})

	w.HandleEvent(Event{
		Type: EventProcessTerminated,
		Payload: map[string]any{
			"instance_id": "inst-1",
			"element_id":  "task-1",
		},
	})

	path := filepath.Join(dir, "audit_inst-1.log")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read audit file: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "TERMINATED") {
		t.Error("expected TERMINATED in audit footer")
	}
}

func TestFileAuditWriter_DifferentInstances(t *testing.T) {
	dir := t.TempDir()

	logger, _ := NewFromConfig("error", "text")
	w, err := NewFileAuditWriter(dir, true, logger)
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}
	defer w.Close()

	w.HandleEvent(Event{
		Type: EventProcessStarted,
		Payload: map[string]any{
			"instance_id": "inst-a",
			"process_id":  "proc-1",
		},
	})
	w.HandleEvent(Event{
		Type: EventProcessStarted,
		Payload: map[string]any{
			"instance_id": "inst-b",
			"process_id":  "proc-2",
		},
	})

	pathA := filepath.Join(dir, "audit_inst-a.log")
	pathB := filepath.Join(dir, "audit_inst-b.log")

	if _, err := os.Stat(pathA); err != nil {
		t.Error("expected audit file for inst-a")
	}
	if _, err := os.Stat(pathB); err != nil {
		t.Error("expected audit file for inst-b")
	}
}

func TestExtractString(t *testing.T) {
	payload := map[string]any{
		"key1": "value1",
		"key2": 42,
	}

	if v := extractString(payload, "key1"); v != "value1" {
		t.Errorf("expected value1, got %s", v)
	}
	if v := extractString(payload, "missing"); v != "" {
		t.Errorf("expected empty, got %s", v)
	}
	if v := extractString(payload, "key2"); v != "" {
		t.Errorf("expected empty for non-string, got %s", v)
	}
	if v := extractString(nil, "key1"); v != "" {
		t.Errorf("expected empty for nil payload, got %s", v)
	}
}

func TestExtractInt(t *testing.T) {
	payload := map[string]any{
		"count": 42,
		"price": 3.14,
	}

	if v := extractInt(payload, "count"); v != 42 {
		t.Errorf("expected 42, got %d", v)
	}
	if v := extractInt(payload, "missing"); v != 0 {
		t.Errorf("expected 0, got %d", v)
	}
	if v := extractInt(nil, "count"); v != 0 {
		t.Errorf("expected 0 for nil payload, got %v", v)
	}
	_ = extractInt(payload, "price")
}

func TestAuditor_RegistersHandlers(t *testing.T) {
	dispatcher := NewDispatcher()
	logger, _ := NewFromConfig("error", "text")
	writer, _ := NewFileAuditWriter(t.TempDir(), true, logger)

	auditor := NewAuditor(dispatcher, writer)
	if auditor == nil {
		t.Fatal("expected non-nil auditor")
	}

	if auditor.Dispatcher() != dispatcher {
		t.Error("expected same dispatcher reference")
	}
}
