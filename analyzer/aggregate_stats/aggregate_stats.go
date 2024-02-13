package aggregate_stats

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/oasisprotocol/nexus/analyzer"
	"github.com/oasisprotocol/nexus/common"
	"github.com/oasisprotocol/nexus/log"
	"github.com/oasisprotocol/nexus/metrics"
	"github.com/oasisprotocol/nexus/storage"
)

const (
	aggregateStatsAnalyzerName = "aggregate_stats"

	// Limit the number of stats aggregation windows to be computed in a single
	// iteration.
	// This limits database batch sizes in cases if the stats analyzer would be far behind
	// latest indexed block (e.g. if it would not be enabled during initial sync).
	// Likely never the case in normal operation.
	statsBatchLimit = 100

	// Name of the consensus layer.
	layerConsensus = "consensus"

	// Timeout when processing a single batch of a single statistic.
	statsComputationTimeout = 3 * time.Minute

	// Interval between stat computation iterations.
	// The workers only computes new data and iteration with no updates are cheap
	// so it's fine to run this frequently.
	statsComputationInterval = 1 * time.Minute

	// Interval between stat computation iterations if the aggregate stats worker is catching up
	// (indicated by the fact that the per iteration batch limit was reached).
	statsComputationIntervalCatchup = 5 * time.Second
)

// Layers that are tracked for aggregate stats.
// XXX: Instead of hardcoding, these could also be obtained
// by (periodically) querying the `runtime_related_transactions` table.
var statsLayers = []string{
	layerConsensus,
	string(common.RuntimeEmerald),
	string(common.RuntimeSapphire),
	string(common.RuntimePontusx),
	string(common.RuntimeCipher),
}

type aggregateStatsAnalyzer struct {
	target storage.TargetStorage

	logger  *log.Logger
	metrics metrics.AnalysisMetrics
}

var _ analyzer.Analyzer = (*aggregateStatsAnalyzer)(nil)

func (a *aggregateStatsAnalyzer) Name() string {
	return aggregateStatsAnalyzerName
}

func NewAggregateStatsAnalyzer(target storage.TargetStorage, logger *log.Logger) (analyzer.Analyzer, error) {
	logger.Info("starting aggregate_stats analyzer")
	return &aggregateStatsAnalyzer{
		target:  target,
		logger:  logger.With("analyzer", aggregateStatsAnalyzerName),
		metrics: metrics.NewDefaultAnalysisMetrics(aggregateStatsAnalyzerName),
	}, nil
}

func (a *aggregateStatsAnalyzer) Start(ctx context.Context) {
	a.aggregateStatsWorker(ctx)
}

func (a *aggregateStatsAnalyzer) writeToDB(ctx context.Context, batch *storage.QueryBatch, opName string) error {
	timer := a.metrics.DatabaseLatencies(a.target.Name(), opName)
	defer timer.ObserveDuration()

	if err := a.target.SendBatch(ctx, batch); err != nil {
		a.metrics.DatabaseOperations(a.target.Name(), opName, "failure").Inc()
		return err
	}
	a.metrics.DatabaseOperations(a.target.Name(), opName, "success").Inc()

	return nil
}

