package nodestats

import (
	"context"
	"time"

	"github.com/oasisprotocol/nexus/analyzer"
	"github.com/oasisprotocol/nexus/analyzer/queries"
	"github.com/oasisprotocol/nexus/config"
	"github.com/oasisprotocol/nexus/log"
	"github.com/oasisprotocol/nexus/metrics"
	"github.com/oasisprotocol/nexus/storage"
	source "github.com/oasisprotocol/nexus/storage/oasis"
)

const (
	fetchHeightTimeout    = 6 * time.Second // average block time
	nodeStatsAnalyzerName = "node_stats"
	consensusLayer        = "consensus"
)

type main struct {
	source   storage.ConsensusSourceStorage
	target   storage.TargetStorage
	logger   *log.Logger
	metrics  metrics.DatabaseMetrics
	interval time.Duration
}

var _ analyzer.Analyzer = (*main)(nil)

func NewMain(
	cfg *config.NodeStatsConfig,
	sourceClient *source.ConsensusClient,
	target storage.TargetStorage,
	logger *log.Logger,
) (analyzer.Analyzer, error) {
	m := &main{
		source:   sourceClient,
		target:   target,
		logger:   logger.With("analyzer", nodeStatsAnalyzerName),
		metrics:  metrics.NewDefaultDatabaseMetrics(nodeStatsAnalyzerName),
		interval: cfg.Interval,
	}

	return m, nil
}

// Queries oasis-node for its current height.
func (m main) Start(ctx context.Context) {
	var (
		requestCtx       context.Context
		requestCtxCancel context.CancelFunc = func() {}
	)
	for {
		requestCtxCancel()
		select {
		case <-ctx.Done():
			m.logger.Warn("shutting down node stats analyzer", "reason", ctx.Err())
			return
		case <-time.After(m.interval):
			// Fetch node height
		}
		requestCtx, requestCtxCancel = context.WithTimeout(ctx, fetchHeightTimeout)

		m.logger.Debug("fetching node height")
		height, err := m.source.LatestBlockHeight(requestCtx)
		if err != nil {
			m.logger.Error("error fetching latest block height",
				"err", err.Error(),
			)
			continue
		}
		batch := &storage.QueryBatch{}
		batch.Queue(queries.ConsensusNodeHeightUpsert,
			consensusLayer,
			height,
		)
		// Apply updates to DB.
		opName := nodeStatsAnalyzerName + "_" + consensusLayer
		timer := m.metrics.DatabaseTimer(m.target.Name(), opName)
		defer timer.ObserveDuration()

		if err := m.target.SendBatch(ctx, batch); err != nil {
			m.metrics.DatabaseCounter(m.target.Name(), opName, "failure").Inc()
			m.logger.Error("error inserting node height into db",
				"err", err.Error(),
			)
			continue
		}
		m.metrics.DatabaseCounter(m.target.Name(), opName, "success").Inc()
	}
}

func (m main) Name() string {
	return nodeStatsAnalyzerName
}
