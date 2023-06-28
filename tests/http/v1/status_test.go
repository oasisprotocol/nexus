package v1

import (
	"testing"

	"github.com/stretchr/testify/require"

	storage "github.com/oasisprotocol/nexus/storage/client"
	"github.com/oasisprotocol/nexus/tests"
)

func TestGetStatus(t *testing.T) {
	tests.SkipUnlessE2E(t)

	tests.Init()

	<-tests.After(tests.GenesisHeight)

	var status storage.Status
	err := tests.GetFrom("/", &status)
	require.Nil(t, err)

	require.LessOrEqual(t, tests.GenesisHeight, status.LatestBlock)
}
