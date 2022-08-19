package v1

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	v1 "github.com/oasisprotocol/oasis-indexer/api/v1"
	"github.com/oasisprotocol/oasis-indexer/tests"
)

func TestGetStatus(t *testing.T) {
	if _, ok := os.LookupEnv("OASIS_INDEXER_E2E"); !ok {
		t.Skip("skipping test since e2e tests are not enabled")
	}

	tests.Init()

	<-tests.After(tests.GenesisHeight)

	var status v1.Status
	err := tests.GetFrom("/", &status)
	require.Nil(t, err)

	require.Equal(t, tests.ChainID, status.LatestChainID)
	require.LessOrEqual(t, tests.GenesisHeight, status.LatestBlock)
}
