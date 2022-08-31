package consensus

import (
	"context"
	"time"

	"github.com/oasisprotocol/oasis-indexer/storage"
)

const (
	aggregateWorkerInterval = 10 * time.Second
	aggregateWorkerTimeout  = 60 * time.Second
)

// The aggregate worker refreshes aggregate statistics for the consensus layer.
func (m *Main) aggregateWorker(ctx context.Context) {
	m.logger.Info("starting aggregate worker")
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(aggregateWorkerInterval):
			func() {
				cancelCtx, cancel := context.WithTimeout(ctx, aggregateWorkerTimeout)
				defer cancel()

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

				m.logger.Debug("aggregate worker step completed")
			}()
		}
	}
}
