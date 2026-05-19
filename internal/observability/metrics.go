package observability

import (
	"net/http"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics holds all Prometheus metrics for the BPMN engine.
type Metrics struct {
	registry         *prometheus.Registry
	processesActive  prometheus.Gauge
	casesTotal       *prometheus.CounterVec
	casesByStatus    *prometheus.GaugeVec
	elementDuration  *prometheus.HistogramVec
	elementErrors    *prometheus.CounterVec
	queueDepth       prometheus.Gauge
	queueRetries     *prometheus.CounterVec
	queueDeadLetters prometheus.Counter
	requestDuration  *prometheus.HistogramVec
	requestErrors    *prometheus.CounterVec
}

var (
	metricsOnce sync.Once
	metrics     *Metrics
)

// DefaultMetrics returns the global metrics instance.
func DefaultMetrics() *Metrics {
	metricsOnce.Do(func() {
		metrics = NewMetrics()
	})
	return metrics
}

// NewMetrics creates a new metrics collector with all BPMN engine metrics.
func NewMetrics() *Metrics {
	reg := prometheus.NewRegistry()

	processesActive := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "bpmn_processes_active",
		Help: "Number of currently active process instances",
	})

	casesTotal := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "bpmn_cases_total",
		Help: "Total number of cases started",
	}, []string{"process_id"})

	casesByStatus := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "bpmn_cases_by_status",
		Help: "Number of cases by status",
	}, []string{"status"})

	elementDuration := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "bpmn_element_duration_ms",
		Help:    "Duration of element execution in milliseconds",
		Buckets: prometheus.ExponentialBuckets(1, 2, 12),
	}, []string{"element_type", "action"})

	elementErrors := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "bpmn_element_errors_total",
		Help: "Total number of element execution errors",
	}, []string{"element_type"})

	queueDepth := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "bpmn_queue_depth",
		Help: "Number of pending jobs in the queue",
	})

	queueRetries := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "bpmn_queue_retries_total",
		Help: "Total number of job retries",
	}, []string{"job_type"})

	queueDeadLetters := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "bpmn_queue_dead_letters_total",
		Help: "Total number of jobs moved to dead letter queue",
	})

	requestDuration := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "bpmn_http_request_duration_ms",
		Help:    "HTTP request duration in milliseconds",
		Buckets: prometheus.DefBuckets,
	}, []string{"method", "path", "status"})

	requestErrors := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "bpmn_http_request_errors_total",
		Help: "Total number of HTTP request errors",
	}, []string{"method", "path"})

	reg.MustRegister(
		processesActive,
		casesTotal,
		casesByStatus,
		elementDuration,
		elementErrors,
		queueDepth,
		queueRetries,
		queueDeadLetters,
		requestDuration,
		requestErrors,
	)

	return &Metrics{
		registry:         reg,
		processesActive:  processesActive,
		casesTotal:       casesTotal,
		casesByStatus:    casesByStatus,
		elementDuration:  elementDuration,
		elementErrors:    elementErrors,
		queueDepth:       queueDepth,
		queueRetries:     queueRetries,
		queueDeadLetters: queueDeadLetters,
		requestDuration:  requestDuration,
		requestErrors:    requestErrors,
	}
}

// Handler returns an HTTP handler that serves Prometheus metrics.
func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{})
}

// Registry returns the underlying Prometheus registry.
func (m *Metrics) Registry() *prometheus.Registry {
	return m.registry
}

func (m *Metrics) SetActiveProcesses(count int) {
	m.processesActive.Set(float64(count))
}

func (m *Metrics) IncCaseStarted(processID string) {
	m.casesTotal.WithLabelValues(processID).Inc()
}

func (m *Metrics) SetCasesByStatus(status string, count int) {
	m.casesByStatus.WithLabelValues(status).Set(float64(count))
}

func (m *Metrics) ObserveElementDuration(elementType, action string, durationMs float64) {
	m.elementDuration.WithLabelValues(elementType, action).Observe(durationMs)
}

func (m *Metrics) IncElementErrors(elementType string) {
	m.elementErrors.WithLabelValues(elementType).Inc()
}

func (m *Metrics) SetQueueDepth(depth int) {
	m.queueDepth.Set(float64(depth))
}

func (m *Metrics) IncQueueRetries(jobType string) {
	m.queueRetries.WithLabelValues(jobType).Inc()
}

func (m *Metrics) IncDeadLetters() {
	m.queueDeadLetters.Inc()
}

func (m *Metrics) ObserveRequestDuration(method, path string, status int, duration time.Duration) {
	m.requestDuration.WithLabelValues(method, path, httpStatusText(status)).Observe(duration.Seconds() * 1000)
}

func (m *Metrics) IncRequestErrors(method, path string) {
	m.requestErrors.WithLabelValues(method, path).Inc()
}

func httpStatusText(code int) string {
	if code < 400 {
		return "2xx"
	}
	if code < 500 {
		return "4xx"
	}
	return "5xx"
}
