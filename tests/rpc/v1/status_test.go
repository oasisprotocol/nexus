package v1

import (
	"testing"

	"github.com/stretchr/testify/require"

	v1 "github.com/oasislabs/oasis-indexer/api/v1"
	"github.com/oasislabs/oasis-indexer/tests"
)

func TestGetStatus(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping testing in short mode")
	}

	tests.Init()

	<-tests.After(tests.GenesisHeight)

	var status v1.Status
	err := tests.GetFrom("/", &status)
	require.Nil(t, err)

	require.Equal(t, tests.ChainID, status.LatestChainID)
	require.LessOrEqual(t, tests.GenesisHeight, status.LatestBlock)
}
