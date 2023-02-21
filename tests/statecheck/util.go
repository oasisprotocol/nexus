package statecheck

import (
	"context"
	"fmt"
	"io"
	"os"
	"testing"

	"github.com/iancoleman/strcase"
	"github.com/jackc/pgx/v4"
	"github.com/oasisprotocol/oasis-indexer/log"
	"github.com/oasisprotocol/oasis-indexer/storage"
	"github.com/oasisprotocol/oasis-indexer/storage/oasis"
	"github.com/oasisprotocol/oasis-indexer/storage/postgres"
	oasisConfig "github.com/oasisprotocol/oasis-sdk/client-sdk/go/config"
	"github.com/stretchr/testify/assert"
)

const (
	MainnetChainContext = "b11b369e0da5bb230b220127f5e7b242d385ef8c6f54906243f30af63c815535"
)

func newTargetClient(t *testing.T) (*postgres.Client, error) {
	connString := os.Getenv("HEALTHCHECK_TEST_CONN_STRING")
	logger, err := log.NewLogger("db-test", io.Discard, log.FmtJSON, log.LevelInfo)
	assert.Nil(t, err)

	return postgres.NewClient(connString, logger)
}

func newSourceClientFactory() (*oasis.ClientFactory, error) {
	network := &oasisConfig.Network{
		ChainContext: os.Getenv("HEALTHCHECK_TEST_CHAIN_CONTEXT"),
		RPC:          os.Getenv("HEALTHCHECK_TEST_NODE_RPC"),
	}
	return oasis.NewClientFactory(context.Background(), network, true)
}

var chainID = "" // Memoization for getChainId(). Assumes all tests access the same chain.
func getChainID(ctx context.Context, t *testing.T, source *oasis.ConsensusClient) string {
	if chainID == "" {
		doc, err := source.GenesisDocument(ctx)
		assert.Nil(t, err)
		chainID = strcase.ToSnake(doc.ChainID)
	}
	return chainID
}

func checkpointBackends(t *testing.T, consensusClient *oasis.ConsensusClient, target *postgres.Client, analyzer string, tables []string) (int64, error) {
	ctx := context.Background()
	chainID = getChainID(ctx, t, consensusClient)

	batch := &storage.QueryBatch{}
	for _, t := range tables {
		batch.Queue(fmt.Sprintf(`
			DROP TABLE IF EXISTS %s.%s_checkpoint CASCADE;
		`, chainID, t))
		batch.Queue(fmt.Sprintf(`
			CREATE TABLE %s.%s_checkpoint AS TABLE %s.%s;
		`, chainID, t, chainID, t))
	}
	batch.Queue(fmt.Sprintf(`
		INSERT INTO %s.checkpointed_heights (analyzer, height)
			SELECT analyzer, height FROM %s.processed_blocks WHERE analyzer='%s' ORDER BY height DESC, processed_time DESC LIMIT 1
			ON CONFLICT DO NOTHING;
	`, chainID, chainID, analyzer))

	// Create the snapshot using a high level of isolation; we don't want another
	// tx to be able to modify the tables while this is running, creating a snapshot that
	// represents indexer state at two (or more) blockchain heights.
	if err := target.SendBatchWithOptions(ctx, batch, pgx.TxOptions{IsoLevel: pgx.Serializable}); err != nil {
		return 0, err
	}

	var checkpointHeight int64
	if err := target.QueryRow(ctx, fmt.Sprintf(`
		SELECT height from %s.checkpointed_heights
			WHERE analyzer='%s'
			ORDER BY height DESC LIMIT 1;
	`, chainID, analyzer)).Scan(&checkpointHeight); err != nil {
		return 0, err
	}

	return checkpointHeight, nil
}
