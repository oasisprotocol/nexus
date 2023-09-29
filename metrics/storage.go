package metrics

import (
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
)

// Default service metrics for database operations.
type StorageMetrics struct {
	// Counts of database operations
	databaseOperations *prometheus.CounterVec

	// Latencies of database operations.
	databaseLatencies *prometheus.HistogramVec

	// Cache hit rates for the local cache.
	localCacheReads *prometheus.CounterVec
}

type CacheReadStatus string

const (
	CacheReadStatusHit     CacheReadStatus = "hit"
	CacheReadStatusMiss    CacheReadStatus = "miss"
	CacheReadStatusBadData CacheReadStatus = "bad_data"
)

// NewDefaultStorageMetrics creates Prometheus metric instrumentation
// for basic metrics common to storage accesses.
func NewDefaultStorageMetrics(pkg string) StorageMetrics {
	metrics := StorageMetrics{
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
		localCacheReads: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: fmt.Sprintf("%s_cache_reads", pkg),
				Help: "How many local cache reads occur, partitioned by status (hit, miss, bad_data).",
			},
			[]string{"cache", "status"}, // Labels.
		),
	}
	metrics.databaseOperations = registerOnce(metrics.databaseOperations).(*prometheus.CounterVec)
	metrics.databaseLatencies = registerOnce(metrics.databaseLatencies).(*prometheus.HistogramVec)
	metrics.localCacheReads = registerOnce(metrics.localCacheReads).(*prometheus.CounterVec)
	return metrics
}

// DatabaseOperations returns the counter for the database operation.
// The provided params are used as labels.
func (m *StorageMetrics) DatabaseOperations(db, operation, status string) prometheus.Counter {
	return m.databaseOperations.WithLabelValues(db, operation, status)
}

// DatabaseLatencies returns a new latency timer for the provided
// database operation.
// The provided params are used as labels.
func (m *StorageMetrics) DatabaseLatencies(db string, operation string) *prometheus.Timer {
	return prometheus.NewTimer(m.databaseLatencies.WithLabelValues(db, operation))
}

// LocalCacheReads returns the counter for the local cache read.
// The provided params are used as labels.
func (m *StorageMetrics) LocalCacheReads(status CacheReadStatus) prometheus.Counter {
	return m.localCacheReads.WithLabelValues("TODO_runtime", string(status))
}
