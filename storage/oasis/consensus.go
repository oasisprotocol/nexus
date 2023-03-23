package oasis

import (
	"context"
	"fmt"

	beaconAPI "github.com/oasisprotocol/oasis-core/go/beacon/api"
	"github.com/oasisprotocol/oasis-core/go/common"
	genesisAPI "github.com/oasisprotocol/oasis-core/go/genesis/api"
	"github.com/oasisprotocol/oasis-indexer/storage/oasis/nodeapi"
	config "github.com/oasisprotocol/oasis-sdk/client-sdk/go/config"

	"github.com/oasisprotocol/oasis-indexer/storage"
)

// ConsensusClient is a client to the consensus methods/data of oasis node. It
// differs from the nodeapi.ConsensusApiLite in that:
//   - Its methods may collect data using multiple RPCs each.
//   - The return types make no effort to closely resemble oasis-core types
//     in structure. Instead, they are structured in a way that is most convenient
//     for the analyzer.
//     TODO: The benefits of this are miniscule, and introduce considerable
//     boilerplate. Consider removing most types from this package, and
//     using nodeapi types directly.
type ConsensusClient struct {
	nodeApi nodeapi.ConsensusApiLite
	network *config.Network
}

// GenesisDocument returns the original genesis document.
func (cc *ConsensusClient) GenesisDocument(ctx context.Context) (*genesisAPI.Document, error) {
	return cc.nodeApi.GetGenesisDocument(ctx)
}

// Name returns the name of the client, for the ConsensusSourceStorage interface.
func (cc *ConsensusClient) Name() string {
	return fmt.Sprintf("%s_consensus", moduleName)
}

// GetEpoch returns the epoch number at the specified block height.
func (cc *ConsensusClient) GetEpoch(ctx context.Context, height int64) (beaconAPI.EpochTime, error) {
	return cc.nodeApi.GetEpoch(ctx, height)
}

// AllData returns all relevant data related to the given height.
func (cc *ConsensusClient) AllData(ctx context.Context, height int64) (*storage.ConsensusAllData, error) {
	blockData, err := cc.BlockData(ctx, height)
	if err != nil {
		return nil, err
	}
	beaconData, err := cc.BeaconData(ctx, height)
	if err != nil {
		return nil, err
	}
	registryData, err := cc.RegistryData(ctx, height)
	if err != nil {
		return nil, err
	}
	stakingData, err := cc.StakingData(ctx, height)
	if err != nil {
		return nil, err
	}
	schedulerData, err := cc.SchedulerData(ctx, height)
	if err != nil {
		return nil, err
	}
	governanceData, err := cc.GovernanceData(ctx, height)
	if err != nil {
		return nil, err
	}
	rootHashData, err := cc.RootHashData(ctx, height)
	if err != nil {
		return nil, err
	}

	data := storage.ConsensusAllData{
		BlockData:      blockData,
		BeaconData:     beaconData,
		RegistryData:   registryData,
		RootHashData:   rootHashData,
		StakingData:    stakingData,
		SchedulerData:  schedulerData,
		GovernanceData: governanceData,
	}
	return &data, nil
}

// BlockData retrieves data about a consensus block at the provided block height.
func (cc *ConsensusClient) BlockData(ctx context.Context, height int64) (*storage.ConsensusBlockData, error) {
	block, err := cc.nodeApi.GetBlock(ctx, height)
	if err != nil {
		return nil, err
	}

	epoch, err := cc.GetEpoch(ctx, height)
	if err != nil {
		return nil, err
	}

	transactionsWithResults, err := cc.nodeApi.GetTransactionsWithResults(ctx, height)
	if err != nil {
		return nil, err
	}

	return &storage.ConsensusBlockData{
		Height:                  height,
		BlockHeader:             block,
		Epoch:                   epoch,
		TransactionsWithResults: transactionsWithResults,
	}, nil
}

// BeaconData retrieves the beacon for the provided block height.
func (cc *ConsensusClient) BeaconData(ctx context.Context, height int64) (*storage.BeaconData, error) {
	// NOTE: The random beacon endpoint is in flux.
	// GetBeacon() consistently errors out (at least for heights soon after Damask genesis) with "beacon: random beacon not available".
	// beacon, err := cc.client.Beacon().GetBeacon(ctx, height)
	// if err != nil {
	// 	return nil, err
	// }

	epoch, err := cc.GetEpoch(ctx, height)
	if err != nil {
		return nil, err
	}

	return &storage.BeaconData{
		Height: height,
		Epoch:  epoch,
		Beacon: nil,
	}, nil
}

// RegistryData retrieves registry events at the provided block height.
func (cc *ConsensusClient) RegistryData(ctx context.Context, height int64) (*storage.RegistryData, error) {
	events, err := cc.nodeApi.RegistryEvents(ctx, height)
	if err != nil {
		return nil, err
	}

	var runtimeEvents []nodeapi.RuntimeEvent
	var entityEvents []nodeapi.EntityEvent
	var nodeEvents []nodeapi.NodeEvent
	var nodeUnfrozenEvents []nodeapi.NodeUnfrozenEvent

	for _, event := range events {
		switch e := event; {
		case e.RegistryRuntime != nil:
			runtimeEvents = append(runtimeEvents, *e.RegistryRuntime)
		case e.RegistryEntity != nil:
			entityEvents = append(entityEvents, *e.RegistryEntity)
		case e.RegistryNode != nil:
			nodeEvents = append(nodeEvents, *e.RegistryNode)
		case e.RegistryNodeUnfrozen != nil:
			nodeUnfrozenEvents = append(nodeUnfrozenEvents, *e.RegistryNodeUnfrozen)
		}
	}

	return &storage.RegistryData{
		Height:             height,
		Events:             events,
		RuntimeEvents:      runtimeEvents,
		EntityEvents:       entityEvents,
		NodeEvents:         nodeEvents,
		NodeUnfrozenEvents: nodeUnfrozenEvents,
	}, nil
}

