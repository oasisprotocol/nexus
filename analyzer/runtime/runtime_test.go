package runtime_test

// These tests exercise the runtime block analyzer.
// They are
//  - More high-level than unit tests; they test the behavior of the analyzer as a whole, including its interactions with the database.
//  - More low-level than end-to-end tests; they do not require a node or test the nexus API.
//  - More controllable than our regression e2e tests. Inputs blocks, txs, events etc are defined manually by each test.

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common/math"
	"github.com/stretchr/testify/require"

	"github.com/oasisprotocol/oasis-core/go/common/cbor"
	"github.com/oasisprotocol/oasis-core/go/common/quantity"
	"github.com/oasisprotocol/oasis-core/go/staking/api"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/modules/consensusaccounts"
	sdkTesting "github.com/oasisprotocol/oasis-sdk/client-sdk/go/testing"
	sdkTypes "github.com/oasisprotocol/oasis-sdk/client-sdk/go/types"

	"github.com/oasisprotocol/nexus/analyzer"
	"github.com/oasisprotocol/nexus/analyzer/runtime"
	"github.com/oasisprotocol/nexus/analyzer/util"
	analyzerCmd "github.com/oasisprotocol/nexus/cmd/analyzer"
	"github.com/oasisprotocol/nexus/common"
	"github.com/oasisprotocol/nexus/config"
	"github.com/oasisprotocol/nexus/log"
	"github.com/oasisprotocol/nexus/storage/oasis/nodeapi"
	"github.com/oasisprotocol/nexus/storage/postgres"
	pgTestUtil "github.com/oasisprotocol/nexus/storage/postgres/testutil"
)

// Relative path to the migrations directory when running tests in this file.
// When running go tests, the working directory is always set to the package directory of the test being run.
const migrationsPath = "file://../../storage/migrations"

const testsTimeout = 10 * time.Second

func setupDB(t *testing.T) *postgres.Client {
	ctx := context.Background()

	// Initialize the test database.
	testDB := pgTestUtil.NewTestClient(t)
	// Ensure the test database is empty.
	require.NoError(t, testDB.Wipe(ctx), "testDb.Wipe")
	// Run DB migrations.
	require.NoError(t, analyzerCmd.RunMigrations(migrationsPath, os.Getenv("CI_TEST_CONN_STRING")), "failed to run migrations")
	// Initialize test table.
	// batch := &storage.QueryBatch{}
	// batch.Queue(testTableCreate)
	// batch.Queue(grantSelectOnAnalysis)
	// batch.Queue(grantExecuteOnAnalysis)
	// require.NoError(t, testDB.SendBatch(ctx, batch), "failed to create test table")

	return testDB
}

// A mock implementation of RuntimeApiLite that returns predefined data.
type mockNode struct {
	Txs         map[uint64][]nodeapi.RuntimeTransactionWithResults // round -> txs
	NonTxEvents map[uint64][]nodeapi.RuntimeEvent                  // round -> events
}

var _ nodeapi.RuntimeApiLite = (*mockNode)(nil)

// Close implements nodeapi.RuntimeApiLite.
func (*mockNode) Close() error {
	return nil
}

// EVMGetCode implements nodeapi.RuntimeApiLite.
func (*mockNode) EVMGetCode(ctx context.Context, round uint64, address []byte) ([]byte, error) {
	panic("unimplemented") // not needed for testing the block analyzer
}

// EVMSimulateCall implements nodeapi.RuntimeApiLite.
func (*mockNode) EVMSimulateCall(ctx context.Context, round uint64, gasPrice []byte, gasLimit uint64, caller []byte, address []byte, value []byte, data []byte) (*nodeapi.FallibleResponse, error) {
	panic("unimplemented") // not needed for testing the block analyzer
}

// GetBlockHeader implements nodeapi.RuntimeApiLite.
func (*mockNode) GetBlockHeader(ctx context.Context, round uint64) (*nodeapi.RuntimeBlockHeader, error) {
	return &nodeapi.RuntimeBlockHeader{
		Round: round,
	}, nil
}

// GetEventsRaw implements nodeapi.RuntimeApiLite.
func (mock *mockNode) GetEventsRaw(ctx context.Context, round uint64) ([]nodeapi.RuntimeEvent, error) {
	events := mock.NonTxEvents[round]

	// Include events that were part of transactions.
	txrs := mock.Txs[round]
	for _, tx := range txrs {
		for _, ev := range tx.Events {
			events = append(events, nodeapi.RuntimeEvent(*ev))
		}
	}

	return events, nil
}

// GetBalances implements nodeapi.RuntimeApiLite.
func (*mockNode) GetBalances(ctx context.Context, round uint64, addr api.Address) (map[sdkTypes.Denomination]common.BigInt, error) {
	panic("unimplemented") // not needed for testing the block analyzer
}

// GetRoflApps implements nodeapi.RuntimeApiLite.
func (*mockNode) RoflApps(ctx context.Context, round uint64) ([]*nodeapi.AppConfig, error) {
	panic("unimplemented") // not needed for testing the block analyzer
}

