// Package consensus implements an analyzer for the consensus layer.
package consensus

import (
	"context"

	"github.com/oasisprotocol/nexus/storage"
	"github.com/oasisprotocol/nexus/storage/oasis/nodeapi"
	"github.com/oasisprotocol/oasis-core/go/common"
	sdkConfig "github.com/oasisprotocol/oasis-sdk/client-sdk/go/config"
)

// AllData returns all relevant data related to the given height.
func AllData(ctx context.Context, cc nodeapi.ConsensusApiLite, network sdkConfig.Network, height int64, fastSync bool) (*storage.ConsensusAllData, error) {
	blockData, err := BlockData(ctx, cc, height)
	if err != nil {
		return nil, err
	}

	beaconData, err := BeaconData(ctx, cc, height)
	if err != nil {
		return nil, err
	}

	registryData, err := RegistryData(ctx, cc, height)
	if err != nil {
		return nil, err
	}

	stakingData, err := StakingData(ctx, cc, height)
	if err != nil {
		return nil, err
	}

	var schedulerData *storage.SchedulerData
	// Scheduler data is not needed during fast sync. It contains no events,
	// only a complete snapshot validators/committees. Since we don't store historical data,
	// any single snapshot during slow-sync is sufficient to reconstruct the state.
	if !fastSync {
		schedulerData, err = SchedulerData(ctx, cc, network, height)
		if err != nil {
			return nil, err
		}
	}

	governanceData, err := GovernanceData(ctx, cc, height)
	if err != nil {
		return nil, err
	}

	rootHashData, err := RootHashData(ctx, cc, height)
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
func BlockData(ctx context.Context, cc nodeapi.ConsensusApiLite, height int64) (*storage.ConsensusBlockData, error) {
	block, err := cc.GetBlock(ctx, height)
	if err != nil {
		return nil, err
	}

	epoch, err := cc.GetEpoch(ctx, height)
	if err != nil {
		return nil, err
	}

	transactionsWithResults, err := cc.GetTransactionsWithResults(ctx, height)
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
func BeaconData(ctx context.Context, cc nodeapi.ConsensusApiLite, height int64) (*storage.BeaconData, error) {
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
func RegistryData(ctx context.Context, cc nodeapi.ConsensusApiLite, height int64) (*storage.RegistryData, error) {
	events, err := cc.RegistryEvents(ctx, height)
	if err != nil {
		return nil, err
	}

	var runtimeStartedEvents []nodeapi.RuntimeStartedEvent
	var runtimeSuspendedEvents []nodeapi.RuntimeSuspendedEvent
	var entityEvents []nodeapi.EntityEvent
	var nodeEvents []nodeapi.NodeEvent
	var nodeUnfrozenEvents []nodeapi.NodeUnfrozenEvent

	for _, event := range events {
		switch e := event; {
		case e.RegistryRuntimeStarted != nil:
			runtimeStartedEvents = append(runtimeStartedEvents, *e.RegistryRuntimeStarted)
		case e.RegistryRuntimeSuspended != nil:
			runtimeSuspendedEvents = append(runtimeSuspendedEvents, *e.RegistryRuntimeSuspended)
		case e.RegistryEntity != nil:
			entityEvents = append(entityEvents, *e.RegistryEntity)
		case e.RegistryNode != nil:
			nodeEvents = append(nodeEvents, *e.RegistryNode)
		case e.RegistryNodeUnfrozen != nil:
			nodeUnfrozenEvents = append(nodeUnfrozenEvents, *e.RegistryNodeUnfrozen)
		}
	}

	return &registryData{
		Height:                 height,
		Events:                 events,
		RuntimeStartedEvents:   runtimeStartedEvents,
		RuntimeSuspendedEvents: runtimeSuspendedEvents,
		EntityEvents:           entityEvents,
		NodeEvents:             nodeEvents,
		NodeUnfrozenEvents:     nodeUnfrozenEvents,
	}, nil
}

// StakingData retrieves staking events at the provided block height.
func StakingData(ctx context.Context, cc nodeapi.ConsensusApiLite, height int64) (*storage.StakingData, error) {
	events, err := cc.StakingEvents(ctx, height)
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
func SchedulerData(ctx context.Context, cc nodeapi.ConsensusApiLite, network sdkConfig.Network, height int64) (*storage.SchedulerData, error) {
	validators, err := cc.GetValidators(ctx, height)
	if err != nil {
		return nil, err
	}

	committees := make(map[common.Namespace][]nodeapi.Committee, len(network.ParaTimes.All))

	for name := range network.ParaTimes.All {
		var runtimeID common.Namespace
		if err := runtimeID.UnmarshalHex(network.ParaTimes.All[name].ID); err != nil {
			return nil, err
		}

		consensusCommittees, err := cc.GetCommittees(ctx, height, runtimeID)
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
func GovernanceData(ctx context.Context, cc nodeapi.ConsensusApiLite, height int64) (*storage.GovernanceData, error) {
	events, err := cc.GovernanceEvents(ctx, height)
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
			proposal, err := cc.GetProposal(ctx, height, event.GovernanceProposalSubmitted.ID)
			if err != nil {
				return nil, err
			}
			submissions = append(submissions, *proposal)
		case event.GovernanceProposalExecuted != nil:
			executions = append(executions, *event.GovernanceProposalExecuted)
		case event.GovernanceProposalFinalized != nil:
			proposal, err := cc.GetProposal(ctx, height, event.GovernanceProposalFinalized.ID)
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
func RootHashData(ctx context.Context, cc nodeapi.ConsensusApiLite, height int64) (*storage.RootHashData, error) {
	events, err := cc.RoothashEvents(ctx, height)
	if err != nil {
		return nil, err
	}

	return &storage.RootHashData{
		Height: height,
		Events: events,
	}, nil
}
