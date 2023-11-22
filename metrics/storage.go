package metrics

import (
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
)

// Default service metrics for analyzer operations.
type AnalysisMetrics struct {
	// Name of the runtime (or "consensus") that is being analyzed.
	runtime string

	// Counts of database operations
	databaseOperations *prometheus.CounterVec

	// Latencies of database operations.
	databaseLatencies *prometheus.HistogramVec

	// Cache hit rates for the local cache.
	localCacheReads *prometheus.CounterVec

	// Latencies of analysis.
	blockAnalysisLatencies *prometheus.HistogramVec

	// Latencies of fetching a block's data from the node.
	blockFetchLatencies *prometheus.HistogramVec
}

type CacheReadStatus string

const (
	CacheReadStatusHit      CacheReadStatus = "hit"
	CacheReadStatusMiss     CacheReadStatus = "miss"
	CacheReadStatusBadValue CacheReadStatus = "bad_value" // Value in cache was not valid (likely because of mismatched types / CBOR encoding).
	CacheReadStatusError    CacheReadStatus = "error"     // Other internal error reading from cache.
)

// NewDefaultAnalysisMetrics creates Prometheus metric instrumentation
// for basic metrics common to storage accesses.
func NewDefaultAnalysisMetrics(runtime string) AnalysisMetrics {
	metrics := AnalysisMetrics{
		runtime: runtime,
		databaseOperations: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: fmt.Sprintf("%s_db_operations", runtime),
				Help: "How many database operations occur, partitioned by operation and status.",
			},
			[]string{"database", "operation", "status"}, // Labels.
		),
		databaseLatencies: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name: fmt.Sprintf("%s_db_latencies", runtime),
				Help: "How long database operations take, partitioned by operation.",
			},
			[]string{"database", "operation"}, // Labels.
		),
		localCacheReads: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "local_cache_reads",
				Help: "How many local cache reads occur, partitioned by status (hit, miss, bad_data, error).",
			},
			[]string{"cache", "status"}, // Labels.
		),
		blockAnalysisLatencies: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name: "block_analysis_latencies",
				Help: "How long it takes to analyze a block, NOT including data fetch (from the node) or writing (to the DB).",
			},
			[]string{"layer"}, // Labels.
		),
		blockFetchLatencies: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name: "block_fetch_latencies",
				Help: "How long it takes to fetch data for a block from the node.",
			},
			[]string{"layer"}, // Labels.
		),
	}
	metrics.databaseOperations = registerOnce(metrics.databaseOperations).(*prometheus.CounterVec)
	metrics.databaseLatencies = registerOnce(metrics.databaseLatencies).(*prometheus.HistogramVec)
	metrics.localCacheReads = registerOnce(metrics.localCacheReads).(*prometheus.CounterVec)
	metrics.blockAnalysisLatencies = registerOnce(metrics.blockAnalysisLatencies).(*prometheus.HistogramVec)
	metrics.blockFetchLatencies = registerOnce(metrics.blockFetchLatencies).(*prometheus.HistogramVec)
	return metrics
}

// DatabaseOperations returns the counter for the database operation.
// The provided params are used as labels.
func (m *AnalysisMetrics) DatabaseOperations(db, operation, status string) prometheus.Counter {
	return m.databaseOperations.WithLabelValues(db, operation, status)
}

// DatabaseLatencies returns a new latency timer for the provided
// database operation.
// The provided params are used as labels.
func (m *AnalysisMetrics) DatabaseLatencies(db string, operation string) *prometheus.Timer {
	return prometheus.NewTimer(m.databaseLatencies.WithLabelValues(db, operation))
}

// LocalCacheReads returns the counter for the local cache read.
// The provided params are used as labels.
func (m *AnalysisMetrics) LocalCacheReads(status CacheReadStatus) prometheus.Counter {
	return m.localCacheReads.WithLabelValues(m.runtime, string(status))
}

func (m *AnalysisMetrics) BlockAnalysisLatencies() *prometheus.Timer {
	return prometheus.NewTimer(m.blockAnalysisLatencies.WithLabelValues(m.runtime))
}

func (m *AnalysisMetrics) BlockFetchLatencies() *prometheus.Timer {
	return prometheus.NewTimer(m.blockFetchLatencies.WithLabelValues(m.runtime))
}
