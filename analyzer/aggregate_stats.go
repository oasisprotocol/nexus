package analyzer

import (
	"context"
	"time"

	"github.com/oasisprotocol/oasis-indexer/config"
	"github.com/oasisprotocol/oasis-indexer/log"
	"github.com/oasisprotocol/oasis-indexer/metrics"
	"github.com/oasisprotocol/oasis-indexer/storage"
)

const (
	AggregateStatsAnalyzerName = "aggregate_stats"
	// We would ideally recompute stats (which include 5-min aggregates) every 5 min, but the
	// current naive implementation is too inefficient for that. Conservatively wait a long time,
	// also so as not to overburden the DB with computing aggregate stats.
	aggregateWorkerTimeout = 10 * time.Minute // as of height 10767524, this takes ~3m15s
)

type AggregateStatsAnalyzer struct {
	qf       QueryFactory
	target   storage.TargetStorage
	logger   *log.Logger
	metrics  metrics.DatabaseMetrics
	interval time.Duration
}

var _ Analyzer = (*AggregateStatsAnalyzer)(nil)

func (a *AggregateStatsAnalyzer) Name() string {
	return AggregateStatsAnalyzerName
}

func NewAggregateStatsAnalyzer(cfg *config.IntervalBasedAnalyzerConfig, target storage.TargetStorage, logger *log.Logger) (*AggregateStatsAnalyzer, error) {
	logger.Info("Starting aggregate_stats analyzer")
	return &AggregateStatsAnalyzer{
		interval: cfg.ParsedInterval(),
		qf:       NewQueryFactory("" /*chainID*/, "" /*runtime*/),
		target:   target,
		logger:   logger.With("analyzer", AggregateStatsAnalyzerName),
		metrics:  metrics.NewDefaultDatabaseMetrics(AggregateStatsAnalyzerName),
	}, nil
}

func (a *AggregateStatsAnalyzer) Start() {
	ctx := context.Background()

	for {
		a.logger.Info("Updating aggregate stats")

		// Note: The order matters here! Daily tx volume is a materialized view
		// that's instantiated from 5 minute tx volume.
		batch := &storage.QueryBatch{}
		batch.Queue(a.qf.RefreshMin5TxVolumeQuery())
		batch.Queue(a.qf.RefreshDailyTxVolumeQuery())

		ctxWithTimeout, cancelCtx := context.WithTimeout(ctx, aggregateWorkerTimeout)
		if err := a.writeToDB(ctxWithTimeout, batch); err != nil {
			a.logger.Error("Failed to trigger DB updates", "err", err)
		}
		a.logger.Info("Updated aggregate stats", "num_materialized_views", batch.Len())
		cancelCtx()

		time.Sleep(a.interval)
	}
}

func (a *AggregateStatsAnalyzer) writeToDB(ctx context.Context, batch *storage.QueryBatch) error {
	opName := "update_aggregate_stats"
	timer := a.metrics.DatabaseTimer(a.target.Name(), opName)
	defer timer.ObserveDuration()

	if err := a.target.SendBatch(ctx, batch); err != nil {
		a.metrics.DatabaseCounter(a.target.Name(), opName, "failure").Inc()
		return err
	}
	a.metrics.DatabaseCounter(a.target.Name(), opName, "success").Inc()

	return nil
}
