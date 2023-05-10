package analyzer_test

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/oasisprotocol/oasis-indexer/cmd/analyzer"
	"github.com/oasisprotocol/oasis-indexer/storage/postgres/testutil"
	"github.com/oasisprotocol/oasis-indexer/tests"
)

// Relative path to the migrations directory when running tests in this file.
// When running go tests, the working directory is always set to the package directory of the test being run.
const migrationsPath = "file://../../storage/migrations"

func TestMigrations(t *testing.T) {
	tests.SkipIfShort(t)
	client := testutil.NewTestClient(t)
	defer client.Close()

	ctx := context.Background()

	// Ensure database is empty before running migrations.
	require.NoError(t, client.Wipe(ctx), "failed to wipe database")

	// Run migrations.
	require.NoError(t, analyzer.RunMigrations(migrationsPath, os.Getenv("CI_TEST_CONN_STRING")), "failed to run migrations")
}
