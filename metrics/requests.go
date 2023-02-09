package metrics

import (
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	// Labels to use for partitioning requests.
	requestLabels = []string{"endpoint", "status", "cause"}

	// Labels to use for partitioning request latencies.
	requestLatencyLabels = []string{"endpoint"}
)

// Default service metrics for requests.
type RequestMetrics struct {
	// Counts of requests made to each service endpoint.
	RequestCounts *prometheus.CounterVec

	// Latencies of serving incoming requests.
	RequestLatencies *prometheus.HistogramVec
}

// NewDefaultRequestMetrics creates Prometheus metric instrumentation for
// basic metrics common to serving requests. Default metrics include:
//
// 1. Counts of service endpoints hit.
// 2. Latencies for requests.
func NewDefaultRequestMetrics(pkg string) RequestMetrics {
	metrics := RequestMetrics{
		RequestCounts: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: fmt.Sprintf("%s_requests", pkg),
				Help: "How many service requests were made, partitioned by request endpoint, status, and cause.",
			},
			requestLabels,
		),
		RequestLatencies: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name: fmt.Sprintf("%s_request_latencies", pkg),
				Help: "How long requests take to process, partitioned by request endpoint.",
			},
			requestLatencyLabels,
		),
	}
	prometheus.MustRegister(metrics.RequestCounts)
	prometheus.MustRegister(metrics.RequestLatencies)
	return metrics
}

// RequestCounter returns the counter for the calling request.
// Provided labels should be endpoint, status, and cause.
func (m *RequestMetrics) RequestCounter(labels ...string) prometheus.Counter {
	if len(labels) > len(requestLabels) {
		labels = labels[:len(requestLabels)]
	}
	labels = append(labels, make([]string, len(requestLabels)-len(labels))...)
	return m.RequestCounts.WithLabelValues(labels...)
}

// RequestTimer creates a new latency timer for the provided request endpoint.
func (m *RequestMetrics) RequestTimer(labels ...string) *prometheus.Timer {
	if len(labels) > len(requestLatencyLabels) {
		labels = labels[:len(requestLatencyLabels)]
	}
	labels = append(labels, make([]string, len(requestLatencyLabels)-len(labels))...)
	return prometheus.NewTimer(m.RequestLatencies.WithLabelValues(labels...))
}
