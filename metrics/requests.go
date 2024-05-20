package metrics

import (
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
)

// Default service metrics for requests.
type RequestMetrics struct {
	// Counts of requests made to each service endpoint.
	requestCounts *prometheus.CounterVec

	// Latencies of serving incoming requests.
	requestLatencies *prometheus.HistogramVec
}

// NewDefaultRequestMetrics creates Prometheus metric instrumentation for
// basic metrics common to serving requests. Default metrics include:
//
// 1. Counts of service endpoints hit.
// 2. Latencies for requests.
func NewDefaultRequestMetrics(pkg string) RequestMetrics {
	metrics := RequestMetrics{
		requestCounts: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: fmt.Sprintf("%s_requests", pkg),
				Help: "How many service requests were made, partitioned by request endpoint and status.",
			},
			[]string{"endpoint", "status"}, // Labels.
		),
		requestLatencies: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name: fmt.Sprintf("%s_request_latencies", pkg),
				Help: "How long requests take to process, partitioned by request endpoint.",
			},
			[]string{"endpoint"}, // Labels.
		),
	}
	prometheus.MustRegister(metrics.requestCounts)
	prometheus.MustRegister(metrics.requestLatencies)
	return metrics
}

// RequestCounts returns the counter for the calling request.
// Provided labels should be endpoint, status, and cause.
func (m *RequestMetrics) RequestCounts(endpoint, status string) prometheus.Counter {
	return m.requestCounts.WithLabelValues(endpoint, status)
}

// RequestLatencies fetches the Histogram Observer for the given endpoint
// If it doesn't exist, a new one is created.
// This creates 12 (with DefBuckets) empty tables in the prometheus database.
func (m *RequestMetrics) RequestLatencies(endpoint string) prometheus.Observer {
	return m.requestLatencies.WithLabelValues(endpoint)
}
