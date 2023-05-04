package analyzer

import (
	"context"
	"fmt"
	"time"

	registry "github.com/oasisprotocol/metadata-registry-tools"
	staking "github.com/oasisprotocol/oasis-core/go/staking/api"
	"github.com/oasisprotocol/oasis-indexer/analyzer/queries"
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

func NewMetadataRegistryAnalyzer(cfg *config.MetadataRegistryConfig, target storage.TargetStorage, logger *log.Logger) (Analyzer, error) {
	logger.Info("Starting metadata_registry analyzer")
	return &MetadataRegistryAnalyzer{
		interval: cfg.Interval,
		target:   target,
		logger:   logger.With("analyzer", MetadataRegistryAnalyzerName),
		metrics:  metrics.NewDefaultDatabaseMetrics(MetadataRegistryAnalyzerName),
	}, nil
}

func (a *MetadataRegistryAnalyzer) Start(ctx context.Context) {
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

		select {
		case <-time.After(a.interval):
			// Fetch data again.
		case <-ctx.Done():
			a.logger.Warn("shutting down metadata analyzer", "reason", ctx.Err())
			return
		}
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
			queries.ConsensusEntityMetaUpsert,
			id.String(),
			staking.NewAddress(id).String(),
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
