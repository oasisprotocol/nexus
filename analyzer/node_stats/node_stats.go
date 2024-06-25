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
	runtimeAPI "github.com/oasisprotocol/nexus/coreapi/v22.2.11/runtime/client/api"
)

const (
	nodeStatsAnalyzerName = "node_stats"
)

type processor struct {
	layers          []common.Layer
	consensusSource nodeapi.ConsensusApiLite
	runtimeSources  map[common.Runtime]nodeapi.RuntimeApiLite
	target          storage.TargetStorage
	logger          *log.Logger
}

var _ item.ItemProcessor[common.Layer] = (*processor)(nil)

func NewAnalyzer(
	cfg config.ItemBasedAnalyzerConfig,
	layers []common.Layer,
	consensusClient nodeapi.ConsensusApiLite,
	runtimeClients map[common.Runtime]nodeapi.RuntimeApiLite,
	target storage.TargetStorage,
	logger *log.Logger,
) (analyzer.Analyzer, error) {
	if cfg.Interval == 0 {
		cfg.Interval = 3 * time.Second
	}
	logger = logger.With("analyzer", nodeStatsAnalyzerName)
	// Default to [consensus, emerald, sapphire, pontusx_test, pontusx_dev] if layers is not specified.
	if len(layers) == 0 {
		layers = []common.Layer{common.LayerConsensus, common.LayerEmerald, common.LayerSapphire, common.LayerPontusxTest, common.LayerPontusxDev}
	}
	p := &processor{
		layers:          layers,
		consensusSource: consensusClient,
		runtimeSources:  runtimeClients,
		target:          target,
		logger:          logger.With("analyzer", nodeStatsAnalyzerName),
	}

	return item.NewAnalyzer[common.Layer](
		nodeStatsAnalyzerName,
		cfg,
		p,
		target,
		logger)
}

func (p *processor) GetItems(ctx context.Context, limit uint64) ([]common.Layer, error) {
	return p.layers, nil
}

func (p *processor) ProcessItem(ctx context.Context, batch *storage.QueryBatch, layer common.Layer) error {
	p.logger.Debug("fetching node height", "layer", layer)
	latestHeight := uint64(0) // will be fetched from the node
	if layer == common.LayerConsensus {
		latestBlock, err := p.consensusSource.GetBlock(ctx, consensusAPI.HeightLatest)
		if err != nil {
			return fmt.Errorf("error fetching latest block height for layer %s: %w", layer, err)
		}
		latestHeight = uint64(latestBlock.Height)
	} else { // the layer is a runtime
		runtimeClient, ok := p.runtimeSources[common.Runtime(layer)]
		if !ok || runtimeClient == nil {
			return fmt.Errorf("unsupported layer %s", layer)
		}
		latestBlock, err := runtimeClient.GetBlockHeader(ctx, runtimeAPI.RoundLatest)
		if err != nil {
			return fmt.Errorf("error fetching latest block height for layer %s: %w", layer, err)
		}
		latestHeight = latestBlock.Round
	}
	batch.Queue(queries.NodeHeightUpsert,
		layer,
		latestHeight,
	)

	return nil
}

func (p *processor) QueueLength(ctx context.Context) (int, error) {
	// The concept of a work queue does not apply to this analyzer
	return 0, nil
}
