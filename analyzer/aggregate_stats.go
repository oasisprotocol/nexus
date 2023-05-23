package analyzer

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/oasisprotocol/oasis-indexer/analyzer/queries"
	"github.com/oasisprotocol/oasis-indexer/common"
	"github.com/oasisprotocol/oasis-indexer/config"
	"github.com/oasisprotocol/oasis-indexer/log"
	"github.com/oasisprotocol/oasis-indexer/metrics"
	"github.com/oasisprotocol/oasis-indexer/storage"
)

const (
	AggregateStatsAnalyzerName = "aggregate_stats"

	// We would ideally recompute tx volume stats (which include 5-min aggregates) every 5 min,
	// but the current naive implementation is too inefficient for that. Allow for the update
	// to take longer.
	txVolumeStatsUpdateTimeout = 10 * time.Minute // as of height 10767524, this takes ~3m15s

	// Name of the consensus layer.
	layerConsensus = "consensus"

	// Limit the number of daily active accounts aggregation windows to be computed in a single
	// iteration.
	// This limits database batch sizes in hypothetical cases if the daily active accounts
	// analyzer would be far behind latest indexed block (e.g. if it would not be enabled during initial sync).
	// Likely never the case in normal operation.
	dailyActiveAccountsBatchLimit = 1_000
	// Interval between daily active accounts iterations.
	// The worker only computes new data and iteration with no updates are cheap
	// so it's fine to run this frequently.
	dailyActiveAccountsInterval = 1 * time.Minute

	// 24-hour sliding window with 5 minute step.
	dailyActiveAccountsWindowSize = 24 * time.Hour
	dailyActiveAccountsWindowStep = 5 * time.Minute
)

// Layers that are tracked for daily active account stats.
// XXX: Instead of hardcoding, these could also be obtained
// by (periodically) querying the `runtime_related_transactions` table.
var dailyActiveAccountsLayers = []string{
	layerConsensus,
	string(common.RuntimeEmerald),
	string(common.RuntimeSapphire),
	// RuntimeCipher.String(), // Enable once Cipher is supported by the indexer.
}

type AggregateStatsAnalyzer struct {
	target storage.TargetStorage

	txVolumeInterval time.Duration

	logger  *log.Logger
	metrics metrics.DatabaseMetrics
}

var _ Analyzer = (*AggregateStatsAnalyzer)(nil)

func (a *AggregateStatsAnalyzer) Name() string {
	return AggregateStatsAnalyzerName
}

func NewAggregateStatsAnalyzer(cfg *config.AggregateStatsConfig, target storage.TargetStorage, logger *log.Logger) (*AggregateStatsAnalyzer, error) {
	logger.Info("starting aggregate_stats analyzer")
	return &AggregateStatsAnalyzer{
		target:           target,
		txVolumeInterval: cfg.TxVolumeInterval,
		logger:           logger.With("analyzer", AggregateStatsAnalyzerName),
		metrics:          metrics.NewDefaultDatabaseMetrics(AggregateStatsAnalyzerName),
	}, nil
}

func (a *AggregateStatsAnalyzer) Start(ctx context.Context) {
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		a.txVolumeWorker(ctx)
	}()
	go func() {
		defer wg.Done()
		a.dailyActiveAccountsWorker(ctx)
	}()

	wg.Wait()
}

// txVolumeWorker periodically updates the tx volume stats.
func (a *AggregateStatsAnalyzer) txVolumeWorker(ctx context.Context) {
	for {
		a.logger.Info("updating tx volume stats (5-min and daily)")

		// Note: The order matters here! Daily tx volume is a materialized view
		// that's instantiated from 5 minute tx volume.
		batch := &storage.QueryBatch{}
		batch.Queue(queries.RefreshMin5TxVolume)
		batch.Queue(queries.RefreshDailyTxVolume)

		ctxWithTimeout, cancelCtx := context.WithTimeout(ctx, txVolumeStatsUpdateTimeout)
		if err := a.writeToDB(ctxWithTimeout, batch, "update_tx_volume_stats"); err != nil {
			a.logger.Error("failed to trigger tx volume stats updates", "err", err)
		}
		a.logger.Info("updated tx volume stats", "num_materialized_views", batch.Len())
		cancelCtx()

		select {
		case <-time.After(a.txVolumeInterval):
			// Update stats.
		case <-ctx.Done():
			a.logger.Error("shutting down tx volume worker", "reason", ctx.Err())
			return
		}
	}
}

func (a *AggregateStatsAnalyzer) writeToDB(ctx context.Context, batch *storage.QueryBatch, opName string) error {
	timer := a.metrics.DatabaseTimer(a.target.Name(), opName)
	defer timer.ObserveDuration()

	if err := a.target.SendBatch(ctx, batch); err != nil {
		a.metrics.DatabaseCounter(a.target.Name(), opName, "failure").Inc()
		return err
	}
	a.metrics.DatabaseCounter(a.target.Name(), opName, "success").Inc()

	return nil
}

