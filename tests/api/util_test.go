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
	connection "github.com/oasisprotocol/oasis-sdk/client-sdk/go/connection"
	"github.com/stretchr/testify/require"

	"github.com/oasisprotocol/oasis-indexer/analyzer/consensus"
	"github.com/oasisprotocol/oasis-indexer/cmd/common"
	"github.com/oasisprotocol/oasis-indexer/config"
	"github.com/oasisprotocol/oasis-indexer/log"
	source "github.com/oasisprotocol/oasis-indexer/storage/oasis"
	fileSource "github.com/oasisprotocol/oasis-indexer/storage/oasis/nodeapi/file"
)

const ConfigFile = "/Users/andrewlow/oasis/oasis-block-indexer/tests/api/api-dev.yml"
const DestFile = "/Users/andrewlow/oasis/oasis-block-indexer/tests/api/data/consensus"

func TestDumpConsensusNodeData(t *testing.T) {
	if _, ok := os.LookupEnv("OASIS_INDEXER_DUMP_NODE_DATA"); !ok {
		t.Skip("skipping dump of node data")
	}

	ctx := context.Background()
	logger := log.NewDefaultLogger("api-tests")
	cfg, err := config.InitConfig(ConfigFile)
	require.Nil(t, err)

	// Initialize and wipe target storage. Note that if we do not
	// wipe storage, the db may already have synced past the endHeight
	// and the indexer will not fetch data from the node.
	target, err := common.NewClient(cfg.Analysis.Storage, logger)
	require.Nil(t, err)
	defer target.Shutdown()
	err = target.Wipe(ctx)
	require.Nil(t, err)

	// Initialize consensus analyzer.
	consensusAnalyzer, err := consensus.NewMain(cfg.Analysis.Node, cfg.Analysis.Analyzers.Consensus, target, logger)

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
	conn, err := connection.ConnectNoVerify(ctx, &networkCfg)
	require.Nil(t, err)
	consensusConn := conn.Consensus()
	fileBackend, err := fileSource.NewFileConsensusApiLite(DestFile, &consensusConn)
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
	t.Log("finished dumping consensus node data to " + DestFile)
}
