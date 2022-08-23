package oasis

import (
	"context"
	"fmt"

	"github.com/oasisprotocol/oasis-core/go/common"
	"github.com/oasisprotocol/oasis-core/go/common/cbor"
	consensus "github.com/oasisprotocol/oasis-core/go/consensus/api"
	"github.com/oasisprotocol/oasis-core/go/consensus/api/transaction"
	genesisAPI "github.com/oasisprotocol/oasis-core/go/genesis/api"
	governanceAPI "github.com/oasisprotocol/oasis-core/go/governance/api"
	registryAPI "github.com/oasisprotocol/oasis-core/go/registry/api"
	schedulerAPI "github.com/oasisprotocol/oasis-core/go/scheduler/api"
	stakingAPI "github.com/oasisprotocol/oasis-core/go/staking/api"
	config "github.com/oasisprotocol/oasis-sdk/client-sdk/go/config"

	"github.com/oasisprotocol/oasis-indexer/storage"
)

// ConsensusClient is a client to the consensus backends.
type ConsensusClient struct {
	client  consensus.ClientBackend
	network *config.Network
}

// GenesisDocument returns the original genesis document.
func (cc *ConsensusClient) GenesisDocument(ctx context.Context) (*genesisAPI.Document, error) {
	doc, err := cc.client.GetGenesisDocument(ctx)
	if err != nil {
		return nil, err
	}

	return doc, nil
}

// GenesisDocumentAtHeight returns the genesis document at the provided height.
func (cc *ConsensusClient) GenesisDocumentAtHeight(ctx context.Context, height int64) (*genesisAPI.Document, error) {
	doc, err := cc.client.StateToGenesis(ctx, height)
	if err != nil {
		return nil, err
	}

	return doc, nil
}

// RegistryGenesis returns the registry genesis document at the provided height.
func (cc *ConsensusClient) RegistryGenesis(ctx context.Context, height int64) (*registryAPI.Genesis, error) {
	doc, err := cc.client.Registry().StateToGenesis(ctx, height)
	if err != nil {
		return nil, err
	}

	return doc, nil
}

// StakingGenesis returns the staking genesis document at the provided height.
func (cc *ConsensusClient) StakingGenesis(ctx context.Context, height int64) (*stakingAPI.Genesis, error) {
	doc, err := cc.client.Staking().StateToGenesis(ctx, height)
	if err != nil {
		return nil, err
	}

	return doc, nil
}

// SchedulerGenesis returns the scheduler genesis document at the provided height.
func (cc *ConsensusClient) SchedulerGenesis(ctx context.Context, height int64) (*schedulerAPI.Genesis, error) {
	doc, err := cc.client.Scheduler().StateToGenesis(ctx, height)
	if err != nil {
		return nil, err
	}

	return doc, nil
}

// GovernanceGenesis returns the governance genesis document at the provided height.
func (cc *ConsensusClient) GovernanceGenesis(ctx context.Context, height int64) (*governanceAPI.Genesis, error) {
	doc, err := cc.client.Governance().StateToGenesis(ctx, height)
	if err != nil {
		return nil, err
	}

	return doc, nil
}

// Name returns the name of the client, for the ConsensusSourceStorage interface.
func (cc *ConsensusClient) Name() string {
	return fmt.Sprintf("%s_consensus", moduleName)
}

// BlockData retrieves data about a consensus block at the provided block height.
func (cc *ConsensusClient) BlockData(ctx context.Context, height int64) (*storage.ConsensusBlockData, error) {
	block, err := cc.client.GetBlock(ctx, height)
	if err != nil {
		return nil, err
	}

	epoch, err := cc.client.Beacon().GetEpoch(ctx, height)
	if err != nil {
		return nil, err
	}

	transactionsWithResults, err := cc.client.GetTransactionsWithResults(ctx, height)
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

	return &storage.ConsensusBlockData{
		BlockHeader:  block,
		Epoch:        epoch,
		Transactions: transactions,
		Results:      transactionsWithResults.Results,
	}, nil
}

// BeaconData retrieves the beacon for the provided block height.
// NOTE: The random beacon endpoint is in flux.
func (cc *ConsensusClient) BeaconData(ctx context.Context, height int64) (*storage.BeaconData, error) {
	beacon, err := cc.client.Beacon().GetBeacon(ctx, height)
	if err != nil {
		return nil, err
	}

	epoch, err := cc.client.Beacon().GetEpoch(ctx, height)
	if err != nil {
		return nil, err
	}

	return &storage.BeaconData{
		Epoch:  epoch,
		Beacon: beacon,
	}, nil
}

// RegistryData retrieves registry events at the provided block height.
func (cc *ConsensusClient) RegistryData(ctx context.Context, height int64) (*storage.RegistryData, error) {
	events, err := cc.client.Registry().GetEvents(ctx, height)
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

	rts, err := cc.runtimeUpdates(ctx, height)
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

// runtimeUpdates gets runtimes that have seen status changes since the previous block.
func (cc *ConsensusClient) runtimeUpdates(ctx context.Context, height int64) (map[string]bool, error) {
	rtsCurr, err := cc.runtimes(ctx, height)
	if err != nil {
		return nil, err
	}

	doc, err := cc.client.GetGenesisDocument(ctx)
	if err != nil {
		return nil, err
	}

	if height != doc.Height {
		rtsPrev, err := cc.runtimes(ctx, height-1)
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

func (cc *ConsensusClient) runtimes(ctx context.Context, height int64) (map[string]bool, error) {
	allRuntimes, err := cc.client.Registry().GetRuntimes(ctx, &registryAPI.GetRuntimesQuery{
		Height:           height,
		IncludeSuspended: true,
	})
	if err != nil {
		return nil, err
	}
	runtimes, err := cc.client.Registry().GetRuntimes(ctx, &registryAPI.GetRuntimesQuery{
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
func (cc *ConsensusClient) StakingData(ctx context.Context, height int64) (*storage.StakingData, error) {
	events, err := cc.client.Staking().GetEvents(ctx, height)
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
func (cc *ConsensusClient) SchedulerData(ctx context.Context, height int64) (*storage.SchedulerData, error) {
	validators, err := cc.client.Scheduler().GetValidators(ctx, height)
	if err != nil {
		return nil, err
	}

	committees := make(map[common.Namespace][]*schedulerAPI.Committee, len(cc.network.ParaTimes.All))

	for name := range cc.network.ParaTimes.All {
		var runtimeID common.Namespace
		if err := runtimeID.UnmarshalHex(cc.network.ParaTimes.All[name].ID); err != nil {
			return nil, err
		}

		consensusCommittees, err := cc.client.Scheduler().GetCommittees(ctx, &schedulerAPI.GetCommitteesRequest{
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
func (cc *ConsensusClient) GovernanceData(ctx context.Context, height int64) (*storage.GovernanceData, error) {
	events, err := cc.client.Governance().GetEvents(ctx, height)
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
			proposal, err := cc.client.Governance().Proposal(ctx, &governanceAPI.ProposalQuery{
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
			proposal, err := cc.client.Governance().Proposal(ctx, &governanceAPI.ProposalQuery{
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
