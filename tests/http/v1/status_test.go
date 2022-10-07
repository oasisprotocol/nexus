package v1

import (
	"testing"

	"github.com/stretchr/testify/require"

	storage "github.com/oasisprotocol/oasis-indexer/storage/client"
	"github.com/oasisprotocol/oasis-indexer/tests"
)

func TestGetStatus(t *testing.T) {
	tests.SkipUnlessE2E(t)

	tests.Init()

	<-tests.After(tests.GenesisHeight)

	var status storage.Status
	err := tests.GetFrom("/", &status)
	require.Nil(t, err)

	require.Equal(t, tests.ChainID, status.LatestChainID)
	require.LessOrEqual(t, tests.GenesisHeight, status.LatestBlock)
}
