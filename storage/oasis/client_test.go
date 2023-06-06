package oasis

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/oasisprotocol/oasis-indexer/config"
	"github.com/oasisprotocol/oasis-indexer/tests"
)

const (
	pastHeight   = 8048956      // oasis-3 genesis height
	futureHeight = 100000000000 // an unreasonably high height
)

var TestSourceConfig = config.CustomSingleNetworkSourceConfig(
	os.Getenv("CI_TEST_NODE_RPC"),
	os.Getenv("CI_TEST_CHAIN_CONTEXT"),
)

func TestConnect(t *testing.T) {
	tests.SkipUnlessE2E(t)
	tests.SkipIfShort(t)

	_, err := NewConsensusClient(context.Background(), TestSourceConfig)
	require.NoError(t, err)
}

func TestInvalidConnect(t *testing.T) {
	tests.SkipUnlessE2E(t)
	tests.SkipIfShort(t)

	invalidRPCSourceConfig := config.CustomSingleNetworkSourceConfig(
		"an invalid rpc endpoint",
		os.Getenv("CI_TEST_CHAIN_CONTEXT"),
	)
	invalidRPCSourceConfig.FastStartup = false
	_, err := NewConsensusClient(context.Background(), invalidRPCSourceConfig)
	require.Error(t, err)

	invalidChainContextSourceConfig := config.CustomSingleNetworkSourceConfig(
		os.Getenv("CI_TEST_NODE_RPC"),
		"ci-test-invalid-chaincontext",
	)
	invalidChainContextSourceConfig.FastStartup = false
	_, err = NewConsensusClient(context.Background(), invalidChainContextSourceConfig)
	require.Error(t, err)
}

func TestGenesisDocument(t *testing.T) {
	tests.SkipUnlessE2E(t)
	tests.SkipIfShort(t)

	ctx := context.Background()

	client, err := NewConsensusClient(ctx, TestSourceConfig)
	require.NoError(t, err)

	_, err = client.GenesisDocument(ctx)
	require.Nil(t, err)
}

func TestBlockData(t *testing.T) {
	tests.SkipUnlessE2E(t)
	tests.SkipIfShort(t)

	ctx := context.Background()

	client, err := NewConsensusClient(ctx, TestSourceConfig)
	require.NoError(t, err)

	_, err = client.BlockData(ctx, pastHeight)
	require.Nil(t, err)

	_, err = client.BlockData(ctx, futureHeight)
	require.NotNil(t, err)
}

func TestBeaconData(t *testing.T) {
	tests.SkipUnlessE2E(t)
	tests.SkipIfShort(t)

	ctx := context.Background()

	client, err := NewConsensusClient(ctx, TestSourceConfig)
	require.NoError(t, err)

	_, err = client.BeaconData(ctx, pastHeight)
	require.NotNil(t, err)
	// ^beacon data was not available at this height

	_, err = client.BeaconData(ctx, futureHeight)
	require.Nil(t, err)
	// ^beacon value is known for future heights
}

func TestRegistryData(t *testing.T) {
	tests.SkipUnlessE2E(t)
	tests.SkipIfShort(t)

	ctx := context.Background()

	client, err := NewConsensusClient(ctx, TestSourceConfig)
	require.NoError(t, err)

	_, err = client.RegistryData(ctx, pastHeight)
	require.Nil(t, err)

	_, err = client.RegistryData(ctx, futureHeight)
	require.NotNil(t, err)
}

func TestStakingData(t *testing.T) {
	tests.SkipUnlessE2E(t)
	tests.SkipIfShort(t)

	ctx := context.Background()

	client, err := NewConsensusClient(ctx, TestSourceConfig)
	require.NoError(t, err)

	_, err = client.StakingData(ctx, pastHeight)
	require.Nil(t, err)

	_, err = client.StakingData(ctx, futureHeight)
	require.NotNil(t, err)
}

func TestSchedulerData(t *testing.T) {
	tests.SkipUnlessE2E(t)
	tests.SkipIfShort(t)

	ctx := context.Background()

	client, err := NewConsensusClient(ctx, TestSourceConfig)
	require.NoError(t, err)

	_, err = client.SchedulerData(ctx, pastHeight)
	require.Nil(t, err)

	_, err = client.SchedulerData(ctx, futureHeight)
	require.Nil(t, err)
	// ^the scheduler returns the list of current validators
	// for future heights as well, so this does not fail
}

func TestGovernanceData(t *testing.T) {
	tests.SkipUnlessE2E(t)
	tests.SkipIfShort(t)

	ctx := context.Background()

	client, err := NewConsensusClient(ctx, TestSourceConfig)
	require.NoError(t, err)

	_, err = client.GovernanceData(ctx, pastHeight)
	require.Nil(t, err)

	_, err = client.GovernanceData(ctx, futureHeight)
	require.NotNil(t, err)
}
