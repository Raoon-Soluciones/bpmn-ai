package observability

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestFileAuditWriter_WriteAndReadEntry(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.jsonl")

	logger, _ := NewFromConfig("error", "text")
	w, err := NewFileAuditWriter(path, true, logger)
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}
	defer w.Close()

	entry := &AuditEntry{
		InstanceID:  "inst-1",
		ProcessID:   "proc-1",
		ElementID:   "elem-1",
		ElementType: "startEvent",
		Action:      "ROUTE",
		EventType:   "element.executed",
		ThreadID:    1,
		FromState:   "CREATED",
		ToState:     "IN_PROGRESS",
	}

	if err := w.WriteEntry(context.Background(), entry); err != nil {
		t.Fatalf("failed to write entry: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}

	var decoded AuditEntry
	if err := json.Unmarshal([]byte(lines[0]), &decoded); err != nil {
		t.Fatalf("failed to decode JSON line: %v", err)
	}

	if decoded.InstanceID != "inst-1" {
		t.Errorf("expected InstanceID inst-1, got %s", decoded.InstanceID)
	}
	if decoded.EventType != "element.executed" {
		t.Errorf("expected EventType element.executed, got %s", decoded.EventType)
	}
	if decoded.ThreadID != 1 {
		t.Errorf("expected ThreadID 1, got %d", decoded.ThreadID)
	}
}

func TestFileAuditWriter_Disabled(t *testing.T) {
	logger, _ := NewFromConfig("error", "text")
	w, err := NewFileAuditWriter("", false, logger)
	if err != nil {
		t.Fatalf("failed to create disabled writer: %v", err)
	}
	defer w.Close()

	if w.file != nil {
		t.Error("expected nil file when disabled")
	}

	entry := &AuditEntry{InstanceID: "test"}
	if err := w.WriteEntry(context.Background(), entry); err != nil {
		t.Fatalf("write on disabled writer should not error: %v", err)
	}
}

func TestFileAuditWriter_ConcurrentWrites(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.jsonl")

	logger, _ := NewFromConfig("error", "text")
	w, err := NewFileAuditWriter(path, true, logger)
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}
	defer w.Close()

	var wg sync.WaitGroup
	n := 50

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			entry := &AuditEntry{
				InstanceID: "inst-1",
				ThreadID:   i,
			}
			_ = w.WriteEntry(context.Background(), entry)
		}(i)
	}

	wg.Wait()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != n {
		t.Errorf("expected %d lines, got %d", n, len(lines))
	}
}

func TestFileAuditWriter_HandleEvent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.jsonl")

	logger, _ := NewFromConfig("error", "text")
	w, err := NewFileAuditWriter(path, true, logger)
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}
	defer w.Close()

	event := Event{
		Type:      EventElementExecuted,
		Timestamp: time.Now(),
		Payload: map[string]any{
			"instance_id":  "inst-1",
			"process_id":   "proc-1",
			"element_id":   "elem-1",
			"element_type": "scriptTask",
			"action":       "ROUTE",
			"thread_id":    1,
		},
	}

	w.HandleEvent(event)

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}

	var decoded AuditEntry
	json.Unmarshal(data, &decoded)

	if decoded.InstanceID != "inst-1" {
		t.Errorf("expected InstanceID inst-1, got %s", decoded.InstanceID)
	}
	if decoded.EventType != EventElementExecuted {
		t.Errorf("expected %s, got %s", EventElementExecuted, decoded.EventType)
	}
}

func TestFileAuditWriter_HandleErrorEvent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.jsonl")

	logger, _ := NewFromConfig("error", "text")
	w, err := NewFileAuditWriter(path, true, logger)
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}
	defer w.Close()

	event := Event{
		Type:      EventElementError,
		Timestamp: time.Now(),
		Payload: map[string]any{
			"instance_id":  "inst-1",
			"process_id":   "proc-1",
			"element_id":   "elem-1",
			"element_type": "serviceTask",
			"error":        "connection refused",
		},
	}

	w.HandleEvent(event)

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}

	var decoded AuditEntry
	json.Unmarshal(data, &decoded)

	if decoded.ErrorMessage != "connection refused" {
		t.Errorf("expected 'connection refused', got '%s'", decoded.ErrorMessage)
	}
}

func TestAuditor_RegistersHandlers(t *testing.T) {
	dispatcher := NewDispatcher()
	logger, _ := NewFromConfig("error", "text")
	writer, _ := NewFileAuditWriter("", false, logger)

	auditor := NewAuditor(dispatcher, writer)
	if auditor == nil {
		t.Fatal("expected non-nil auditor")
	}

	// Verify dispatcher is accessible
	if auditor.Dispatcher() != dispatcher {
		t.Error("expected same dispatcher reference")
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
		t.Errorf("expected 0 for nil payload, got %d", v)
	}
	_ = extractInt(payload, "price") // float64 case, just verify no panic
}
