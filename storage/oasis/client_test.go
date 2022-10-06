package oasis

import (
	"context"
	"os"
	"testing"

	"github.com/oasisprotocol/oasis-indexer/tests"
	config "github.com/oasisprotocol/oasis-sdk/client-sdk/go/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	pastHeight   = 8048956      // oasis-3 genesis height
	futureHeight = 100000000000 // an unreasonably high height
)

func newClientFactory() (*ClientFactory, error) {
	network := &config.Network{
		ChainContext: os.Getenv("CI_TEST_CHAIN_CONTEXT"),
		RPC:          os.Getenv("CI_TEST_NODE_RPC"),
	}
	return NewClientFactory(context.Background(), network)
}

func TestConnect(t *testing.T) {
	tests.SkipUnlessE2E(t)
	tests.SkipIfShort(t)

	_, err := newClientFactory()
	require.Nil(t, err)
}

func TestInvalidConnect(t *testing.T) {
	tests.SkipUnlessE2E(t)
	tests.SkipIfShort(t)

	network := &config.Network{
		ChainContext: os.Getenv("CI_TEST_CHAIN_CONTEXT"),
		RPC:          "an invalid rpc endpoint",
	}
	_, err := NewClientFactory(context.Background(), network)
	require.NotNil(t, err)

	network = &config.Network{
		ChainContext: "an invalid chaincontext",
		RPC:          os.Getenv("CI_TEST_NODE_RPC"),
	}
	_, err = NewClientFactory(context.Background(), network)
	require.NotNil(t, err)
}

func TestGenesisDocument(t *testing.T) {
	tests.SkipUnlessE2E(t)
	tests.SkipIfShort(t)

	ctx := context.Background()

	factory, err := newClientFactory()
	require.Nil(t, err)

	client, err := factory.Consensus()
	require.Nil(t, err)

	_, err = client.GenesisDocument(ctx)
	assert.Nil(t, err)
}

func TestBlockData(t *testing.T) {
	tests.SkipUnlessE2E(t)
	tests.SkipIfShort(t)

	ctx := context.Background()

	factory, err := newClientFactory()
	require.Nil(t, err)

	client, err := factory.Consensus()
	require.Nil(t, err)

	_, err = client.BlockData(ctx, pastHeight)
	require.Nil(t, err)

	_, err = client.BlockData(ctx, futureHeight)
	require.NotNil(t, err)
}

func TestBeaconData(t *testing.T) {
	tests.SkipUnlessE2E(t)
	tests.SkipIfShort(t)

	ctx := context.Background()

	factory, err := newClientFactory()
	require.Nil(t, err)

	client, err := factory.Consensus()
	require.Nil(t, err)

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

	factory, err := newClientFactory()
	require.Nil(t, err)

	client, err := factory.Consensus()
	require.Nil(t, err)

	_, err = client.RegistryData(ctx, pastHeight)
	require.Nil(t, err)

	_, err = client.RegistryData(ctx, futureHeight)
	require.NotNil(t, err)
}

func TestStakingData(t *testing.T) {
	tests.SkipUnlessE2E(t)
	tests.SkipIfShort(t)

	ctx := context.Background()

	factory, err := newClientFactory()
	require.Nil(t, err)

	client, err := factory.Consensus()
	require.Nil(t, err)

	_, err = client.StakingData(ctx, pastHeight)
	require.Nil(t, err)

	_, err = client.StakingData(ctx, futureHeight)
	require.NotNil(t, err)
}

func TestSchedulerData(t *testing.T) {
	tests.SkipUnlessE2E(t)
	tests.SkipIfShort(t)

	ctx := context.Background()

	factory, err := newClientFactory()
	require.Nil(t, err)

	client, err := factory.Consensus()
	require.Nil(t, err)

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

	factory, err := newClientFactory()
	require.Nil(t, err)

	client, err := factory.Consensus()
	require.Nil(t, err)

	_, err = client.GovernanceData(ctx, pastHeight)
	require.Nil(t, err)

	_, err = client.GovernanceData(ctx, futureHeight)
	require.NotNil(t, err)
}
