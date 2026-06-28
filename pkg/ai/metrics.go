package ai

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	AICallsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "bpmn",
		Subsystem: "ai",
		Name:      "calls_total",
		Help:      "Total number of AI calls.",
	}, []string{"model", "provider", "success"})

	AITokensTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "bpmn",
		Subsystem: "ai",
		Name:      "tokens_total",
		Help:      "Total tokens processed by AI.",
	}, []string{"model", "type"})

	AILatencyMs = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "bpmn",
		Subsystem: "ai",
		Name:      "latency_ms",
		Help:      "AI call latency in milliseconds.",
		Buckets:   []float64{50, 100, 200, 500, 1000, 2000, 5000, 10000, 30000},
	}, []string{"model"})

	AICostUSD = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "bpmn",
		Subsystem: "ai",
		Name:      "cost_usd_total",
		Help:      "Total estimated cost of AI calls in USD.",
	}, []string{"model"})
)

type MetricsRecorder struct{}

func NewMetricsRecorder() *MetricsRecorder {
	return &MetricsRecorder{}
}

func (m *MetricsRecorder) Record(model string, provider string, success bool, tokensIn, tokensOut, durationMs int, costUSD float64) {
	successStr := "false"
	if success {
		successStr = "true"
	}

	AICallsTotal.WithLabelValues(model, provider, successStr).Inc()

	AITokensTotal.WithLabelValues(model, "input").Add(float64(tokensIn))
	AITokensTotal.WithLabelValues(model, "output").Add(float64(tokensOut))

	AILatencyMs.WithLabelValues(model).Observe(float64(durationMs))

	if costUSD > 0 {
		AICostUSD.WithLabelValues(model).Add(costUSD)
	}
}
