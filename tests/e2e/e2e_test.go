package e2e

import (
	"context"
	"crypto"
	"fmt"
	"math/rand"
	"os"
	"testing"
	"time"

	"github.com/oasisprotocol/oasis-core/go/common/crypto/drbg"
	"github.com/oasisprotocol/oasis-core/go/common/crypto/mathrand"
	"github.com/oasisprotocol/oasis-core/go/common/crypto/signature"
	memorySigner "github.com/oasisprotocol/oasis-core/go/common/crypto/signature/signers/memory"
	oasisGrpc "github.com/oasisprotocol/oasis-core/go/common/grpc"
	"github.com/oasisprotocol/oasis-core/go/common/quantity"
	"github.com/oasisprotocol/oasis-core/go/oasis-node/cmd/common"
	"github.com/stretchr/testify/require"

	"github.com/oasisprotocol/oasis-core/go/common/entity"
	consensus "github.com/oasisprotocol/oasis-core/go/consensus/api"
	"github.com/oasisprotocol/oasis-core/go/consensus/api/transaction"

	"github.com/oasisprotocol/oasis-core/go/oasis-node/cmd/debug/txsource/workload"
	staking "github.com/oasisprotocol/oasis-core/go/staking/api"
	v1 "github.com/oasisprotocol/oasis-indexer/api/v1"
	"github.com/oasisprotocol/oasis-indexer/tests"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	fundAccountAmount = 10000000000
	timeout           = 1 * time.Second
)

func TestIndexer(t *testing.T) {
	if _, ok := os.LookupEnv("OASIS_INDEXER_E2E"); !ok {
		t.Skip("skipping test since e2e tests are not enabled")
	}

	if err := common.Init(); err != nil {
		common.EarlyLogAndExit(err)
	}
	tests.Init()

	// Indexer should be up
	var status v1.Status
	err := tests.GetFrom("/", &status)
	require.Nil(t, err)

	connString := "unix:/testnet/net-runner/network/validator-0/internal.sock"
	conn, _ := oasisGrpc.Dial(connString, grpc.WithTransportCredentials(insecure.NewCredentials()))

	ctx := context.Background()
	cnsc := consensus.NewConsensusClient(conn)
	doc, _ := cnsc.GetGenesisDocument(ctx)
	doc.SetChainContext()

	hash := crypto.SHA512
	seed := []byte("seeeeeeeeeeeeeeeeeeeeeeeeeeeeeed")
	name := "transfer"
	src, _ := drbg.New(hash, seed, nil, []byte(fmt.Sprintf("txsource workload generator v1, workload %s", name)))

	rng := rand.New(mathrand.New(src)) //nolint:gosec // G404: Use of weak random number generator (math/rand instead of crypto/rand)

	fundingAccount, _ := memorySigner.NewFactory().Generate(signature.SignerEntity, rng)

	fmt.Println("Funding account address: ", staking.NewAddress(fundingAccount.Public()))

	fac := memorySigner.NewFactory()

	aliceAccount, _ := fac.Generate(signature.SignerEntity, rng)
	bobAccount, _ := fac.Generate(signature.SignerEntity, rng)
	aliceAddress := staking.NewAddress(aliceAccount.Public())
	bobAddress := staking.NewAddress(bobAccount.Public())

	fmt.Println("Alice address: ", aliceAddress)
	fmt.Println("Bob address: ", bobAddress)

	pd, _ := consensus.NewStaticPriceDiscovery(uint64(0))
	sm := consensus.NewSubmissionManager(cnsc, pd, 0)

	_, testEntitySigner, _ := entity.TestEntity()

	// Seed Alice account
	bw := workload.NewBaseWorkload("funding")
	bw.Init(cnsc, sm, testEntitySigner)
	if err := workload.FundAccountFromTestEntity(ctx, cnsc, sm, fundingAccount); err != nil {
		panic(fmt.Errorf("test entity account funding failure: %w", err))
	}
	if err := bw.TransferFunds(ctx, fundingAccount, aliceAddress, fundAccountAmount); err != nil {
		panic(fmt.Errorf("account funding failure: %w", err))
	}

	// Bob account does not exist
	var account v1.Account
	err = tests.GetFrom(fmt.Sprintf("/consensus/accounts/%s", bobAddress), &account)
	require.Nil(t, err)
	require.Empty(t, account.Address)

	// Transfer to Bob account
	if err := bw.TransferFunds(ctx, aliceAccount, bobAddress, 100000000); err != nil {
		panic(fmt.Errorf("account funding failure: %w", err))
	}

	time.Sleep(timeout)
	err = tests.GetFrom(fmt.Sprintf("/consensus/accounts/%s", bobAddress), &account)
	require.Nil(t, err)
	require.Equal(t, account.Available, uint64(100000000))

	// Create escrow tx.
	escrow := &staking.Escrow{
		Account: aliceAddress,
	}
	delegateAmount := uint64(25000000)
	if err := escrow.Amount.FromUint64(delegateAmount); err != nil {
		panic(fmt.Errorf("escrow amount error: %w", err))
	}

	tx := staking.NewAddEscrowTx(uint64(1), nil, escrow)
	tx.Fee = &transaction.Fee{
		Amount: *quantity.NewFromUint64(3),
		Gas:    0x9000000000000000,
	}

	if err := bw.FundSignAndSubmitTx(ctx, bobAccount, tx); err != nil {
		panic(fmt.Errorf("failed to sign and submit tx: %w", err))
	}

	// Bob account has correct delegation balance
	time.Sleep(timeout)
	err = tests.GetFrom(fmt.Sprintf("/consensus/accounts/%s", bobAddress), &account)
	require.Nil(t, err)
	require.Equal(t, account.DelegationsBalance, uint64(25000000))
	require.Equal(t, account.Available, uint64(75000000))

	// Alice account has correct escrow balance
	time.Sleep(timeout)
	err = tests.GetFrom(fmt.Sprintf("/consensus/accounts/%s", aliceAddress), &account)
	require.Nil(t, err)
	require.Equal(t, account.Escrow, uint64(25000000))
	require.Equal(t, account.Available, uint64(9900000000))
}
