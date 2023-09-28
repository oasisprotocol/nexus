package metrics

import (
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
)

// Default service metrics for database operations.
type DatabaseMetrics struct {
	// Counts of database operations
	databaseOperations *prometheus.CounterVec

	// Latencies of database operations.
	databaseLatencies *prometheus.HistogramVec
}

// NewDefaultDatabaseMetrics creates Prometheus metric instrumentation
// for basic metrics common to database accesses. Default metrics include:
//
// 1. Counts of database operations.
// 2. Latencies for database operations.
func NewDefaultDatabaseMetrics(pkg string) DatabaseMetrics {
	metrics := DatabaseMetrics{
		databaseOperations: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: fmt.Sprintf("%s_db_operations", pkg),
				Help: "How many database operations occur, partitioned by operation and status.",
			},
			[]string{"database", "operation", "status"}, // Labels.
		),
		databaseLatencies: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name: fmt.Sprintf("%s_db_latencies", pkg),
				Help: "How long database operations take, partitioned by operation.",
			},
			[]string{"database", "operation"}, // Labels.
		),
	}
	metrics.databaseOperations = registerOnce(metrics.databaseOperations).(*prometheus.CounterVec)
	metrics.databaseLatencies = registerOnce(metrics.databaseLatencies).(*prometheus.HistogramVec)
	return metrics
}

// DatabaseOperations returns the counter for the database operation.
// The provided params are used as labels.
func (m *DatabaseMetrics) DatabaseOperations(db, operation, status string) prometheus.Counter {
	return m.databaseOperations.WithLabelValues(db, operation, status)
}

// DatabaseLatencies returns a new latency timer for the provided
// database operation.
// The provided params are used as labels.
func (m *DatabaseMetrics) DatabaseLatencies(db string, operation string) *prometheus.Timer {
	return prometheus.NewTimer(m.databaseLatencies.WithLabelValues(db, operation))
}
