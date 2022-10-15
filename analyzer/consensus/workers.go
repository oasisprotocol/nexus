package consensus

import (
	"context"
	"time"

	"github.com/oasisprotocol/oasis-indexer/storage"
)

const (
	// We would ideally recompute stats (which include 5-min aggregates) every 5 min, but the
	// current naive implementation is too inefficient for that. Conservatively wait a long time,
	// also so as not to overburden the DB with computing aggregate stats.
	aggregateWorkerInterval = 6 * time.Hour
	aggregateWorkerTimeout  = 10 * time.Minute // as of height 10767524, this takes ~3m15s
)

// The aggregate worker continually refreshes aggregate statistics for the consensus layer.
//
// NOTE: The worker generates stats by bucketing _all_ txs into 5-minute (and 1-day)
// intervals. In other words, all historical stats are recomputed every time.
// This is inefficient, but easy to implement.
func (m *Main) aggregateWorker(ctx context.Context) {
	m.logger.Info("starting aggregate worker")
	m.aggregate(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(aggregateWorkerInterval):
			m.aggregate(ctx)
		}
	}
}

// Performs a single run of aggregation.
func (m *Main) aggregate(ctx context.Context) {
	cancelCtx, cancel := context.WithTimeout(ctx, aggregateWorkerTimeout)
	defer cancel()
	startTime := time.Now()
	m.logger.Info("starting aggregate statistics refresh")

	// Note: The order matters here! Daily tx volume is a materialized view
	// that's instantiated from 5 minute tx volume.
	batch := &storage.QueryBatch{}
	batch.Queue(m.qf.RefreshMin5TxVolumeQuery())
	batch.Queue(m.qf.RefreshDailyTxVolumeQuery())

	opName := "aggregate_stats"
	timer := m.metrics.DatabaseTimer(m.target.Name(), opName)
	defer timer.ObserveDuration()

	if err := m.target.SendBatch(cancelCtx, batch); err != nil {
		m.metrics.DatabaseCounter(m.target.Name(), opName, "failure").Inc()
		m.logger.Error("Failed to refresh aggregate statistics",
			"err", err,
		)
	}

	m.logger.Info("refreshed aggregate statistics", "duration", time.Since(startTime))
}
