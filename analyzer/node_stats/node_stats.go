package nodestats

import (
	"context"
	"fmt"
	"time"

	"github.com/oasisprotocol/nexus/analyzer"
	"github.com/oasisprotocol/nexus/analyzer/item"
	"github.com/oasisprotocol/nexus/analyzer/queries"
	"github.com/oasisprotocol/nexus/common"
	"github.com/oasisprotocol/nexus/config"
	"github.com/oasisprotocol/nexus/log"
	"github.com/oasisprotocol/nexus/storage"
	"github.com/oasisprotocol/nexus/storage/oasis/nodeapi"

	consensusAPI "github.com/oasisprotocol/nexus/coreapi/v22.2.11/consensus/api"
)

const (
	nodeStatsAnalyzerName = "node_stats"
)

type processor struct {
	source nodeapi.ConsensusApiLite
	target storage.TargetStorage
	logger *log.Logger
}

var _ item.ItemProcessor[common.Layer] = (*processor)(nil)

func NewAnalyzer(
	cfg config.ItemBasedAnalyzerConfig,
	sourceClient nodeapi.ConsensusApiLite,
	target storage.TargetStorage,
	logger *log.Logger,
) (analyzer.Analyzer, error) {
	if cfg.Interval == 0 {
		cfg.Interval = 3 * time.Second
	}
	logger = logger.With("analyzer", nodeStatsAnalyzerName)
	p := &processor{
		source: sourceClient,
		target: target,
		logger: logger.With("analyzer", nodeStatsAnalyzerName),
	}

	return item.NewAnalyzer[common.Layer](
		nodeStatsAnalyzerName,
		cfg,
		p,
		target,
		logger)
}

func (p *processor) GetItems(ctx context.Context, limit uint64) ([]common.Layer, error) {
	return []common.Layer{common.LayerConsensus}, nil
}

func (p *processor) ProcessItem(ctx context.Context, batch *storage.QueryBatch, layer common.Layer) error {
	p.logger.Debug("fetching node height", "layer", layer)
	// We currently only support consensus. Update when this is no longer the case.
	if layer != common.LayerConsensus {
		return fmt.Errorf("unsupported layer %s", layer)
	}
	latestBlock, err := p.source.GetBlock(ctx, consensusAPI.HeightLatest)
	if err != nil {
		return fmt.Errorf("error fetching latest block height for layer %s, %w", layer, err)
	}
	batch.Queue(queries.ConsensusNodeHeightUpsert,
		layer,
		latestBlock.Height,
	)

	return nil
}

func (p *processor) QueueLength(ctx context.Context) (int, error) {
	// The concept of a work queue does not apply to this analyzer
	return 0, nil
}
