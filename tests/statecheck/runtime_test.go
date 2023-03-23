package statecheck

import (
	"context"
	"os"
	"testing"

	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/config"
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

	network, err := analyzer.FromChainContext(MainnetChainContext)
	require.Nil(t, err)

	var runtimeID string
	switch runtime {
	case "emerald":
		runtimeID, err = analyzer.RuntimeEmerald.ID(network)
	case "sapphire":
		runtimeID, err = analyzer.RuntimeSapphire.ID(network)
	}
	require.Nil(t, err)
	t.Log("Runtime ID determined", "runtime", runtime, "runtime_id", runtimeID)

	conn, err := newSdkConnection(ctx)
	require.Nil(t, err)
	oasisRuntimeClient := conn.Runtime(&config.ParaTime{
		ID: runtimeID,
	})

	postgresClient, err := newTargetClient(t)
	assert.Nil(t, err)

	t.Log("Creating snapshot for runtime tables...")
	height, err := snapshotBackends(postgresClient, runtime, RuntimeTables)
	assert.Nil(t, err)

	t.Logf("Fetching accounts information at height %d...", height)
	addresses, err := oasisRuntimeClient.Accounts.Addresses(ctx, uint64(height), sdkTypes.NativeDenomination)
	assert.Nil(t, err)
	expectedAccts := make(map[sdkTypes.Address]bool)
	for _, addr := range addresses {
		expectedAccts[addr] = true
	}
	t.Logf("Fetched %d account addresses", len(addresses))

	acctRows, err := postgresClient.Query(ctx,
		`SELECT account_address, balance, symbol
			FROM snapshot.runtime_sdk_balances
			WHERE runtime=$1`, runtime,
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
		balances, err := oasisRuntimeClient.Accounts.Balances(ctx, uint64(height), actualAddr)
		assert.Nil(t, err)
		for denom, amount := range balances.Balances {
			if stringifyDenomination(denom, runtimeID) == a.Symbol {
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

func nativeTokenSymbol(runtimeID string) string {
	// Iterate over all networks and find the one that contains the runtime.
	// Any network will do; we assume that paratime IDs are unique across networks.
	for _, network := range config.DefaultNetworks.All {
		for _, paratime := range network.ParaTimes.All {
			if paratime.ID == runtimeID {
				return paratime.Denominations[config.NativeDenominationKey].Symbol
			}
		}
	}
	panic("Cannot find native token symbol for runtime")
}

// StringifyDenomination returns a string representation denomination `d`
// in the context of `runtimeID`. The context matters for the native denomination.
func stringifyDenomination(d sdkTypes.Denomination, runtimeID string) string {
	if d.IsNative() {
		return nativeTokenSymbol(runtimeID)
	}

	return d.String()
}