func (a *aggregateStatsAnalyzer) aggregateStatsWorker(ctx context.Context) {
	statsComputations := []*statsComputation{}

	// Compute 5-minute tx volume stats every 5 minutes for all layers.
	for _, layer := range statsLayers {
		sc := &statsComputation{
			target:       a.target,
			name:         "min5_tx_volume_" + layer,
			layer:        layer,
			outputTable:  "stats.min5_tx_volume",
			outputColumn: "tx_volume",
			windowSize:   5 * time.Minute,
			windowStep:   5 * time.Minute,
		}
		if layer == layerConsensus {
			// Consensus layer queries.
			sc.statsQuery = QueryConsensusTxVolume
			sc.latestAvailableDataTs = func(ctx context.Context, target storage.TargetStorage) (*time.Time, error) {
				var latestBlockTs *time.Time
				err := target.QueryRow(ctx, QueryLatestConsensusBlockTime).Scan(&latestBlockTs)
				return latestBlockTs, err
			}
		} else {
			// Runtime layer queries.
			sc.statsQuery = QueryRuntimeTxVolume
			sc.latestAvailableDataTs = func(ctx context.Context, target storage.TargetStorage) (*time.Time, error) {
				var latestBlockTs *time.Time
				err := target.QueryRow(ctx, QueryLatestRuntimeBlockTime, sc.layer).Scan(&latestBlockTs)
				return latestBlockTs, err
			}
		}
		statsComputations = append(statsComputations, sc)
	}

	// Compute daily tx volume stats every 5 minutes for all layers.
	// Uses the stats.min5_tx_volume results so that it is efficient.
	for _, layer := range statsLayers {
		layer := layer
		sc := &statsComputation{
			target:       a.target,
			name:         "daily_tx_volume_" + layer,
			layer:        layer,
			outputTable:  "stats.daily_tx_volume",
			outputColumn: "tx_volume",
			windowSize:   24 * time.Hour,
			windowStep:   5 * time.Minute,
			// Latest available data is the latest computed 5-minute tx volume window.
			latestAvailableDataTs: func(ctx context.Context, target storage.TargetStorage) (*time.Time, error) {
				var latestTs *time.Time
				return latestTs, a.target.QueryRow(ctx, QueryLatestMin5TxVolume, layer).Scan(&latestTs)
			},
			statsQuery: QueryDailyTxVolume,
		}
		statsComputations = append(statsComputations, sc)
	}

	// Compute daily active accounts every 5 minutes for all layers.
	for _, layer := range statsLayers {
		sc := &statsComputation{
			target:       a.target,
			name:         "daily_active_accounts" + "_" + layer,
			layer:        layer,
			outputTable:  "stats.daily_active_accounts",
			outputColumn: "active_accounts",
			windowSize:   24 * time.Hour,
			windowStep:   5 * time.Minute,
		}
		if layer == layerConsensus {
			// Consensus layer queries.
			sc.statsQuery = QueryConsensusActiveAccounts
			sc.latestAvailableDataTs = func(ctx context.Context, target storage.TargetStorage) (*time.Time, error) {
				var latestBlockTs *time.Time
				return latestBlockTs, target.QueryRow(ctx, QueryLatestConsensusBlockTime).Scan(&latestBlockTs)
			}
		} else {
			// Runtime layer queries.
			sc.statsQuery = QueryRuntimeActiveAccounts
			sc.latestAvailableDataTs = func(ctx context.Context, target storage.TargetStorage) (*time.Time, error) {
				var latestBlockTs *time.Time
				return latestBlockTs, target.QueryRow(ctx, QueryLatestRuntimeBlockTime, sc.layer).Scan(&latestBlockTs)
			}
		}
		statsComputations = append(statsComputations, sc)
	}

	for {
		// If batch limit was reached, or the batch timeout was reached, start the next iteration sooner.
		var useCatchupTimeout bool
		for _, statsComputation := range statsComputations {
			statCtx, cancel := context.WithTimeout(ctx, statsComputationTimeout)
			statEndTime := time.Now().Add(statsComputationTimeout)
			logger := a.logger.With("name", statsComputation.name, "layer", statsComputation.layer)

			windowSize := statsComputation.windowSize
			windowStep := statsComputation.windowStep
			// Rounds the timestamp down to the nearest windowStep interval.
			// e.g. if windowStep is 5 minutes, then 12:34:56 will be rounded down to 12:30:00.
			floorWindow := func(ts *time.Time) time.Time {
				return ts.Truncate(windowStep)
			}

			logger.Info("updating stats")

			// First we find the start and end timestamps for which we can compute stats.
			// Start at the latest already computed window, or at the earliest indexed
			// block if no stats have been computed yet.
			latestComputed, err := statsComputation.LatestComputedTs(statCtx)
			switch {
			case err == nil:
				// Continues below.
			case errors.Is(pgx.ErrNoRows, err):
				// No stats yet. Start at the earliest indexed block.
				var earliestBlockTs *time.Time
				earliestBlockTs, err = a.earliestBlockTs(statCtx, statsComputation.layer)
				switch {
				case err == nil:
					latestComputed = floorWindow(earliestBlockTs)
				case errors.Is(pgx.ErrNoRows, err):
					// No data log a debug only log.
					logger.Debug("no stats available yet, skipping iteration")
					cancel()
					continue
				default:
					logger.Error("failed querying earliest indexed block timestamp", "err", err)
					cancel()
					continue
				}
			default:
				logger.Error("failed querying latest computed stats window", "err", err)
				cancel()
				continue
			}

			// End at the latest available data timestamp rounded down to the windowStep interval.
			latestPossibleWindow, err := statsComputation.latestAvailableDataTs(statCtx, a.target)
			switch {
			case err == nil:
				// Continues below.
			case errors.Is(pgx.ErrNoRows, err):
				logger.Debug("no stats available yet, skipping iteration")
				cancel()
				continue
			default:
				logger.Error("failed querying the latest possible window, skipping iteration", "err", err)
				cancel()
				continue
			}
			latestPossibleWindow = common.Ptr(floorWindow(latestPossibleWindow))

			// Compute stats for all windows between `latestComputed` and `latestPossibleWindow`.
			batch := &storage.QueryBatch{}
			for {
				nextWindow := latestComputed.Add(windowStep)
				if !latestPossibleWindow.After(nextWindow) {
					// Cannot yet compute stats for this window. Stop.
					break
				}
				windowStart := nextWindow.Add(-windowSize)
				windowEnd := nextWindow

				// Compute stats for the provided time window.
				queries, err := statsComputation.ComputeStats(statCtx, windowStart, windowEnd)
				if err != nil {
					logger.Error("failed to compute stat for the window", "window_start", windowStart, "window_end", windowEnd, "err", err)
					break
				}
				batch.Extend(queries)

				if batch.Len() > statsBatchLimit {
					useCatchupTimeout = true
					break
				}
				latestComputed = nextWindow
			}
			cancel()
			// If the computation was limited by the timeout; start the next computation sooner.
			if time.Now().After(statEndTime) {
				useCatchupTimeout = true
			}
			if err := a.writeToDB(ctx, batch, fmt.Sprintf("update_stats_%s", a.Name())); err != nil {
				logger.Error("failed to insert computed stats update", "err", err)
				continue
			}
			logger.Info("updated stats", "num_inserts", batch.Len())
		}

		timeout := statsComputationInterval
		if useCatchupTimeout {
			// Batch limit reached, or batch timeout was exceeded.
			timeout = statsComputationIntervalCatchup
		}
		select {
		case <-time.After(timeout):
			// Update stats again.
		case <-ctx.Done():
			a.logger.Error("shutting down aggregate stats worker", "reason", ctx.Err())
			return
		}
	}
}

// Queries the earliest indexed block for the specified layer.
func (a *aggregateStatsAnalyzer) earliestBlockTs(ctx context.Context, layer string) (*time.Time, error) {
	var earliestBlockTsRow pgx.Row
	switch layer {
	case layerConsensus:
		earliestBlockTsRow = a.target.QueryRow(ctx, QueryEarliestConsensusBlockTime)
	default:
		earliestBlockTsRow = a.target.QueryRow(ctx, QueryEarliestRuntimeBlockTime, layer)
	}
	var earliestBlockTs *time.Time
	if err := earliestBlockTsRow.Scan(&earliestBlockTs); err != nil { // Fails with ErrNoRows if no blocks.
		return nil, err
	}
	return earliestBlockTs, nil
}
