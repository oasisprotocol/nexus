package statecheck

import (
	"context"
	"fmt"
	"os"
	"testing"

	sdkTypes "github.com/oasisprotocol/oasis-sdk/client-sdk/go/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/oasisprotocol/oasis-indexer/analyzer"
	common "github.com/oasisprotocol/oasis-indexer/common"
)

const (
	EmeraldName = "emerald"
)

var RuntimeTables = []string{"runtime_sdk_balances"}

type TestRuntimeAccount struct {
	Address string
	Balance common.BigInt
	Symbol  string
}

func TestEmeraldAccounts(t *testing.T) {
	testRuntimeAccounts(t, EmeraldName)
}

func testRuntimeAccounts(t *testing.T, runtime string) {
	if _, ok := os.LookupEnv("OASIS_INDEXER_HEALTHCHECK"); !ok {
		t.Skip("skipping test since healthcheck tests are not enabled")
	}

	t.Log("Initializing data stores...")

	ctx := context.Background()

	factory, err := newSourceClientFactory()
	require.Nil(t, err)

	oasisConsensusClient, err := factory.Consensus()
	require.Nil(t, err)

	network, err := analyzer.FromChainContext(MainnetChainContext)
	require.Nil(t, err)

	var id string
	switch runtime {
	case "emerald":
		id, err = analyzer.RuntimeEmerald.ID(network)
	case "sapphire":
		id, err = analyzer.RuntimeSapphire.ID(network)
	}
	require.Nil(t, err)
	t.Log("Runtime ID determined", "runtime", runtime, "runtime_id", id)

	oasisRuntimeClient, err := factory.Runtime(id)
	require.Nil(t, err)

	postgresClient, err := newTargetClient(t)
	assert.Nil(t, err)

	t.Log("Creating checkpoint for runtime tables...")
	height, err := checkpointBackends(t, oasisConsensusClient, postgresClient, runtime, RuntimeTables)
	assert.Nil(t, err)

	t.Logf("Fetching accounts information at height %d...", height)
	addresses, err := oasisRuntimeClient.GetAccountAddresses(ctx, uint64(height), sdkTypes.NativeDenomination)
	assert.Nil(t, err)
	expectedAccts := make(map[sdkTypes.Address]bool)
	for _, addr := range addresses {
		expectedAccts[addr] = true
	}
	t.Logf("Fetched %d account addresses", len(addresses))

	acctRows, err := postgresClient.Query(ctx, fmt.Sprintf(
		`SELECT account_address, balance, symbol
			FROM %s.runtime_sdk_balances_checkpoint
			WHERE runtime='%s'`, chainID, runtime),
	)
	require.Nil(t, err)
	actualAccts := make(map[string]bool)
	for acctRows.Next() {
		var a TestRuntimeAccount
		err = acctRows.Scan(
			&a.Address,
			&a.Balance,
			&a.Symbol,
		)
		assert.Nil(t, err)

		// Check that the account exists.
		var actualAddr sdkTypes.Address
		err = actualAddr.UnmarshalText([]byte(a.Address))
		assert.Nil(t, err)
		_, ok := expectedAccts[actualAddr]
		if !ok {
			t.Logf("address %s found, but not expected", a.Address)
			t.Fail()
			continue
		}

		// Check that the account balance is accurate.
		balances, err := oasisRuntimeClient.GetAccountBalances(ctx, uint64(height), actualAddr)
		assert.Nil(t, err)
		for denom, amount := range balances.Balances {
			if oasisRuntimeClient.StringifyDenomination(denom) == a.Symbol {
				assert.Equal(t, amount.ToBigInt(), a.Balance)
			}
			assert.Equal(t, amount.ToBigInt().Int64(), a.Balance)
		}
		actualAccts[a.Address] = true
	}

	for _, addr := range addresses {
		_, ok := actualAccts[addr.String()]
		if !ok {
			t.Logf("address %s expected, but not found", addr.String())
			t.Fail()
		}
	}
}
