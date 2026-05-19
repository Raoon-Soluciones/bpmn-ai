package observability

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewMetrics(t *testing.T) {
	m := NewMetrics()

	if m == nil {
		t.Fatal("expected non-nil metrics")
	}
	if m.Registry() == nil {
		t.Error("expected non-nil registry")
	}
}

func TestDefaultMetrics(t *testing.T) {
	m1 := DefaultMetrics()
	m2 := DefaultMetrics()

	if m1 != m2 {
		t.Error("expected same singleton instance")
	}
}

func TestMetrics_SetActiveProcesses(t *testing.T) {
	m := NewMetrics()

	m.SetActiveProcesses(5)
	m.SetActiveProcesses(10)
}

func TestMetrics_IncCaseStarted(t *testing.T) {
	m := NewMetrics()

	m.IncCaseStarted("proc-1")
	m.IncCaseStarted("proc-1")
	m.IncCaseStarted("proc-2")
}

func TestMetrics_SetCasesByStatus(t *testing.T) {
	m := NewMetrics()

	m.SetCasesByStatus("IN_PROGRESS", 5)
	m.SetCasesByStatus("COMPLETED", 10)
	m.SetCasesByStatus("WAITING", 2)
}

func TestMetrics_ObserveElementDuration(t *testing.T) {
	m := NewMetrics()

	m.ObserveElementDuration("userTask", "FORM", 150.0)
	m.ObserveElementDuration("serviceTask", "QUEUE", 50.0)
	m.ObserveElementDuration("startEvent", "ROUTE", 1.0)
}

func TestMetrics_IncElementErrors(t *testing.T) {
	m := NewMetrics()

	m.IncElementErrors("userTask")
	m.IncElementErrors("serviceTask")
}

func TestMetrics_SetQueueDepth(t *testing.T) {
	m := NewMetrics()

	m.SetQueueDepth(5)
	m.SetQueueDepth(0)
	m.SetQueueDepth(100)
}

func TestMetrics_IncQueueRetries(t *testing.T) {
	m := NewMetrics()

	m.IncQueueRetries("serviceTask")
	m.IncQueueRetries("scriptTask")
}

func TestMetrics_IncDeadLetters(t *testing.T) {
	m := NewMetrics()

	m.IncDeadLetters()
	m.IncDeadLetters()
}

func TestMetrics_ObserveRequestDuration(t *testing.T) {
	m := NewMetrics()

	m.ObserveRequestDuration("GET", "/health", 200, 10*time.Millisecond)
	m.ObserveRequestDuration("POST", "/api/v1/processes", 201, 50*time.Millisecond)
	m.ObserveRequestDuration("GET", "/api/v1/cases", 404, 5*time.Millisecond)
	m.ObserveRequestDuration("POST", "/api/v1/tasks", 500, 100*time.Millisecond)
}

func TestMetrics_IncRequestErrors(t *testing.T) {
	m := NewMetrics()

	m.IncRequestErrors("GET", "/api/v1/cases")
	m.IncRequestErrors("POST", "/api/v1/processes")
}

func TestMetrics_Handler(t *testing.T) {
	m := NewMetrics()

	handler := m.Handler()
	if handler == nil {
		t.Fatal("expected non-nil handler")
	}

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestHttpStatusText(t *testing.T) {
	tests := []struct {
		code int
		want string
	}{
		{200, "2xx"},
		{201, "2xx"},
		{400, "4xx"},
		{404, "4xx"},
		{500, "5xx"},
		{503, "5xx"},
	}

	for _, tt := range tests {
		got := httpStatusText(tt.code)
		if got != tt.want {
			t.Errorf("httpStatusText(%d) = %s, want %s", tt.code, got, tt.want)
		}
	}
}
