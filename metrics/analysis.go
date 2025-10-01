package metrics

import (
	"fmt"
	"math"

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

	// Count of block fetche statuses, either successful or failed.
	blockFetches *prometheus.CounterVec

	// Queue length of analyzer.
	queueLengths *prometheus.GaugeVec
}

type CacheReadStatus string

const (
	CacheReadStatusHit      CacheReadStatus = "hit"
	CacheReadStatusMiss     CacheReadStatus = "miss"
	CacheReadStatusBadValue CacheReadStatus = "bad_value" // Value in cache was not valid (likely because of mismatched types / CBOR encoding).
	CacheReadStatusError    CacheReadStatus = "error"     // Other internal error reading from cache.
)

type BlockFetchStatus string

const (
	BlockFetchStatusSuccess BlockFetchStatus = "success"
	BlockFetchStatusError   BlockFetchStatus = "error"
)

// defaultTimeBuckets returns a set of buckets for use in a timing histogram.
// The buckets are logarithmically spaced between two hardcoded thresholds.
func defaultTimeBuckets() []float64 {
	const (
		minThreshold                  = 0.0001
		maxThreshold                  = 1000 // The resulting output might be one bucket short due to rounding errors.
		thresholdsPerOrderOfMagnitude = 10   // "Order of magnitude" being 10x.
	)

	buckets := []float64{}
	threshold := minThreshold
	for threshold <= maxThreshold {
		buckets = append(buckets, threshold)
		threshold *= math.Pow(10, 1.0/thresholdsPerOrderOfMagnitude)
	}
	return buckets
}

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
				Name:    fmt.Sprintf("%s_db_latencies", runtime),
				Help:    "How long database operations take, partitioned by operation.",
				Buckets: defaultTimeBuckets(),
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
				Name:    "block_analysis_latencies",
				Help:    "How long it takes to analyze a block, NOT including data fetch (from the node) or writing (to the DB).",
				Buckets: defaultTimeBuckets(),
			},
			[]string{"layer"}, // Labels.
		),
		blockFetchLatencies: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "block_fetch_latencies",
				Help:    "How long it takes to fetch data for a block from the node (possibly sped up by a local cache).",
				Buckets: defaultTimeBuckets(),
			},
			[]string{"layer"}, // Labels.
		),
		blockFetches: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "block_fetches",
				Help: "Block fetch counter, by request status (success, error).",
			},
			[]string{"layer", "status"}, // Labels.
		),
		queueLengths: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: fmt.Sprintf("%s_queue_length", runtime),
				Help: "How many blocks or items are left to process, partitioned by analyzer",
			},
			[]string{"analyzer"}, // Labels
		),
	}
	metrics.databaseOperations = registerOnce(metrics.databaseOperations).(*prometheus.CounterVec)
	metrics.databaseLatencies = registerOnce(metrics.databaseLatencies).(*prometheus.HistogramVec)
	metrics.localCacheReads = registerOnce(metrics.localCacheReads).(*prometheus.CounterVec)
	metrics.blockAnalysisLatencies = registerOnce(metrics.blockAnalysisLatencies).(*prometheus.HistogramVec)
	metrics.blockFetchLatencies = registerOnce(metrics.blockFetchLatencies).(*prometheus.HistogramVec)
	metrics.blockFetches = registerOnce(metrics.blockFetches).(*prometheus.CounterVec)
	metrics.queueLengths = registerOnce(metrics.queueLengths).(*prometheus.GaugeVec)
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

func (m *AnalysisMetrics) BlockFetches(status BlockFetchStatus) prometheus.Counter {
	return m.blockFetches.WithLabelValues(m.runtime, string(status))
}

func (m *AnalysisMetrics) QueueLength(analyzer string) prometheus.Gauge {
	return m.queueLengths.WithLabelValues(analyzer)
}
