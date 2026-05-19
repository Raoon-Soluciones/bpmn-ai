package observability

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

func TestNew_JSON(t *testing.T) {
	var buf bytes.Buffer
	l := New(0, "json", &buf)
	l.Info("test message", "key", "value")

	output := buf.String()
	if !strings.Contains(output, "test message") {
		t.Errorf("expected output to contain 'test message', got %s", output)
	}
	if !strings.Contains(output, "key") {
		t.Errorf("expected output to contain 'key', got %s", output)
	}
	if !strings.Contains(output, "value") {
		t.Errorf("expected output to contain 'value', got %s", output)
	}
}

func TestNew_Text(t *testing.T) {
	var buf bytes.Buffer
	l := New(0, "text", &buf)
	l.Info("test message", "key", "value")

	output := buf.String()
	if !strings.Contains(output, "test message") {
		t.Errorf("expected output to contain 'test message', got %s", output)
	}
}

func TestNewFromConfig(t *testing.T) {
	tests := []struct {
		level  string
		format string
	}{
		{"debug", "json"},
		{"info", "text"},
		{"warn", "json"},
		{"error", "text"},
		{"invalid", "invalid"}, // should default to info
	}

	for _, tt := range tests {
		t.Run(tt.level+"_"+tt.format, func(t *testing.T) {
			l, err := NewFromConfig(tt.level, tt.format)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if l == nil {
				t.Fatal("expected logger, got nil")
			}
		})
	}
}

func TestLogger_With(t *testing.T) {
	var buf bytes.Buffer
	l := New(0, "json", &buf)
	child := l.With("component", "engine")
	child.Info("test", "key", "val")

	output := buf.String()
	if !strings.Contains(output, "component") {
		t.Errorf("expected output to contain 'component', got %s", output)
	}
	if !strings.Contains(output, "engine") {
		t.Errorf("expected output to contain 'engine', got %s", output)
	}
}

func TestLogger_Error(t *testing.T) {
	var buf bytes.Buffer
	l := New(0, "json", &buf)
	l.Error("error message", "err", "something failed")

	output := buf.String()
	if !strings.Contains(output, "error message") {
		t.Errorf("expected output to contain 'error message', got %s", output)
	}
}

func TestLogger_Debug(t *testing.T) {
	var buf bytes.Buffer
	l := New(slog.LevelDebug, "json", &buf)
	l.Debug("debug message")

	output := buf.String()
	if !strings.Contains(output, "debug message") {
		t.Errorf("expected output to contain 'debug message', got %s", output)
	}
}
