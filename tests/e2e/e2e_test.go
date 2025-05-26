package e2e

import (
	"context"
	"crypto"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/oasisprotocol/oasis-core/go/common/crypto/drbg"
	"github.com/oasisprotocol/oasis-core/go/common/crypto/mathrand"
	"github.com/oasisprotocol/oasis-core/go/common/crypto/signature"
	memorySigner "github.com/oasisprotocol/oasis-core/go/common/crypto/signature/signers/memory"
	"github.com/oasisprotocol/oasis-core/go/common/entity"
	oasisGrpc "github.com/oasisprotocol/oasis-core/go/common/grpc"
	"github.com/oasisprotocol/oasis-core/go/common/quantity"
	consensus "github.com/oasisprotocol/oasis-core/go/consensus/api"
	"github.com/oasisprotocol/oasis-core/go/consensus/api/transaction"
	cmdCommon "github.com/oasisprotocol/oasis-core/go/oasis-node/cmd/common"
	"github.com/oasisprotocol/oasis-core/go/oasis-node/cmd/debug/txsource/workload"
	staking "github.com/oasisprotocol/oasis-core/go/staking/api"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/oasisprotocol/nexus/common"
	storage "github.com/oasisprotocol/nexus/storage/client"
	"github.com/oasisprotocol/nexus/tests"
)

const (
	fundAccountAmount = 10000000000
	// The amount of time to wait after a blockchain tx to make sure that the a block with the
	// tx was produced and the consensus analyzer has indexed the block.
	// The node seems to generate a block per second.
	// 3 seconds is experimentally enough, 2 second was flaky.
	analyzerDelay = 3 * time.Second
)

// Taken from https://github.com/oasisprotocol/oasis-core/blob/v23.0.11/go/consensus/api/submission.go#L31
type staticPriceDiscovery struct {
	price quantity.Quantity
}

// NewStaticPriceDiscovery creates a price discovery mechanism which always returns the same static
// price specified at construction time.
func NewStaticPriceDiscovery(price uint64) (consensus.PriceDiscovery, error) {
	pd := &staticPriceDiscovery{}
	if err := pd.price.FromUint64(price); err != nil {
		return nil, fmt.Errorf("submission: failed to convert gas price: %w", err)
	}
	return pd, nil
}

func (pd *staticPriceDiscovery) GasPrice() (*quantity.Quantity, error) {
	return pd.price.Clone(), nil
}

func TestConsensusTransfer(t *testing.T) {
	tests.SkipUnlessE2E(t)

	if err := cmdCommon.Init(); err != nil {
		cmdCommon.EarlyLogAndExit(err)
	}
	tests.Init()

	// API server should be up
	var status storage.Status
	err := tests.GetFrom("/", &status)
	require.NoError(t, err)

	connString := "unix:/testnet/net-runner/network/validator-0/internal.sock"
	conn, err := oasisGrpc.Dial(connString, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)

	ctx := context.Background()
	cnsc := consensus.NewServicesClient(conn)
	doc, err := cnsc.Core().GetGenesisDocument(ctx)
	require.NoError(t, err)
	doc.SetChainContext()

	hash := crypto.SHA512
	seed := []byte("seeeeeeeeeeeeeeeeeeeeeeeeeeeeeed")
	name := "transfer"
	src, err := drbg.New(hash, seed, nil, []byte(fmt.Sprintf("txsource workload generator v1, workload %s", name)))
	require.NoError(t, err)

	rng := rand.New(mathrand.New(src)) //nolint:gosec // G404: Use of weak random number generator (math/rand instead of crypto/rand)

	fundingAccount, err := memorySigner.NewFactory().Generate(signature.SignerEntity, rng)
	require.NoError(t, err)

	t.Log("Funding account address: ", staking.NewAddress(fundingAccount.Public()))

	fac := memorySigner.NewFactory()

	aliceAccount, err := fac.Generate(signature.SignerEntity, rng)
	require.NoError(t, err)
	bobAccount, err := fac.Generate(signature.SignerEntity, rng)
	require.NoError(t, err)
	aliceAddress := staking.NewAddress(aliceAccount.Public())
	bobAddress := staking.NewAddress(bobAccount.Public())

	t.Log("Alice address: ", aliceAddress)
	t.Log("Bob address: ", bobAddress)

	pd, err := NewStaticPriceDiscovery(uint64(0))
	require.NoError(t, err)
	sm := consensus.NewSubmissionManager(cnsc.Core(), pd, 0)

	_, testEntitySigner, err := entity.TestEntity()
	require.NoError(t, err)

	// Seed Alice account
	bw := workload.NewBaseWorkload("funding")
	bw.Init(cnsc, sm, testEntitySigner)
	if err := workload.FundAccountFromTestEntity(ctx, cnsc, sm, fundingAccount); err != nil {
		t.Errorf("test entity account funding failure: %v", err)
	}
	if err := bw.TransferFunds(ctx, fundingAccount, aliceAddress, fundAccountAmount); err != nil {
		t.Errorf("account funding failure: %v", err)
	}

	// Bob account "does not exist". Technically, every valid account exists, but it has no balance or activity.
	var account storage.Account
	err = tests.GetFrom(fmt.Sprintf("/consensus/accounts/%s", bobAddress), &account)
	require.NoError(t, err)
	require.Equal(t, account.Address, bobAddress.String())
	require.Zero(t, account.Nonce)
	require.Zero(t, account.Available)
	require.Zero(t, account.DelegationsBalance)
	require.Zero(t, account.DebondingDelegationsBalance)
	require.Zero(t, account.Escrow)

	// Transfer to Bob account
	if err := bw.TransferFunds(ctx, aliceAccount, bobAddress, 100000000); err != nil {
		t.Errorf("account funding failure: %v", err)
	}

	time.Sleep(analyzerDelay)
	err = tests.GetFrom(fmt.Sprintf("/consensus/accounts/%s", bobAddress), &account)
	require.NoError(t, err)
	expected := common.NewBigInt(100000000)
	require.Equal(t, expected, account.Available)

	// Create escrow tx.
	escrow := &staking.Escrow{
		Account: aliceAddress,
	}
	delegateAmount := uint64(25000000)
	if err := escrow.Amount.FromUint64(delegateAmount); err != nil {
		t.Errorf("escrow amount error: %v", err)
	}

	tx := staking.NewAddEscrowTx(uint64(1), nil, escrow)
	tx.Fee = &transaction.Fee{
		Amount: *quantity.NewFromUint64(3),
		Gas:    0x9000000000000000,
	}

	if err := bw.FundSignAndSubmitTx(ctx, bobAccount, tx); err != nil {
		t.Errorf("failed to sign and submit tx: %v", err)
	}

	// Bob account has correct delegation balance
	time.Sleep(analyzerDelay)
	err = tests.GetFrom(fmt.Sprintf("/consensus/accounts/%s", bobAddress), &account)
	require.NoError(t, err)
	expectedDelegationsBalance := common.NewBigInt(25000000)
	expectedAvailable := common.NewBigInt(75000000)
	require.Equal(t, expectedDelegationsBalance, account.DelegationsBalance)
	require.Equal(t, expectedAvailable, account.Available)

	// Alice account has correct escrow balance
	time.Sleep(analyzerDelay)
	err = tests.GetFrom(fmt.Sprintf("/consensus/accounts/%s", aliceAddress), &account)
	require.NoError(t, err)
	expectedEscrow := common.NewBigInt(25000000)
	expectedAvailable = common.NewBigInt(9900000000)
	require.Equal(t, expectedEscrow, account.Escrow)
	require.Equal(t, expectedAvailable, account.Available)
}