// dailyActiveAccountsWorker computes a sliding window of daily unique active accounts.
func (a *AggregateStatsAnalyzer) dailyActiveAccountsWorker(ctx context.Context) {
	windowSize := dailyActiveAccountsWindowSize
	windowStep := dailyActiveAccountsWindowStep

	// Rounds the timestamp down to the nearest windowStep interval.
	// e.g. if windowStep is 5 minutes, then 12:34:56 will be rounded down to 12:30:00.
	floorWindow := func(ts *time.Time) time.Time {
		return ts.Truncate(windowStep)
	}

	for {
		for _, layer := range dailyActiveAccountsLayers {
			a.logger.Info("updating daily active account stats", "layer", layer)

			// Query the latest indexed block timestamp.
			latestBlockTs, err := a.latestBlockTs(ctx, layer)
			if err != nil {
				// Log this as info as this can be expected when there's no indexed blocks yet, or if any of the layers are not being indexed.
				a.logger.Info("failed querying latest indexed block timestamp, skipping iteration", "layer", layer, "err", err)
				continue
			}
			latestPossibleWindow := floorWindow(latestBlockTs)

			// Start at the latest already computed window, or at the earliest indexed
			// block if no stats have been computed yet.
			var latestStatsTs time.Time
			err = a.target.QueryRow(
				ctx,
				queries.LatestDailyAccountStats,
				layer,
			).Scan(&latestStatsTs)
			switch {
			case err == nil:
				// Continues below.
			case errors.Is(pgx.ErrNoRows, err):
				// No stats yet. Start at the earliest indexed block.
				var earliestBlockTs *time.Time
				earliestBlockTs, err = a.earliestBlockTs(ctx, layer)
				if err != nil {
					a.logger.Error("failed querying earliest indexed block timestamp", "layer", layer, "err", err)
					continue
				}
				latestStatsTs = floorWindow(earliestBlockTs)
			default:
				a.logger.Error("failed querying latest daily accounts stats window", "layer", layer, "err", err)
				continue
			}

			// Compute daily unique active accounts for windows until the latest indexed block.
			batch := &storage.QueryBatch{}
			for {
				nextWindow := latestStatsTs.Add(windowStep)
				if !latestPossibleWindow.After(nextWindow) {
					// Cannot yet compute stats for next window. Stop.
					break
				}
				windowStart := nextWindow.Add(-windowSize)
				windowEnd := nextWindow

				// Compute active accounts for the provided time window.
				var acctsRow pgx.Row
				switch layer {
				case layerConsensus:
					acctsRow = a.target.QueryRow(
						ctx,
						queries.ConsensusActiveAccounts,
						windowStart, // from
						windowEnd,   // to
					)
				default:
					acctsRow = a.target.QueryRow(
						ctx,
						queries.RuntimeActiveAccounts,
						layer,       // runtime
						windowStart, // from
						windowEnd,   // to
					)
				}
				var activeAccounts uint64
				if err := acctsRow.Scan(&activeAccounts); err != nil {
					a.logger.Error("failed to compute daily active accounts", "layer", layer, "window_start", windowStart, "window_end", windowEnd, "err", err)
					continue
				}

				// Insert computed active accounts.
				batch.Queue(queries.InsertDailyAccountStats, layer, windowEnd.UTC(), activeAccounts)
				if batch.Len() > dailyActiveAccountsBatchLimit {
					break
				}
				latestStatsTs = nextWindow
			}
			if err := a.writeToDB(ctx, batch, fmt.Sprintf("update_daily_active_account_%s_stats", layer)); err != nil {
				a.logger.Error("failed to insert daily active account stats update", "err", err, "layer", layer)
			}
			a.logger.Info("updated daily active account stats", "layer", layer, "num_inserts", batch.Len())
		}

		select {
		case <-time.After(dailyActiveAccountsInterval):
			// Update stats again.
		case <-ctx.Done():
			a.logger.Error("shutting down daily active accounts worker", "reason", ctx.Err())
			return
		}
	}
}

// Queries the latest indexed block of the specified layer.
func (a *AggregateStatsAnalyzer) latestBlockTs(ctx context.Context, layer string) (*time.Time, error) {
	var latestBlockTsRow pgx.Row
	switch layer {
	case layerConsensus:
		latestBlockTsRow = a.target.QueryRow(ctx, queries.LatestConsensusBlockTime)
	default:
		latestBlockTsRow = a.target.QueryRow(ctx, queries.LatestRuntimeBlockTime, layer)
	}
	var latestBlockTs *time.Time
	if err := latestBlockTsRow.Scan(&latestBlockTs); err != nil { // Fails with ErrNoRows if no blocks.
		return nil, err
	}
	return latestBlockTs, nil
}

// Queries the earliest indexed block for the specified layer.
func (a *AggregateStatsAnalyzer) earliestBlockTs(ctx context.Context, layer string) (*time.Time, error) {
	var earliestBlockTsRow pgx.Row
	switch layer {
	case layerConsensus:
		earliestBlockTsRow = a.target.QueryRow(ctx, queries.EarliestConsensusBlockTime)
	default:
		earliestBlockTsRow = a.target.QueryRow(ctx, queries.EarliestRuntimeBlockTime, layer)
	}
	var earliestBlockTs *time.Time
	if err := earliestBlockTsRow.Scan(&earliestBlockTs); err != nil { // Fails with ErrNoRows if no blocks.
		return nil, err
	}
	return earliestBlockTs, nil
}
