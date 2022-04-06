// Package oasis implements the source storage interface
// backed by oasis-node.
package oasis

import (
	"context"

	"github.com/oasislabs/oasis-block-indexer/go/storage"
	"github.com/oasisprotocol/oasis-core/go/common"
	"github.com/oasisprotocol/oasis-core/go/common/cbor"
	"github.com/oasisprotocol/oasis-core/go/consensus/api/transaction"
	governanceAPI "github.com/oasisprotocol/oasis-core/go/governance/api"
	registryAPI "github.com/oasisprotocol/oasis-core/go/registry/api"
	schedulerAPI "github.com/oasisprotocol/oasis-core/go/scheduler/api"
	stakingAPI "github.com/oasisprotocol/oasis-core/go/staking/api"
	config "github.com/oasisprotocol/oasis-sdk/client-sdk/go/config"
	connection "github.com/oasisprotocol/oasis-sdk/client-sdk/go/connection"
	"github.com/spf13/cobra"
)

const (
	clientName = "oasis-node"
)

type OasisNodeClient struct {
	connection *connection.Connection
}

// NewOasisNodeClient creates a new oasis-node client.
func NewOasisNodeClient(ctx context.Context) (*OasisNodeClient, error) {
	// Assume default for now
	network := config.DefaultNetworks.All[config.DefaultNetworks.Default]
	connection, err := connection.Connect(ctx, network)
	cobra.CheckErr(err)

	return &OasisNodeClient{
		&connection,
	}, nil
}

// Name returns the name of the oasis-node client.
func (c *OasisNodeClient) Name() string {
	return clientName
}

func (c *OasisNodeClient) BlockData(ctx context.Context, height int64) (*storage.BlockData, error) {
	connection := *c.connection
	block, err := connection.Consensus().GetBlock(ctx, height)
	cobra.CheckErr(err)
	transactionsWithResults, err := connection.Consensus().GetTransactionsWithResults(ctx, height)
	cobra.CheckErr(err)

	var transactions []*transaction.SignedTransaction

	for _, bytes := range transactionsWithResults.Transactions {
		var transaction transaction.SignedTransaction
		cbor.Unmarshal(bytes, &transaction)
		transactions = append(transactions, &transaction)
	}

	return &storage.BlockData{
		BlockHeader:  block,
		Transactions: transactions,
		Results:      transactionsWithResults.Results,
	}, nil
}

func (c *OasisNodeClient) BeaconData(ctx context.Context, height int64) (*storage.BeaconData, error) {
	connection := *c.connection
	beacon, err := connection.Consensus().Beacon().GetBeacon(ctx, height)
	cobra.CheckErr(err)

	epoch, err := connection.Consensus().Beacon().GetEpoch(ctx, height)
	cobra.CheckErr(err)

	return &storage.BeaconData{
		Epoch:  epoch,
		Beacon: beacon,
	}, nil
}

func (c *OasisNodeClient) RegistryData(ctx context.Context, height int64) (*storage.RegistryData, error) {
	connection := *c.connection
	events, err := connection.Consensus().Registry().GetEvents(ctx, height)
	cobra.CheckErr(err)

	var runtimeEvents []*registryAPI.RuntimeEvent
	var entityEvents []*registryAPI.EntityEvent
	var nodeEvents []*registryAPI.NodeEvent
	var nodeUnfrozenEvents []*registryAPI.NodeUnfrozenEvent

	for _, event := range events {
		if event.RuntimeEvent != nil {
			runtimeEvents = append(runtimeEvents, event.RuntimeEvent)
		} else if event.EntityEvent != nil {
			entityEvents = append(entityEvents, event.EntityEvent)
		} else if event.NodeEvent != nil {
			nodeEvents = append(nodeEvents, event.NodeEvent)
		} else if event.NodeUnfrozenEvent != nil {
			nodeUnfrozenEvents = append(nodeUnfrozenEvents, event.NodeUnfrozenEvent)
		}
	}

	return &storage.RegistryData{
		RuntimeEvents:     runtimeEvents,
		EntityEvents:      entityEvents,
		NodeEvent:         nodeEvents,
		NodeUnfrozenEvent: nodeUnfrozenEvents,
	}, nil
}

func (c *OasisNodeClient) StakingData(ctx context.Context, height int64) (*storage.StakingData, error) {
	connection := *c.connection
	events, err := connection.Consensus().Staking().GetEvents(ctx, height)
	cobra.CheckErr(err)

	var transfers []*stakingAPI.TransferEvent
	var burns []*stakingAPI.BurnEvent
	var escrows []*stakingAPI.EscrowEvent
	var allowanceChanges []*stakingAPI.AllowanceChangeEvent

	for _, event := range events {
		if event.Transfer != nil {
			transfers = append(transfers, event.Transfer)
		} else if event.Burn != nil {
			burns = append(burns, event.Burn)
		} else if event.Escrow != nil {
			escrows = append(escrows, event.Escrow)
		} else if event.AllowanceChange != nil {
			allowanceChanges = append(allowanceChanges, event.AllowanceChange)
		}
	}

	return &storage.StakingData{
		Transfers:        transfers,
		Burns:            burns,
		Escrows:          escrows,
		AllowanceChanges: allowanceChanges,
	}, nil
}

func (c *OasisNodeClient) ChainContext(ctx context.Context) (string, error) {
	connection := *c.connection
	return connection.Consensus().GetChainContext(ctx)
}

func (c *OasisNodeClient) SchedulerData(ctx context.Context, height int64) (*storage.SchedulerData, error) {
	connection := *c.connection
	validators, err := connection.Consensus().Scheduler().GetValidators(ctx, height)
	cobra.CheckErr(err)

	var committees = make(map[common.Namespace][]*schedulerAPI.Committee)

	for k := range config.DefaultNetworks.All[config.DefaultNetworks.Default].ParaTimes.All {
		var runtimeID common.Namespace
		runtimeID.UnmarshalHex(k)

		consensusCommittees, err := connection.Consensus().Scheduler().GetCommittees(ctx, &schedulerAPI.GetCommitteesRequest{
			Height:    height,
			RuntimeID: runtimeID,
		})
		cobra.CheckErr(err)
		committees[runtimeID] = consensusCommittees
	}

	return &storage.SchedulerData{
		Validators: validators,
		Committees: committees,
	}, nil
}

func (c *OasisNodeClient) GovernanceData(ctx context.Context, height int64) (*storage.GovernanceData, error) {
	connection := *c.connection
	events, err := connection.Consensus().Governance().GetEvents(ctx, height)
	cobra.CheckErr(err)

	var submissions []*governanceAPI.ProposalSubmittedEvent
	var executions []*governanceAPI.ProposalExecutedEvent
	var finalizations []*governanceAPI.ProposalFinalizedEvent
	var votes []*governanceAPI.VoteEvent

	for _, event := range events {
		if event.ProposalSubmitted != nil {
			submissions = append(submissions, event.ProposalSubmitted)
		} else if event.ProposalExecuted != nil {
			executions = append(executions, event.ProposalExecuted)
		} else if event.ProposalFinalized != nil {
			finalizations = append(finalizations, event.ProposalFinalized)
		} else if event.Vote != nil {
			votes = append(votes, event.Vote)
		}
	}
	return &storage.GovernanceData{
		ProposalSubmissions:   submissions,
		ProposalExecutions:    executions,
		ProposalFinalizations: finalizations,
		Votes:                 votes,
	}, nil
}
