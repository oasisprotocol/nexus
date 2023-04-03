package api

import (
	"context"
	"os"
	"testing"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres" // postgres driver for golang_migrate
	_ "github.com/golang-migrate/migrate/v4/source/file"       // support file scheme for golang_migrate
	_ "github.com/golang-migrate/migrate/v4/source/github"     // support github scheme for golang_migrate
	oasisConfig "github.com/oasisprotocol/oasis-sdk/client-sdk/go/config"
	"github.com/stretchr/testify/require"

	"github.com/oasisprotocol/oasis-indexer/analyzer/consensus"
	"github.com/oasisprotocol/oasis-indexer/cmd/common"
	"github.com/oasisprotocol/oasis-indexer/config"
	"github.com/oasisprotocol/oasis-indexer/log"
	source "github.com/oasisprotocol/oasis-indexer/storage/oasis"
	fileSource "github.com/oasisprotocol/oasis-indexer/storage/oasis/nodeapi/file"
)

func TestRunFileBasedIndexer(t *testing.T) {
	if _, ok := os.LookupEnv("OASIS_INDEXER_API_TESTS"); !ok {
		t.Skip("skipping run of file-based indexer")
	}

	if _, err := os.Stat(DestFile); err != nil {
		t.Error("consensus node data not found")
	}

	ctx := context.Background()
	logger := log.NewDefaultLogger("api-tests")
	cfg, err := config.InitConfig(ConfigFile)
	require.Nil(t, err)

	// Initialize consensus analyzer.
	target, err := common.NewClient(cfg.Analysis.Storage, logger)
	require.Nil(t, err)
	defer target.Shutdown()
	// Always wipe storage and reindex
	err = target.Wipe(ctx)
	require.Nil(t, err)

	consensusAnalyzer, err := consensus.NewMain(cfg.Analysis.Node, cfg.Analysis.Analyzers.Consensus, target, logger)
	require.Nil(t, err)

	// Initialize file-based source storage.
	// TODO: rip this out once the config changes go in
	networkCfg := oasisConfig.Network{
		ChainContext: cfg.Analysis.Node.ChainContext,
		RPC:          cfg.Analysis.Node.RPC,
	}
	factory, err := source.NewClientFactory(ctx, &networkCfg, true)
	require.Nil(t, err)
	sourceClient, err := factory.Consensus()
	require.Nil(t, err)
	// We do not provide a damask connection to the fileBackend to
	// ensure that only file data is used.
	fileBackend, err := fileSource.NewFileConsensusApiLite(DestFile, nil)
	require.Nil(t, err)
	sourceClient.NodeApi = fileBackend
	consensusAnalyzer.Cfg.Source = sourceClient

	// Perform migrations.
	m, err := migrate.New(
		cfg.Analysis.Storage.Migrations,
		cfg.Analysis.Storage.Endpoint,
	)
	require.Nil(t, err)

	switch err = m.Up(); {
	case err == migrate.ErrNoChange:
		logger.Info("no migrations needed to be applied")
	case err != nil:
		require.Nil(t, err)
	default:
		logger.Info("migrations completed")
	}

	// Run analyzer
	consensusAnalyzer.Start()
	t.Log("file-backed analyzer finished processing blocks")
}
