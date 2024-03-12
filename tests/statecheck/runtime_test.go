package statecheck

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	sdkConfig "github.com/oasisprotocol/oasis-sdk/client-sdk/go/config"
	sdkTypes "github.com/oasisprotocol/oasis-sdk/client-sdk/go/types"

	"github.com/oasisprotocol/nexus/analyzer/runtime/evm"
	common "github.com/oasisprotocol/nexus/common"
)

var RuntimeTables = []string{"runtime_sdk_balances"}

type TestRuntimeAccount struct {
	Address string
	Balance common.BigInt
	Symbol  string
}

func TestEmeraldAccounts(t *testing.T) {
	testRuntimeAccounts(t, common.RuntimeEmerald)
}

func TestSapphireAccounts(t *testing.T) {
	testRuntimeAccounts(t, common.RuntimeSapphire)
}

func testRuntimeAccounts(t *testing.T, runtime common.Runtime) {
	if _, ok := os.LookupEnv("OASIS_INDEXER_HEALTHCHECK"); !ok {
		t.Skip("skipping test since healthcheck tests are not enabled")
	}

	schema := fmt.Sprintf("snapshot_%s", runtime)

	t.Log("Initializing data stores...")

	ctx := context.Background()

	sdkNet := sdkConfig.DefaultNetworks.All[networkFromChainContext(os.Getenv("HEALTHCHECK_TEST_CHAIN_CONTEXT"))]
	sdkPT := sdkNet.ParaTimes.All[string(runtime)]
	t.Log("Runtime ID determined", "runtime", runtime, "runtime_id", sdkPT.ID)

	conn, err := newSdkConnection(ctx)
	require.NoError(t, err)
	oasisRuntimeClient := conn.Runtime(sdkPT)

	postgresClient, err := newTargetClient(t)
	require.NoError(t, err)

	t.Log("Creating snapshot for runtime tables...")
	height, err := snapshotBackends(postgresClient, string(runtime), RuntimeTables)
	require.NoError(t, err)

	t.Logf("Fetching accounts information at height %d...", height)
	addresses, err := oasisRuntimeClient.Accounts.Addresses(ctx, uint64(height), sdkTypes.NativeDenomination)
	require.NoError(t, err)
	expectedAccts := make(map[sdkTypes.Address]bool)
	for _, addr := range addresses {
		expectedAccts[addr] = true
	}
	t.Logf("Fetched %d account addresses", len(addresses))

	// Fetch addresses for which native balance is known to be stale.
	// These are balances that we expect them to be invalid, so exempt them from the following test.
	exemptRows, err := postgresClient.Query(ctx, fmt.Sprintf(`
		SELECT account_address
		FROM %s.stale_balances
		WHERE runtime=$1 AND token_address=$2
	`, schema), runtime, evm.NativeRuntimeTokenAddress)
	require.NoError(t, err)
	exemptAccs := make(map[string]bool)
	defer exemptRows.Close()
	for exemptRows.Next() {
		var addr string
		err = exemptRows.Scan(&addr)
		require.NoError(t, err)
		exemptAccs[addr] = true
	}
	t.Logf("Found %d exempted accounts (skipped for the test)", len(exemptAccs))

	// Fetch account balances.
	acctRows, err := postgresClient.Query(ctx, fmt.Sprintf(`
		SELECT account_address, balance, symbol
		FROM %s.runtime_sdk_balances
		WHERE runtime=$1
	`, schema), runtime)
	require.NoError(t, err)
	defer acctRows.Close()
	actualAccts := make(map[string]bool)
	var allBalances uint64
	var balanceDiscrepancies uint64
	var notExpectedFound uint64

	// Check that the account balances are accurate.
	for acctRows.Next() {
		var a TestRuntimeAccount
		err = acctRows.Scan(
			&a.Address,
			&a.Balance,
			&a.Symbol,
		)
		require.NoError(t, err)

		// Check that the account exists.
		var actualAddr sdkTypes.Address
		err = actualAddr.UnmarshalText([]byte(a.Address))
		require.NoError(t, err)

		// Skip accounts that are exempted.
		if _, ok := exemptAccs[a.Address]; ok {
			actualAccts[a.Address] = true
			continue
		}

		_, ok := expectedAccts[actualAddr]
		if !ok {
			// If the account is not expected (missing from oasis-node) it should have a zero balance.
			if !a.Balance.IsZero() {
				notExpectedFound++
				t.Logf("Unexpected address '%s' found, reported balance (Nexus): %s", a.Address, a.Balance)
				t.Fail()
			} else {
				allBalances++
			}
			continue
		}

		// Check that the account balance is accurate.
		balances, err := oasisRuntimeClient.Accounts.Balances(ctx, uint64(height), actualAddr)
		require.NoError(t, err)
		for denom, amount := range balances.Balances {
			if stringifyDenomination(denom, sdkPT) == a.Symbol {
				allBalances++

				if !a.Balance.Eq(common.BigIntFromQuantity(amount)) {
					t.Errorf("Unexpected %s balance for address '%s': expected (node) %s, actual (Nexus) %s",
						stringifyDenomination(denom, sdkPT), a.Address, amount.String(), a.Balance.String())
					balanceDiscrepancies++
				}
			}
		}
		actualAccts[a.Address] = true
	}

	// Check addresses that are expected, but were not present in the snapshot.
	var expectedNotFound uint64
	var expectedNotFoundNonZero uint64
	for _, addr := range addresses {
		_, ok := actualAccts[addr.String()]
		if !ok {
			t.Logf("Expected address '%s' not found", addr.String())
			expectedNotFound++
			t.Fail()
			balances, err := oasisRuntimeClient.Accounts.Balances(ctx, uint64(height), addr)
			assert.NoError(t, err)
			for denom, amount := range balances.Balances {
				if stringifyDenomination(denom, sdkPT) == nativeTokenSymbol(sdkPT) {
					t.Logf("Balance: %s", amount)
					if !amount.IsZero() {
						expectedNotFoundNonZero++
					}
				}
			}
		}
	}

	// Report results.
	t.Logf(`Number of discrepancies in account balances: %d (out of: %d).
 		- Does not include accounts that are only listed in Nexus or on the chain.
		- Exempted accounts because of known stale balance in Nexus: %d
	`, balanceDiscrepancies, allBalances, len(exemptAccs))
	t.Logf("Number of unexpected addresses found in Nexus: %d", notExpectedFound)
	t.Logf("Number of expected addresses not found in Nexus: %d (with non-zero balance: %d)", expectedNotFound, expectedNotFoundNonZero)
}

func nativeTokenSymbol(sdkPT *sdkConfig.ParaTime) string {
	return sdkPT.Denominations[sdkConfig.NativeDenominationKey].Symbol
}

// StringifyDenomination returns a string representation denomination `d`
// in the context of `runtimeID`. The context matters for the native denomination.
func stringifyDenomination(d sdkTypes.Denomination, sdkPT *sdkConfig.ParaTime) string {
	if d.IsNative() {
		return nativeTokenSymbol(sdkPT)
	}

	return d.String()
}
