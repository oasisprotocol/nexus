package analyzer_test

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	cmdAnalyzer "github.com/oasisprotocol/nexus/cmd/analyzer"
	"github.com/oasisprotocol/nexus/storage/postgres/testutil"
	"github.com/oasisprotocol/nexus/tests"
)

func TestMigrations(t *testing.T) {
	tests.SkipIfShort(t)
	client := testutil.NewTestClient(t)
	defer client.Close()

	ctx := context.Background()

	// Ensure database is empty before running migrations.
	require.NoError(t, client.Wipe(ctx), "failed to wipe database")

	// Run migrations.
	require.NoError(t, cmdAnalyzer.RunMigrations(os.Getenv("CI_TEST_CONN_STRING")), "failed to run migrations")
}
