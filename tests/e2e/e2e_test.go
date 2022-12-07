package e2e

import (
	"context"
	"crypto"
	"fmt"
	"math/big"
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
	"github.com/oasisprotocol/oasis-core/go/oasis-node/cmd/common"
	"github.com/oasisprotocol/oasis-core/go/oasis-node/cmd/debug/txsource/workload"
	staking "github.com/oasisprotocol/oasis-core/go/staking/api"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	storage "github.com/oasisprotocol/oasis-indexer/storage/client"
	"github.com/oasisprotocol/oasis-indexer/tests"
)

const (
	fundAccountAmount = 10000000000
	// The amount of time to wait after a blockchain tx to make sure that the a block with the
	// tx was produced and the indexer has indexed the block.
	// The node seems to generate a block per second.
	// 2 seconds is experimentally enough, 1 second was flaky.
	indexerDelay = 2 * time.Second
)

func TestIndexer(t *testing.T) {
	tests.SkipUnlessE2E(t)

	if err := common.Init(); err != nil {
		common.EarlyLogAndExit(err)
	}
	tests.Init()

	// Indexer should be up
	var status storage.Status
	err := tests.GetFrom("/", &status)
	require.Nil(t, err)

	connString := "unix:/testnet/net-runner/network/validator-0/internal.sock"
	conn, err := oasisGrpc.Dial(connString, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.Nil(t, err)

	ctx := context.Background()
	cnsc := consensus.NewConsensusClient(conn)
	doc, err := cnsc.GetGenesisDocument(ctx)
	require.Nil(t, err)
	doc.SetChainContext()

	hash := crypto.SHA512
	seed := []byte("seeeeeeeeeeeeeeeeeeeeeeeeeeeeeed")
	name := "transfer"
	src, err := drbg.New(hash, seed, nil, []byte(fmt.Sprintf("txsource workload generator v1, workload %s", name)))
	require.Nil(t, err)

	rng := rand.New(mathrand.New(src)) //nolint:gosec // G404: Use of weak random number generator (math/rand instead of crypto/rand)

	fundingAccount, err := memorySigner.NewFactory().Generate(signature.SignerEntity, rng)
	require.Nil(t, err)

	t.Log("Funding account address: ", staking.NewAddress(fundingAccount.Public()))

	fac := memorySigner.NewFactory()

	aliceAccount, err := fac.Generate(signature.SignerEntity, rng)
	require.Nil(t, err)
	bobAccount, err := fac.Generate(signature.SignerEntity, rng)
	require.Nil(t, err)
	aliceAddress := staking.NewAddress(aliceAccount.Public())
	bobAddress := staking.NewAddress(bobAccount.Public())

	t.Log("Alice address: ", aliceAddress)
	t.Log("Bob address: ", bobAddress)

	pd, err := consensus.NewStaticPriceDiscovery(uint64(0))
	require.Nil(t, err)
	sm := consensus.NewSubmissionManager(cnsc, pd, 0)

	_, testEntitySigner, err := entity.TestEntity()
	require.Nil(t, err)

	// Seed Alice account
	bw := workload.NewBaseWorkload("funding")
	bw.Init(cnsc, sm, testEntitySigner)
	if err := workload.FundAccountFromTestEntity(ctx, cnsc, sm, fundingAccount); err != nil {
		t.Errorf("test entity account funding failure: %v", err)
	}
	if err := bw.TransferFunds(ctx, fundingAccount, aliceAddress, fundAccountAmount); err != nil {
		t.Errorf("account funding failure: %v", err)
	}

	// Bob account does not exist
	var account storage.Account
	err = tests.GetFrom(fmt.Sprintf("/consensus/accounts/%s", bobAddress), &account)
	require.Nil(t, err)
	require.Empty(t, account.Address)

	// Transfer to Bob account
	if err := bw.TransferFunds(ctx, aliceAccount, bobAddress, 100000000); err != nil {
		t.Errorf("account funding failure: %v", err)
	}

	time.Sleep(indexerDelay)
	err = tests.GetFrom(fmt.Sprintf("/consensus/accounts/%s", bobAddress), &account)
	require.Nil(t, err)
	require.Equal(t, big.NewInt(100000000), account.Available)

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
	time.Sleep(indexerDelay)
	err = tests.GetFrom(fmt.Sprintf("/consensus/accounts/%s", bobAddress), &account)
	require.Nil(t, err)
	require.Equal(t, big.NewInt(25000000), account.DelegationsBalance)
	require.Equal(t, big.NewInt(75000000), account.Available)

	// Alice account has correct escrow balance
	time.Sleep(indexerDelay)
	err = tests.GetFrom(fmt.Sprintf("/consensus/accounts/%s", aliceAddress), &account)
	require.Nil(t, err)
	require.Equal(t, big.NewInt(25000000), account.Escrow)
	require.Equal(t, big.NewInt(9900000000), account.Available)
}
