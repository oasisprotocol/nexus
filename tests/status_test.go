package tests

import (
	"testing"
	"time"

	v1 "github.com/oasislabs/oasis-indexer/api/v1"
	"github.com/stretchr/testify/require"
)

// indexBuffer is a buffer to sanity check that the last block
// has been processed in a timely fashion. For now it's set
// to a very, very generous value.
var indexBuffer = time.Duration(-1) * time.Minute

func TestGetStatus(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping testing in short mode")
	}

	Init()

	<-After(GenesisHeight)

	start := time.Now()

	var status v1.Status
	GetFrom("/", &status)

	require.Equal(t, ChainID, status.LatestChainID)
	require.LessOrEqual(t, GenesisHeight, status.LatestBlock)
	require.LessOrEqual(t, start.Add(indexBuffer).Unix(), status.LatestUpdate.Unix())
}
