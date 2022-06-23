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

	"github.com/oasislabs/oasis-indexer/storage"
)

const (
	moduleName = "storage_oasis"
)

// Client supports connections to an oasis-node instance.
type Client struct {
	connection    *connection.Connection
	network       *config.Network
	genesisHeight int64
}

// NewClient creates a new oasis-node client.
func NewClient(ctx context.Context, network *config.Network) (*Client, error) {
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

	c := &Client{
		connection: &connection,
		network:    network,
	}
	doc, err := c.GenesisDocument(ctx)
	if err != nil {
		return nil, err
	}
	c.genesisHeight = doc.Height

	return c, nil
}

// GenesisDocument returns the original genesis document.
func (c *Client) GenesisDocument(ctx context.Context) (*genesisAPI.Document, error) {
	connection := *c.connection
	doc, err := connection.Consensus().GetGenesisDocument(ctx)
	if err != nil {
		return nil, err
	}

	return doc, nil
}

// GenesisDocumentAtHeight returns the genesis document at the provided height.
func (c *Client) GenesisDocumentAtHeight(ctx context.Context, height int64) (*genesisAPI.Document, error) {
	connection := *c.connection
	doc, err := connection.Consensus().StateToGenesis(ctx, height)
	if err != nil {
		return nil, err
	}

	return doc, nil
}

// RegistryGenesis returns the registry genesis document at the provided height.
func (c *Client) RegistryGenesis(ctx context.Context, height int64) (*registryAPI.Genesis, error) {
	connection := *c.connection
	doc, err := connection.Consensus().Registry().StateToGenesis(ctx, height)
	if err != nil {
		return nil, err
	}

	return doc, nil
}

// StakingGenesis returns the staking genesis document at the provided height.
func (c *Client) StakingGenesis(ctx context.Context, height int64) (*stakingAPI.Genesis, error) {
	connection := *c.connection
	doc, err := connection.Consensus().Staking().StateToGenesis(ctx, height)
	if err != nil {
		return nil, err
	}

	return doc, nil
}

// SchedulerGenesis returns the scheduler genesis document at the provided height.
func (c *Client) SchedulerGenesis(ctx context.Context, height int64) (*schedulerAPI.Genesis, error) {
	connection := *c.connection
	doc, err := connection.Consensus().Scheduler().StateToGenesis(ctx, height)
	if err != nil {
		return nil, err
	}

	return doc, nil
}

// GovernanceGenesis returns the governance genesis document at the provided height.
func (c *Client) GovernanceGenesis(ctx context.Context, height int64) (*governanceAPI.Genesis, error) {
	connection := *c.connection
	doc, err := connection.Consensus().Governance().StateToGenesis(ctx, height)
	if err != nil {
		return nil, err
	}

	return doc, nil
}

// Name returns the name of the oasis-node client.
func (c *Client) Name() string {
	return moduleName
}