// StakingData retrieves staking events at the provided block height.
func (cc *ConsensusClient) StakingData(ctx context.Context, height int64) (*storage.StakingData, error) {
	events, err := cc.nodeApi.StakingEvents(ctx, height)
	if err != nil {
		return nil, err
	}

	epoch, err := cc.GetEpoch(ctx, height)
	if err != nil {
		return nil, err
	}

	var transfers []nodeapi.TransferEvent
	var burns []nodeapi.BurnEvent
	var addEscrows []nodeapi.AddEscrowEvent
	var reclaimEscrows []nodeapi.ReclaimEscrowEvent
	var debondingStartEscrows []nodeapi.DebondingStartEscrowEvent
	var takeEscrows []nodeapi.TakeEscrowEvent
	var allowanceChanges []nodeapi.AllowanceChangeEvent

	for _, event := range events {
		switch e := event; {
		case e.StakingTransfer != nil:
			transfers = append(transfers, *event.StakingTransfer)
		case e.StakingBurn != nil:
			burns = append(burns, *event.StakingBurn)
		case e.StakingAddEscrow != nil:
			addEscrows = append(addEscrows, *event.StakingAddEscrow)
		case e.StakingReclaimEscrow != nil:
			reclaimEscrows = append(reclaimEscrows, *event.StakingReclaimEscrow)
		case e.StakingDebondingStart != nil:
			debondingStartEscrows = append(debondingStartEscrows, *event.StakingDebondingStart)
		case e.StakingTakeEscrow != nil:
			takeEscrows = append(takeEscrows, *event.StakingTakeEscrow)
		case e.StakingAllowanceChange != nil:
			allowanceChanges = append(allowanceChanges, *event.StakingAllowanceChange)
		}
	}

	return &storage.StakingData{
		Height:                height,
		Epoch:                 epoch,
		Events:                events,
		Transfers:             transfers,
		Burns:                 burns,
		AddEscrows:            addEscrows,
		ReclaimEscrows:        reclaimEscrows,
		DebondingStartEscrows: debondingStartEscrows,
		TakeEscrows:           takeEscrows,
		AllowanceChanges:      allowanceChanges,
	}, nil
}

// SchedulerData retrieves validators and runtime committees at the provided block height.
func (cc *ConsensusClient) SchedulerData(ctx context.Context, height int64) (*storage.SchedulerData, error) {
	validators, err := cc.nodeApi.GetValidators(ctx, height)
	if err != nil {
		return nil, err
	}

	committees := make(map[common.Namespace][]nodeapi.Committee, len(cc.network.ParaTimes.All))

	for name := range cc.network.ParaTimes.All {
		var runtimeID common.Namespace
		if err := runtimeID.UnmarshalHex(cc.network.ParaTimes.All[name].ID); err != nil {
			return nil, err
		}

		consensusCommittees, err := cc.nodeApi.GetCommittees(ctx, height, runtimeID)
		if err != nil {
			return nil, err
		}
		committees[runtimeID] = consensusCommittees
	}

	return &storage.SchedulerData{
		Height:     height,
		Validators: validators,
		Committees: committees,
	}, nil
}

// GovernanceData retrieves governance events at the provided block height.
func (cc *ConsensusClient) GovernanceData(ctx context.Context, height int64) (*storage.GovernanceData, error) {
	events, err := cc.nodeApi.GovernanceEvents(ctx, height)
	if err != nil {
		return nil, err
	}

	var submissions []nodeapi.Proposal
	var executions []nodeapi.ProposalExecutedEvent
	var finalizations []nodeapi.Proposal
	var votes []nodeapi.VoteEvent

	for _, event := range events {
		switch {
		case event.GovernanceProposalSubmitted != nil:
			proposal, err := cc.nodeApi.GetProposal(ctx, height, event.GovernanceProposalSubmitted.ID)
			if err != nil {
				return nil, err
			}
			submissions = append(submissions, *proposal)
		case event.GovernanceProposalExecuted != nil:
			executions = append(executions, *event.GovernanceProposalExecuted)
		case event.GovernanceProposalFinalized != nil:
			proposal, err := cc.nodeApi.GetProposal(ctx, height, event.GovernanceProposalFinalized.ID)
			if err != nil {
				return nil, err
			}
			finalizations = append(finalizations, *proposal)
		case event.GovernanceVote != nil:
			votes = append(votes, *event.GovernanceVote)
		}
	}
	return &storage.GovernanceData{
		Height:                height,
		Events:                events,
		ProposalSubmissions:   submissions,
		ProposalExecutions:    executions,
		ProposalFinalizations: finalizations,
		Votes:                 votes,
	}, nil
}

// RootHashData retrieves roothash events at the provided block height.
func (cc *ConsensusClient) RootHashData(ctx context.Context, height int64) (*storage.RootHashData, error) {
	events, err := cc.nodeApi.RoothashEvents(ctx, height)
	if err != nil {
		return nil, err
	}

	return &storage.RootHashData{
		Height: height,
		Events: events,
	}, nil
}
