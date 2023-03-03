package analyzer

import (
	"context"
	"fmt"
	"time"

	registry "github.com/oasisprotocol/metadata-registry-tools"
	"github.com/oasisprotocol/oasis-indexer/config"
	"github.com/oasisprotocol/oasis-indexer/log"
	"github.com/oasisprotocol/oasis-indexer/metrics"
	"github.com/oasisprotocol/oasis-indexer/storage"
)

const MetadataRegistryAnalyzerName = "metadata_registry"

type MetadataRegistryAnalyzer struct {
	target   storage.TargetStorage
	logger   *log.Logger
	metrics  metrics.DatabaseMetrics
	interval time.Duration
}

var _ Analyzer = (*MetadataRegistryAnalyzer)(nil)

func (a *MetadataRegistryAnalyzer) Name() string {
	return MetadataRegistryAnalyzerName
}

func NewMetadataRegistryAnalyzer(chainID string, cfg *config.MetadataRegistryConfig, target storage.TargetStorage, logger *log.Logger) (*MetadataRegistryAnalyzer, error) {
	if chainID == "" {
		return nil, fmt.Errorf("metadata_registry analyzer: `ChainID` must be specified in the config")
	}

	logger.Info("Starting metadata_registry analyzer")
	return &MetadataRegistryAnalyzer{
		interval: cfg.Interval,
		target:   target,
		logger:   logger.With("analyzer", MetadataRegistryAnalyzerName),
		metrics:  metrics.NewDefaultDatabaseMetrics(MetadataRegistryAnalyzerName),
	}, nil
}

func (a *MetadataRegistryAnalyzer) Start() {
	ctx := context.Background()

	for {
		a.logger.Info("Updating metadata registry snapshot")

		batch := &storage.QueryBatch{}
		if err := a.queueUpdates(ctx, batch); err != nil {
			a.logger.Error("Failed to generate DB updates", "err", err)
		}
		if err := a.writeToDB(ctx, batch); err != nil {
			a.logger.Error("Failed to write DB updates", "err", err)
		}
		a.logger.Info("Updated metadata registry", "num_updates", batch.Len())

		time.Sleep(a.interval)
	}
}

func (a *MetadataRegistryAnalyzer) queueUpdates(ctx context.Context, batch *storage.QueryBatch) error {
	gp, err := registry.NewGitProvider(registry.NewGitConfig())
	if err != nil {
		a.logger.Error(fmt.Sprintf("Failed to create Git registry provider: %s\n", err))
		return err
	}

	// Get a list of all entities in the registry.
	entities, err := gp.GetEntities(ctx)
	if err != nil {
		a.logger.Error(fmt.Sprintf("Failed to get a list of entities in registry: %s\n", err))
		return err
	}

	for id, meta := range entities {
		batch.Queue(
			a.qf.ConsensusEntityMetaUpsertQuery(),
			id.String(),
			meta,
		)
	}

	return nil
}

func (a *MetadataRegistryAnalyzer) writeToDB(ctx context.Context, batch *storage.QueryBatch) error {
	opName := "update_metadata_registry"
	timer := a.metrics.DatabaseTimer(a.target.Name(), opName)
	defer timer.ObserveDuration()

	if err := a.target.SendBatch(ctx, batch); err != nil {
		a.metrics.DatabaseCounter(a.target.Name(), opName, "failure").Inc()
		return err
	}
	a.metrics.DatabaseCounter(a.target.Name(), opName, "success").Inc()

	return nil
}
