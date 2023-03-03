package statecheck

import (
	"context"
	"fmt"
	"io"
	"os"
	"testing"

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

func snapshotBackends(target *postgres.Client, analyzer string, tables []string) (int64, error) {
	ctx := context.Background()

	batch := &storage.QueryBatch{}
	batch.Queue(`CREATE SCHEMA IF NOT EXISTS snapshot;`)
	for _, t := range tables {
		batch.Queue(fmt.Sprintf(`
			DROP TABLE IF EXISTS snapshot.%s CASCADE;
		`, t))
		batch.Queue(fmt.Sprintf(`
			CREATE TABLE snapshot.%[1]s AS TABLE chain.%[1]s;
		`, t))
	}
	batch.Queue(`
		INSERT INTO snapshot.snapshotted_heights (analyzer, height)
			SELECT analyzer, height FROM chain.processed_blocks WHERE analyzer=$1 ORDER BY height DESC, processed_time DESC LIMIT 1
			ON CONFLICT DO NOTHING;
	`, analyzer)

	// Create the snapshot using a high level of isolation; we don't want another
	// tx to be able to modify the tables while this is running, creating a snapshot that
	// represents indexer state at two (or more) blockchain heights.
	if err := target.SendBatchWithOptions(ctx, batch, pgx.TxOptions{IsoLevel: pgx.Serializable}); err != nil {
		return 0, err
	}

	var snapshotHeight int64
	if err := target.QueryRow(ctx, `
		SELECT height from snapshot.snapshotted_heights
			WHERE analyzer=$1
			ORDER BY height DESC LIMIT 1;
	`, analyzer).Scan(&snapshotHeight); err != nil {
		return 0, err
	}

	return snapshotHeight, nil
}
