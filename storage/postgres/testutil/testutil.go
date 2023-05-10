package testutil

import (
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/oasisprotocol/oasis-indexer/log"
	"github.com/oasisprotocol/oasis-indexer/storage/postgres"
)

// NewTestClient returns a postgres client used in CI tests.
func NewTestClient(t *testing.T) *postgres.Client {
	connString := os.Getenv("CI_TEST_CONN_STRING")
	logger, err := log.NewLogger("postgres-test", io.Discard, log.FmtJSON, log.LevelInfo)
	require.Nil(t, err, "log.NewLogger")

	client, err := postgres.NewClient(connString, logger)
	require.Nil(t, err, "postgres.NewClient")
	return client
}
