// Package oasis implements the source storage interface
// backed by oasis-node.
package oasis

import (
	"context"

	"github.com/oasisprotocol/oasis-core/go/common"
	"github.com/oasisprotocol/oasis-core/go/common/cbor"
	"github.com/oasisprotocol/oasis-core/go/common/crypto/signature"
	"github.com/oasisprotocol/oasis-core/go/consensus/api/transaction"
	genesisAPI "github.com/oasisprotocol/oasis-core/go/genesis/api"
	governanceAPI "github.com/oasisprotocol/oasis-core/go/governance/api"
	registryAPI "github.com/oasisprotocol/oasis-core/go/registry/api"
	schedulerAPI "github.com/oasisprotocol/oasis-core/go/scheduler/api"
	stakingAPI "github.com/oasisprotocol/oasis-core/go/staking/api"
	config "github.com/oasisprotocol/oasis-sdk/client-sdk/go/config"
	connection "github.com/oasisprotocol/oasis-sdk/client-sdk/go/connection"

	"github.com/oasislabs/oasis-block-indexer/go/storage"
)

const (
	moduleName = "storage.oasis"
)

// OasisNodeClient supports connections to an oasis-node instance.
type OasisNodeClient struct {
	connection *connection.Connection
	network    *config.Network
}

// NewOasisNodeClient creates a new oasis-node client.
func NewOasisNodeClient(ctx context.Context, network *config.Network) (*OasisNodeClient, error) {
	connection, err := connection.Connect(ctx, network)
	if err != nil {
		return nil, err
	}

	chainContext, err := connection.Consensus().GetChainContext(ctx)
	if err != nil {
		return nil, err
	}

	// Configure chain context for all signatures using chain domain separation.
	signature.SetChainContext(chainContext)

	return &OasisNodeClient{
		&connection,
		network,
	}, nil
}

// GenesisDocument returns the original genesis document.
func (c *OasisNodeClient) GenesisDocument(ctx context.Context) (*genesisAPI.Document, error) {
	connection := *c.connection
	doc, err := connection.Consensus().GetGenesisDocument(ctx)
	if err != nil {
		return nil, err
	}

	return doc, nil
}

// Name returns the name of the oasis-node client.
func (c *OasisNodeClient) Name() string {
	return moduleName
}

// BlockData retrieves data about a block at the provided block height.
func (c *OasisNodeClient) BlockData(ctx context.Context, height int64) (*storage.BlockData, error) {
	connection := *c.connection
	block, err := connection.Consensus().GetBlock(ctx, height)
	if err != nil {
		return nil, err
	}

	transactionsWithResults, err := connection.Consensus().GetTransactionsWithResults(ctx, height)
	if err != nil {
		return nil, err
	}

	var transactions []*transaction.SignedTransaction

	for _, bytes := range transactionsWithResults.Transactions {
		var transaction transaction.SignedTransaction
		if err := cbor.Unmarshal(bytes, &transaction); err != nil {
			return nil, err
		}
		transactions = append(transactions, &transaction)
	}

	return &storage.BlockData{
		BlockHeader:  block,
		Transactions: transactions,
		Results:      transactionsWithResults.Results,
	}, nil
}

// BeaconData retrieves the beacon for the provided block height.
// NOTE: The random beacon endpoint is in flux.
func (c *OasisNodeClient) BeaconData(ctx context.Context, height int64) (*storage.BeaconData, error) {
	connection := *c.connection
	beacon, err := connection.Consensus().Beacon().GetBeacon(ctx, height)
	if err != nil {
		return nil, err
	}

	epoch, err := connection.Consensus().Beacon().GetEpoch(ctx, height)
	if err != nil {
		return nil, err
	}

	return &storage.BeaconData{
		Epoch:  epoch,
		Beacon: beacon,
	}, nil
}

// RegistryData retrieves registry events at the provided block height.
func (c *OasisNodeClient) RegistryData(ctx context.Context, height int64) (*storage.RegistryData, error) {
	connection := *c.connection
	events, err := connection.Consensus().Registry().GetEvents(ctx, height)
	if err != nil {
		return nil, err
	}

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

// StakingData retrieves staking events at the provided block height.
func (c *OasisNodeClient) StakingData(ctx context.Context, height int64) (*storage.StakingData, error) {
	connection := *c.connection
	events, err := connection.Consensus().Staking().GetEvents(ctx, height)
	if err != nil {
		return nil, err
	}

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

// SchedulerData retrieves validators and runtime committees at the provided block height.
func (c *OasisNodeClient) SchedulerData(ctx context.Context, height int64) (*storage.SchedulerData, error) {
	connection := *c.connection
	validators, err := connection.Consensus().Scheduler().GetValidators(ctx, height)
	if err != nil {
		return nil, err
	}

	var committees = make(map[common.Namespace][]*schedulerAPI.Committee, len(c.network.ParaTimes.All))

	for name := range c.network.ParaTimes.All {
		var runtimeID common.Namespace
		if err := runtimeID.UnmarshalHex(c.network.ParaTimes.All[name].ID); err != nil {
			return nil, err
		}

		consensusCommittees, err := connection.Consensus().Scheduler().GetCommittees(ctx, &schedulerAPI.GetCommitteesRequest{
			Height:    height,
			RuntimeID: runtimeID,
		})
		if err != nil {
			return nil, err
		}
		committees[runtimeID] = consensusCommittees
	}

	return &storage.SchedulerData{
		Validators: validators,
		Committees: committees,
	}, nil
}

// GovernanceData retrieves governance events at the provided block height.
func (c *OasisNodeClient) GovernanceData(ctx context.Context, height int64) (*storage.GovernanceData, error) {
	connection := *c.connection
	events, err := connection.Consensus().Governance().GetEvents(ctx, height)
	if err != nil {
		return nil, err
	}

	var submissions []*governanceAPI.Proposal
	var executions []*governanceAPI.ProposalExecutedEvent
	var finalizations []*governanceAPI.Proposal
	var votes []*governanceAPI.VoteEvent

	for _, event := range events {
		if event.ProposalSubmitted != nil {
			proposal, err := connection.Consensus().Governance().Proposal(ctx, &governanceAPI.ProposalQuery{
				Height:     height,
				ProposalID: event.ProposalSubmitted.ID,
			})
			if err != nil {
				return nil, err
			}
			submissions = append(submissions, proposal)
		} else if event.ProposalExecuted != nil {
			executions = append(executions, event.ProposalExecuted)
		} else if event.ProposalFinalized != nil {
			proposal, err := connection.Consensus().Governance().Proposal(ctx, &governanceAPI.ProposalQuery{
				Height:     height,
				ProposalID: event.ProposalFinalized.ID,
			})
			if err != nil {
				return nil, err
			}
			finalizations = append(finalizations, proposal)
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