// BlockData retrieves data about a block at the provided block height.
func (c *Client) BlockData(ctx context.Context, height int64) (*storage.BlockData, error) {
	connection := *c.connection
	block, err := connection.Consensus().GetBlock(ctx, height)
	if err != nil {
		return nil, err
	}

	transactionsWithResults, err := connection.Consensus().GetTransactionsWithResults(ctx, height)
	if err != nil {
		return nil, err
	}

	transactions := make([]*transaction.SignedTransaction, 0, len(transactionsWithResults.Transactions))
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
func (c *Client) BeaconData(ctx context.Context, height int64) (*storage.BeaconData, error) {
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
func (c *Client) RegistryData(ctx context.Context, height int64) (*storage.RegistryData, error) {
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
		switch e := event; {
		case e.RuntimeEvent != nil:
			runtimeEvents = append(runtimeEvents, e.RuntimeEvent)
		case e.EntityEvent != nil:
			entityEvents = append(entityEvents, e.EntityEvent)
		case e.NodeEvent != nil:
			nodeEvents = append(nodeEvents, e.NodeEvent)
		case e.NodeUnfrozenEvent != nil:
			nodeUnfrozenEvents = append(nodeUnfrozenEvents, e.NodeUnfrozenEvent)
		}
	}

	rts, err := c.runtimeUpdates(ctx, height)
	if err != nil {
		return nil, err
	}

	var suspensions, unsuspensions []string
	for rt, suspended := range rts {
		if suspended {
			suspensions = append(suspensions, rt)
		} else {
			unsuspensions = append(unsuspensions, rt)
		}
	}

	return &storage.RegistryData{
		RuntimeEvents:        runtimeEvents,
		EntityEvents:         entityEvents,
		NodeEvents:           nodeEvents,
		NodeUnfrozenEvents:   nodeUnfrozenEvents,
		RuntimeSuspensions:   suspensions,
		RuntimeUnsuspensions: unsuspensions,
	}, nil
}

func (c *Client) runtimeUpdates(ctx context.Context, height int64) (map[string]bool, error) {
	rtsCurr, err := c.runtimes(ctx, height)
	if err != nil {
		return nil, err
	}

	if height != c.genesisHeight {
		rtsPrev, err := c.runtimes(ctx, height-1)
		if err != nil {
			return nil, err
		}

		for r, suspended := range rtsPrev {
			if rtsCurr[r] == suspended {
				delete(rtsCurr, r)
			}
		}
	}
	return rtsCurr, nil
}

func (c *Client) runtimes(ctx context.Context, height int64) (map[string]bool, error) {
	connection := *c.connection
	allRuntimes, err := connection.Consensus().Registry().GetRuntimes(ctx, &registryAPI.GetRuntimesQuery{
		Height:           height,
		IncludeSuspended: true,
	})
	if err != nil {
		return nil, err
	}
	runtimes, err := connection.Consensus().Registry().GetRuntimes(ctx, &registryAPI.GetRuntimesQuery{
		Height:           height,
		IncludeSuspended: false,
	})
	if err != nil {
		return nil, err
	}

	// Mark suspended runtimes.
	rts := make(map[string]bool)
	for _, r := range allRuntimes {
		rts[r.ID.String()] = true
	}
	for _, r := range runtimes {
		rts[r.ID.String()] = false
	}

	return rts, nil
}

// StakingData retrieves staking events at the provided block height.
func (c *Client) StakingData(ctx context.Context, height int64) (*storage.StakingData, error) {
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
		switch e := event; {
		case e.Transfer != nil:
			transfers = append(transfers, event.Transfer)
		case e.Burn != nil:
			burns = append(burns, event.Burn)
		case e.Escrow != nil:
			escrows = append(escrows, event.Escrow)
		case e.AllowanceChange != nil:
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
func (c *Client) SchedulerData(ctx context.Context, height int64) (*storage.SchedulerData, error) {
	connection := *c.connection
	validators, err := connection.Consensus().Scheduler().GetValidators(ctx, height)
	if err != nil {
		return nil, err
	}

	committees := make(map[common.Namespace][]*schedulerAPI.Committee, len(c.network.ParaTimes.All))

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
func (c *Client) GovernanceData(ctx context.Context, height int64) (*storage.GovernanceData, error) {
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
		switch e := event; {
		case e.ProposalSubmitted != nil:
			proposal, err := connection.Consensus().Governance().Proposal(ctx, &governanceAPI.ProposalQuery{
				Height:     height,
				ProposalID: event.ProposalSubmitted.ID,
			})
			if err != nil {
				return nil, err
			}
			submissions = append(submissions, proposal)
		case e.ProposalExecuted != nil:
			executions = append(executions, event.ProposalExecuted)
		case e.ProposalFinalized != nil:
			proposal, err := connection.Consensus().Governance().Proposal(ctx, &governanceAPI.ProposalQuery{
				Height:     height,
				ProposalID: event.ProposalFinalized.ID,
			})
			if err != nil {
				return nil, err
			}
			finalizations = append(finalizations, proposal)
		case e.Vote != nil:
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
