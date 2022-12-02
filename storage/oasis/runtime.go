package oasis

import (
	"context"
	"fmt"

	config "github.com/oasisprotocol/oasis-sdk/client-sdk/go/config"
	connection "github.com/oasisprotocol/oasis-sdk/client-sdk/go/connection"
	runtimeSignature "github.com/oasisprotocol/oasis-sdk/client-sdk/go/crypto/signature"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/modules/accounts"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/modules/consensusaccounts"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/modules/core"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/modules/evm"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/types"

	"github.com/oasisprotocol/oasis-indexer/storage"
)

// RuntimeClient is a client to a ParaTime.
type RuntimeClient struct {
	client  connection.RuntimeClient
	network *config.Network

	info  *types.RuntimeInfo
	rtCtx runtimeSignature.Context
}

// BlockData gets block data in the specified round.
func (rc *RuntimeClient) BlockData(ctx context.Context, round uint64) (*storage.RuntimeBlockData, error) {
	block, err := rc.client.GetBlock(ctx, round)
	if err != nil {
		return nil, err
	}

	transactionsWithResults, err := rc.client.GetTransactionsWithResults(ctx, round)
	if err != nil {
		return nil, err
	}

	return &storage.RuntimeBlockData{
		Round:                   round,
		BlockHeader:             block,
		TransactionsWithResults: transactionsWithResults,
	}, nil
}

// CoreData gets data in the specified round emitted by the `core` module.
func (rc *RuntimeClient) CoreData(ctx context.Context, round uint64) (*storage.CoreData, error) {
	events, err := rc.client.Core.GetEvents(ctx, round)
	if err != nil {
		return nil, err
	}

	gasUsedEvents := make([]*core.GasUsedEvent, 0, len(events))
	for _, event := range events {
		switch e := event; {
		case e.GasUsed != nil:
			gasUsedEvents = append(gasUsedEvents, e.GasUsed)
		default:
			continue
		}
	}

	return &storage.CoreData{
		Round:   round,
		GasUsed: gasUsedEvents,
	}, nil
}

// AccountsData gets data in the specified round emitted by the `accounts` module.
func (rc *RuntimeClient) AccountsData(ctx context.Context, round uint64) (*storage.AccountsData, error) {
	events, err := rc.client.Accounts.GetEvents(ctx, round)
	if err != nil {
		return nil, err
	}

	transfers := make([]*accounts.TransferEvent, 0)
	mints := make([]*accounts.MintEvent, 0)
	burns := make([]*accounts.BurnEvent, 0)
	for _, event := range events {
		switch e := event; {
		case e.Transfer != nil:
			transfers = append(transfers, e.Transfer)
		case e.Mint != nil:
			mints = append(mints, e.Mint)
		case e.Burn != nil:
			burns = append(burns, e.Burn)
		default:
			continue
		}
	}

	return &storage.AccountsData{
		Round:     round,
		Transfers: transfers,
		Mints:     mints,
		Burns:     burns,
	}, nil
}

// ConsensusAccountsData gets data in the specified round emitted by the `consensusaccounts` module.
func (rc *RuntimeClient) ConsensusAccountsData(ctx context.Context, round uint64) (*storage.ConsensusAccountsData, error) {
	events, err := rc.client.ConsensusAccounts.GetEvents(ctx, round)
	if err != nil {
		return nil, err
	}

	deposits := make([]*consensusaccounts.DepositEvent, 0)
	withdraws := make([]*consensusaccounts.WithdrawEvent, 0)
	for _, event := range events {
		switch e := event; {
		case e.Deposit != nil:
			deposits = append(deposits, e.Deposit)
		case e.Withdraw != nil:
			withdraws = append(withdraws, e.Withdraw)
		default:
			continue
		}
	}

	return &storage.ConsensusAccountsData{
		Round:     round,
		Deposits:  deposits,
		Withdraws: withdraws,
	}, nil
}

func (rc *RuntimeClient) EvmSimulateCall(ctx context.Context, round uint64, gasPrice []byte, gasLimit uint64, caller []byte, address []byte, value []byte, data []byte) ([]byte, error) {
	return evm.NewV1(rc.client).SimulateCall(ctx, round, gasPrice, gasLimit, caller, address, value, data)
}

// Name returns the name of the client, for the RuntimeSourceStorage interface.
func (rc *RuntimeClient) Name() string {
	paratimeName := "unknown"
	for _, network := range config.DefaultNetworks.All {
		for pName := range network.ParaTimes.All {
			if pName == rc.info.ID.String() {
				paratimeName = pName
				break
			}
		}

		if paratimeName != "unknown" {
			break
		}
	}
	return fmt.Sprintf("%s_runtime", paratimeName)
}
