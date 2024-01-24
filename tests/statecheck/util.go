package statecheck

import (
	"context"
	"fmt"
	"io"
	"os"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"

	oasisConfig "github.com/oasisprotocol/oasis-sdk/client-sdk/go/config"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/connection"

	"github.com/oasisprotocol/nexus/common"
	"github.com/oasisprotocol/nexus/log"
	"github.com/oasisprotocol/nexus/storage"
	"github.com/oasisprotocol/nexus/storage/postgres"
)

const (
	ChainName = common.ChainNameMainnet
)

func newTargetClient(t *testing.T) (*postgres.Client, error) {
	connString := os.Getenv("HEALTHCHECK_TEST_CONN_STRING")
	logger, err := log.NewLogger("db-test", io.Discard, log.FmtJSON, log.LevelInfo)
	require.NoError(t, err)

	return postgres.NewClient(connString, logger)
}

func newSdkConnection(ctx context.Context) (connection.Connection, error) {
	net := &oasisConfig.Network{
		ChainContext: os.Getenv("HEALTHCHECK_TEST_CHAIN_CONTEXT"),
		RPC:          os.Getenv("HEALTHCHECK_TEST_NODE_RPC"),
	}
	return connection.ConnectNoVerify(ctx, net)
}

func snapshotBackends(target *postgres.Client, analyzer string, tables []string) (int64, error) {
	ctx := context.Background()

	schema := fmt.Sprintf("snapshot_%s", analyzer)

	batch := &storage.QueryBatch{}
	batch.Queue(fmt.Sprintf(`CREATE SCHEMA IF NOT EXISTS %s;`, schema))
	for _, t := range tables {
		batch.Queue(fmt.Sprintf(`
			DROP TABLE IF EXISTS %s.%s CASCADE;
		`, schema, t))
		batch.Queue(fmt.Sprintf(`
			CREATE TABLE %s.%[2]s AS TABLE chain.%[2]s;
		`, schema, t))
	}

	// Store the height at which the snapshot was taken.
	batch.Queue(fmt.Sprintf(`
		CREATE TEMP TABLE %s_meta (height UINT63 NOT NULL);
	`, schema))
	batch.Queue(fmt.Sprintf(`
		INSERT INTO %s_meta (height)
			SELECT height
			FROM analysis.processed_blocks
			WHERE analyzer=$1 AND processed_time IS NOT NULL
			ORDER BY height DESC LIMIT 1;
	`, schema), analyzer)

	// Store balances that are known to be stale - so that the statecheck tests can avoid comparing those.
	batch.Queue(fmt.Sprintf(`
		DROP TABLE IF EXISTS %s.stale_balances CASCADE;
	`, schema))
	batch.Queue(fmt.Sprintf(`
		CREATE TABLE %s.stale_balances AS
		SELECT * FROM analysis.evm_token_balances
		WHERE last_download_round IS NULL OR last_download_round < last_mutate_round;
	`, schema))

	// Create the snapshot using a high level of isolation; we don't want another
	// tx to be able to modify the tables while this is running, creating a snapshot that
	// represents analyzer state at two (or more) blockchain heights.
	if err := target.SendBatchWithOptions(ctx, batch, pgx.TxOptions{IsoLevel: pgx.Serializable}); err != nil {
		return 0, err
	}

	// Report snapshotted height.
	var snapshotHeight int64
	if err := target.QueryRow(ctx, fmt.Sprintf(`
		SELECT height from %s_meta
			ORDER BY height DESC LIMIT 1;
	`, schema)).Scan(&snapshotHeight); err != nil {
		return 0, err
	}

	return snapshotHeight, nil
}
