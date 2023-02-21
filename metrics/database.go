package metrics

import (
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	// Labels to use for partitioning database operations.
	databaseOperationLabels = []string{"database", "operation", "status"}

	// Labels to use for partitioning database latencies.
	databaseLatencyLabels = []string{"database", "operation"}
)

// Default service metrics for database operations.
type DatabaseMetrics struct {
	// Counts of database operations
	DatabaseOperations *prometheus.CounterVec

	// Latencies of database operations.
	DatabaseLatencies *prometheus.HistogramVec
}

// NewDefaultDatabaseMetrics creates Prometheus metric instrumentation
// for basic metrics common to database accesses. Default metrics include:
//
// 1. Counts of database operations.
// 2. Latencies for database operations.
func NewDefaultDatabaseMetrics(pkg string) DatabaseMetrics {
	metrics := DatabaseMetrics{
		DatabaseOperations: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: fmt.Sprintf("%s_db_operations", pkg),
				Help: "How many database operations occur, partitioned by operation, status, and cause.",
			},
			databaseOperationLabels,
		),
		DatabaseLatencies: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name: fmt.Sprintf("%s_db_latencies", pkg),
				Help: "How long database operations take, partitioned by operation.",
			},
			databaseLatencyLabels,
		),
	}
	prometheus.MustRegister(metrics.DatabaseOperations)
	prometheus.MustRegister(metrics.DatabaseLatencies)
	return metrics
}

// DatabaseCounter returns the counter for the database operation.
// Provided labels should be database, operation, status, and cause.
func (m *DatabaseMetrics) DatabaseCounter(labels ...string) prometheus.Counter {
	if len(labels) > len(databaseOperationLabels) {
		labels = labels[:len(databaseOperationLabels)]
	}
	labels = append(labels, make([]string, len(databaseOperationLabels)-len(labels))...)
	return m.DatabaseOperations.WithLabelValues(labels...)
}

// DatabaseTimer returns a new latency timer for the provided
// database operation.
// Provided labels should be database and operation.
func (m *DatabaseMetrics) DatabaseTimer(labels ...string) *prometheus.Timer {
	if len(labels) > len(databaseLatencyLabels) {
		labels = labels[:len(databaseLatencyLabels)]
	}
	labels = append(labels, make([]string, len(databaseLatencyLabels)-len(labels))...)
	return prometheus.NewTimer(m.DatabaseLatencies.WithLabelValues(labels...))
}
