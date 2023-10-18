package metadata_registry

import (
	"context"
	"fmt"
	"time"

	registry "github.com/oasisprotocol/metadata-registry-tools"

	staking "github.com/oasisprotocol/nexus/coreapi/v22.2.11/staking/api"

	"github.com/oasisprotocol/nexus/analyzer"
	"github.com/oasisprotocol/nexus/analyzer/item"
	"github.com/oasisprotocol/nexus/analyzer/queries"
	"github.com/oasisprotocol/nexus/config"
	"github.com/oasisprotocol/nexus/log"
	"github.com/oasisprotocol/nexus/storage"
)

const MetadataRegistryAnalyzerName = "metadata_registry"

type processor struct {
	target storage.TargetStorage
	logger *log.Logger
}

var _ item.ItemProcessor[struct{}] = (*processor)(nil)

func NewAnalyzer(
	cfg config.ItemBasedAnalyzerConfig,
	target storage.TargetStorage,
	logger *log.Logger,
) (analyzer.Analyzer, error) {
	logger.Info("Starting metadata_registry analyzer")
	if cfg.Interval == 0 {
		cfg.Interval = 2 * time.Minute
	}
	if cfg.Interval < time.Minute {
		return nil, fmt.Errorf("invalid interval %s, metadata registry interval must be at least 1 minute", cfg.Interval)
	}
	logger = logger.With("analyzer", MetadataRegistryAnalyzerName)
	p := &processor{
		target: target,
		logger: logger,
	}

	return item.NewAnalyzer[struct{}](
		MetadataRegistryAnalyzerName,
		cfg,
		p,
		target,
		logger,
	)
}

func (p *processor) GetItems(ctx context.Context, limit uint64) ([]struct{}, error) {
	return []struct{}{{}}, nil
}

func (p *processor) ProcessItem(ctx context.Context, batch *storage.QueryBatch, item struct{}) error {
	gp, err := registry.NewGitProvider(registry.NewGitConfig())
	if err != nil {
		return fmt.Errorf("failed to create Git registry provider: %s", err)
	}

	// Get a list of all entities in the registry.
	entities, err := gp.GetEntities(ctx)
	if err != nil {
		return fmt.Errorf("failed to get a list of entities in registry: %s", err)
	}

	for id, meta := range entities {
		batch.Queue(
			queries.ConsensusEntityMetaUpsert,
			id.String(),
			staking.NewAddress(id).String(),
			meta,
		)
	}

	return nil
}

func (p *processor) QueueLength(ctx context.Context) (int, error) {
	// The concept of a work queue does not apply to this analyzer.
	return 0, nil
}