// GetRoflApp implements nodeapi.RuntimeApiLite.
func (*mockNode) RoflApp(ctx context.Context, round uint64, id nodeapi.AppID) (*nodeapi.AppConfig, error) {
	panic("unimplemented") // not needed for testing the block analyzer
}

// GetRoflAppInstances implements nodeapi.RuntimeApiLite.
func (*mockNode) RoflAppInstances(ctx context.Context, round uint64, id nodeapi.AppID) ([]*nodeapi.Registration, error) {
	panic("unimplemented") // not needed for testing the block analyzer
}

func (*mockNode) RoflAppInstance(ctx context.Context, round uint64, id nodeapi.AppID, rak nodeapi.PublicKey) (*nodeapi.Registration, error) {
	panic("unimplemented") // not needed for testing the block analyzer
}

// GetTransactionsWithResults implements nodeapi.RuntimeApiLite.
func (mock *mockNode) GetTransactionsWithResults(ctx context.Context, round uint64) ([]nodeapi.RuntimeTransactionWithResults, error) {
	return mock.Txs[round], nil
}

func setupAnalyzer(t *testing.T, testDb *postgres.Client, node *mockNode) analyzer.Analyzer {
	logger := log.NewDefaultLogger(fmt.Sprintf("runtime_%s", t.Name()))

	// Create a runtime metadata object. We reuse a real runtime's metadata here, but the contents
	// (runtime ID, currency name) are not important for these tests.
	sourceConfig := config.SourceConfig{
		ChainName: "testnet",
	}
	sdkPT := sourceConfig.SDKParaTime("pontusx_dev")

	// Determine the min and max rounds at which the mock node has data.
	minRound := math.MaxInt64
	maxRound := 0
	for round := range node.Txs {
		minRound = min(minRound, int(round))
		maxRound = max(maxRound, int(round))
	}
	for round := range node.NonTxEvents {
		minRound = min(minRound, int(round))
		maxRound = max(maxRound, int(round))
	}
	if minRound > maxRound {
		panic("no data in mock node")
	}
	if minRound == 0 && maxRound == 0 {
		// A block analyzer that's configured to go from 0 to 0 thinks that's it's being asked
		// to use the default bounds, i.e. to analyzer endlessly. We don't want that, so we add another round (empty) for analysis.
		maxRound = 1
	}

	analyzer, err := runtime.NewRuntimeAnalyzer(
		"testnet",
		"pontusx_dev", // We borrow a real runtime's name to comply with DB's enums.
		sdkPT,
		config.BlockRange{From: uint64(minRound), To: uint64(maxRound)},
		10 /*batchSize*/, analyzer.SlowSyncMode, node, testDb, logger)
	require.NoError(t, err, "item.NewAnalyzer")

	return analyzer
}

// Constructs a simple runtime transaction containing the given method and body.
func simpleRuntimeTxWithResults(signer sdkTypes.SignatureAddressSpec, method string, body interface{}) nodeapi.RuntimeTransactionWithResults {
	return nodeapi.RuntimeTransactionWithResults{
		Tx: sdkTypes.UnverifiedTransaction{
			Body: cbor.Marshal(sdkTypes.Transaction{
				Versioned: cbor.NewVersioned(1),
				AuthInfo: sdkTypes.AuthInfo{
					SignerInfo: []sdkTypes.SignerInfo{{AddressSpec: sdkTypes.AddressSpec{Signature: &signer}}},
				},
				Call: sdkTypes.Call{
					Method: sdkTypes.MethodName(method),
					Body:   cbor.Marshal(body),
				},
			}),
			AuthProofs: nil,
		},
		Result: sdkTypes.CallResult{
			Ok: cbor.Marshal(nil),
		},
		Events: nil, // Current test cases do not use events, but this is subject to expansion.
	}
}

func runToCompletion(ctx context.Context, analyzer analyzer.Analyzer) {
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		analyzer.Start(ctx)
	}()

	// Wait for analyzer to finish
	analyzersDone := util.ClosingChannel(&wg)
	select {
	case <-time.After(testsTimeout):
		panic("timed out waiting for analyzer to finish")
	case <-analyzersDone:
	}
}

func TestImplicitTo(t *testing.T) {
	ctx := context.Background()
	db := setupDB(t)

	// Create a deposit with no `to` field in the body.
	analyzer := setupAnalyzer(t, db, &mockNode{
		Txs: map[uint64][]nodeapi.RuntimeTransactionWithResults{
			0: {
				simpleRuntimeTxWithResults(
					sdkTesting.Alice.SigSpec,
					"consensus.Deposit",
					consensusaccounts.Deposit{Amount: sdkTypes.NewBaseUnits(*quantity.NewFromUint64(0), sdkTypes.NativeDenomination)},
				),
			},
		},
	})

	runToCompletion(ctx, analyzer)

	// Check that the "to" field in the tx was populated with the sender (= implicit recipient of the Deposit).
	var to string
	row := db.QueryRow(ctx, "SELECT \"to\" from chain.runtime_transactions")
	err := row.Scan(&to)
	require.NoError(t, err, "db fetch")
	require.Equal(t, sdkTesting.Alice.Address.String(), to, "unexpected `to` value for tx")
}
