package damask

import (
	"strings"

	// nexus-internal data types.
	coreCommon "github.com/oasisprotocol/oasis-core/go/common"
	"github.com/oasisprotocol/oasis-core/go/common/crypto/signature"

	apiTypes "github.com/oasisprotocol/nexus/api/v1/types"
	"github.com/oasisprotocol/nexus/common"
	"github.com/oasisprotocol/nexus/storage/oasis/nodeapi"

	// data types for Damask gRPC APIs.
	txResultsDamask "github.com/oasisprotocol/nexus/coreapi/v22.2.11/consensus/api/transaction/results"
	governanceDamask "github.com/oasisprotocol/nexus/coreapi/v22.2.11/governance/api"
	registryDamask "github.com/oasisprotocol/nexus/coreapi/v22.2.11/registry/api"
	roothashDamask "github.com/oasisprotocol/nexus/coreapi/v22.2.11/roothash/api"
	stakingDamask "github.com/oasisprotocol/nexus/coreapi/v22.2.11/staking/api"
)

func convertStakingEvent(e stakingDamask.Event) nodeapi.Event {
	ret := nodeapi.Event{}
	switch {
	case e.Transfer != nil:
		ret = nodeapi.Event{
			StakingTransfer: (*nodeapi.TransferEvent)(e.Transfer),
			RawBody:         common.TryAsJSON(e.Transfer),
			Type:            apiTypes.ConsensusEventTypeStakingTransfer,
		}
	case e.Burn != nil:
		ret = nodeapi.Event{
			StakingBurn: (*nodeapi.BurnEvent)(e.Burn),
			RawBody:     common.TryAsJSON(e.Burn),
			Type:        apiTypes.ConsensusEventTypeStakingBurn,
		}
	case e.Escrow != nil:
		switch {
		case e.Escrow.Add != nil:
			ret = nodeapi.Event{
				StakingAddEscrow: (*nodeapi.AddEscrowEvent)(e.Escrow.Add),
				RawBody:          common.TryAsJSON(e.Escrow.Add),
				Type:             apiTypes.ConsensusEventTypeStakingEscrowAdd,
			}
		case e.Escrow.Take != nil:
			ret = nodeapi.Event{
				StakingTakeEscrow: &nodeapi.TakeEscrowEvent{
					Owner:           e.Escrow.Take.Owner,
					Amount:          e.Escrow.Take.Amount,
					DebondingAmount: nil, // Not present in Cobalt and Damask.
				},
				RawBody: common.TryAsJSON(e.Escrow.Take),
				Type:    apiTypes.ConsensusEventTypeStakingEscrowTake,
			}
		case e.Escrow.Reclaim != nil:
			ret = nodeapi.Event{
				StakingReclaimEscrow: (*nodeapi.ReclaimEscrowEvent)(e.Escrow.Reclaim),
				RawBody:              common.TryAsJSON(e.Escrow.Reclaim),
				Type:                 apiTypes.ConsensusEventTypeStakingEscrowReclaim,
			}
		case e.Escrow.DebondingStart != nil:
			ret = nodeapi.Event{
				StakingDebondingStart: (*nodeapi.DebondingStartEscrowEvent)(e.Escrow.DebondingStart),
				RawBody:               common.TryAsJSON(e.Escrow.DebondingStart),
				Type:                  apiTypes.ConsensusEventTypeStakingEscrowDebondingStart,
			}
		}
	case e.AllowanceChange != nil:
		ret = nodeapi.Event{
			StakingAllowanceChange: (*nodeapi.AllowanceChangeEvent)(e.AllowanceChange),
			RawBody:                common.TryAsJSON(e.AllowanceChange),
			Type:                   apiTypes.ConsensusEventTypeStakingAllowanceChange,
		}
	}
	ret.Height = e.Height
	ret.TxHash = e.TxHash
	return ret
}

func convertRegistryEvent(e registryDamask.Event) nodeapi.Event {
	ret := nodeapi.Event{}
	switch {
	case e.RuntimeEvent != nil && e.RuntimeEvent.Runtime != nil:
		ret = nodeapi.Event{
			RegistryRuntimeStarted: &nodeapi.RuntimeStartedEvent{
				ID:          e.RuntimeEvent.Runtime.ID,
				EntityID:    e.RuntimeEvent.Runtime.EntityID,
				Kind:        e.RuntimeEvent.Runtime.Kind.String(),
				KeyManager:  e.RuntimeEvent.Runtime.KeyManager,
				TEEHardware: e.RuntimeEvent.Runtime.TEEHardware.String(),
			},
			RawBody: common.TryAsJSON(e.RuntimeEvent),
			Type:    apiTypes.ConsensusEventTypeRegistryRuntime,
		}
	case e.EntityEvent != nil:
		ret = nodeapi.Event{
			RegistryEntity: (*nodeapi.EntityEvent)(e.EntityEvent),
			RawBody:        common.TryAsJSON(e.EntityEvent),
			Type:           apiTypes.ConsensusEventTypeRegistryEntity,
		}
	case e.NodeEvent != nil:
		var vrfID *signature.PublicKey
		if e.NodeEvent.Node.VRF != nil {
			vrfID = &e.NodeEvent.Node.VRF.ID
		}
		runtimeIDs := make([]coreCommon.Namespace, len(e.NodeEvent.Node.Runtimes))
		for i, r := range e.NodeEvent.Node.Runtimes {
			runtimeIDs[i] = r.ID
		}
		tlsAddresses := make([]string, len(e.NodeEvent.Node.TLS.Addresses))
		for i, a := range e.NodeEvent.Node.TLS.Addresses {
			tlsAddresses[i] = a.String()
		}
		p2pAddresses := make([]string, len(e.NodeEvent.Node.P2P.Addresses))
		for i, a := range e.NodeEvent.Node.P2P.Addresses {
			p2pAddresses[i] = a.String()
		}
		consensusAddresses := make([]string, len(e.NodeEvent.Node.Consensus.Addresses))
		for i, a := range e.NodeEvent.Node.Consensus.Addresses {
			consensusAddresses[i] = a.String()
		}
		ret = nodeapi.Event{
			RegistryNode: &nodeapi.NodeEvent{
				NodeID:             e.NodeEvent.Node.ID,
				EntityID:           e.NodeEvent.Node.EntityID,
				Expiration:         e.NodeEvent.Node.Expiration,
				VRFPubKey:          vrfID,
				TLSAddresses:       tlsAddresses,
				TLSPubKey:          e.NodeEvent.Node.TLS.PubKey,
				TLSNextPubKey:      e.NodeEvent.Node.TLS.NextPubKey,
				P2PID:              e.NodeEvent.Node.P2P.ID,
				P2PAddresses:       p2pAddresses,
				RuntimeIDs:         runtimeIDs,
				ConsensusID:        e.NodeEvent.Node.Consensus.ID,
				ConsensusAddresses: consensusAddresses,
				IsRegistration:     e.NodeEvent.IsRegistration,
				Roles:              strings.Split(e.NodeEvent.Node.Roles.String(), ","),
				SoftwareVersion:    e.NodeEvent.Node.SoftwareVersion,
			},
			RawBody: common.TryAsJSON(e.NodeEvent),
			Type:    apiTypes.ConsensusEventTypeRegistryNode,
		}
	case e.NodeUnfrozenEvent != nil:
		ret = nodeapi.Event{
			RegistryNodeUnfrozen: (*nodeapi.NodeUnfrozenEvent)(e.NodeUnfrozenEvent),
			RawBody:              common.TryAsJSON(e.NodeUnfrozenEvent),
			Type:                 apiTypes.ConsensusEventTypeRegistryNodeUnfrozen,
		}
	}
	ret.Height = e.Height
	ret.TxHash = e.TxHash
	return ret
}

func convertRoothashEvent(e roothashDamask.Event) nodeapi.Event {
	ret := nodeapi.Event{}
	switch {
	case e.ExecutorCommitted != nil:
		ret = nodeapi.Event{
			RoothashExecutorCommitted: &nodeapi.ExecutorCommittedEvent{
				RuntimeID: e.RuntimeID,
				Round:     e.ExecutorCommitted.Commit.Header.Round,
				NodeID:    &e.ExecutorCommitted.Commit.NodeID,
			},
			RawBody: common.TryAsJSON(e.ExecutorCommitted),
			Type:    apiTypes.ConsensusEventTypeRoothashExecutorCommitted,
		}
	case e.ExecutionDiscrepancyDetected != nil:
		ret = nodeapi.Event{
			RoothashMisc: &nodeapi.RoothashEvent{
				RuntimeID: e.RuntimeID,
			},
			RawBody: common.TryAsJSON(e.ExecutionDiscrepancyDetected),
			Type:    apiTypes.ConsensusEventTypeRoothashExecutionDiscrepancy,
		}
	case e.Finalized != nil:
		ret = nodeapi.Event{
			RoothashMisc: &nodeapi.RoothashEvent{
				RuntimeID: e.RuntimeID,
				Round:     &e.Finalized.Round,
			},
			RawBody: common.TryAsJSON(e.Finalized),
			Type:    apiTypes.ConsensusEventTypeRoothashFinalized,
		}
	case e.InMsgProcessed != nil:
		ret = nodeapi.Event{
			RoothashMisc: &nodeapi.RoothashEvent{
				RuntimeID: e.RuntimeID,
				Round:     &e.InMsgProcessed.Round,
			},
			RawBody: common.TryAsJSON(e.InMsgProcessed),
			Type:    apiTypes.ConsensusEventTypeRoothashInMsgProcessed,
		}
	}
	ret.Height = e.Height
	ret.TxHash = e.TxHash
	return ret
}

func convertGovernanceEvent(e governanceDamask.Event) nodeapi.Event {
	ret := nodeapi.Event{}
	switch {
	case e.ProposalSubmitted != nil:
		ret = nodeapi.Event{
			GovernanceProposalSubmitted: (*nodeapi.ProposalSubmittedEvent)(e.ProposalSubmitted),
			RawBody:                     common.TryAsJSON(e.ProposalSubmitted),
			Type:                        apiTypes.ConsensusEventTypeGovernanceProposalSubmitted,
		}
	case e.ProposalExecuted != nil:
		ret = nodeapi.Event{
			GovernanceProposalExecuted: (*nodeapi.ProposalExecutedEvent)(e.ProposalExecuted),
			RawBody:                    common.TryAsJSON(e.ProposalExecuted),
			Type:                       apiTypes.ConsensusEventTypeGovernanceProposalExecuted,
		}
	case e.ProposalFinalized != nil:
		ret = nodeapi.Event{
			GovernanceProposalFinalized: (*nodeapi.ProposalFinalizedEvent)(e.ProposalFinalized),
			RawBody:                     common.TryAsJSON(e.ProposalFinalized),
			Type:                        apiTypes.ConsensusEventTypeGovernanceProposalFinalized,
		}
	case e.Vote != nil:
		ret = nodeapi.Event{
			GovernanceVote: &nodeapi.VoteEvent{
				ID:        e.Vote.ID,
				Submitter: e.Vote.Submitter,
				Vote:      e.Vote.Vote.String(),
			},
			RawBody: common.TryAsJSON(e.Vote),
			Type:    apiTypes.ConsensusEventTypeGovernanceVote,
		}
	}
	ret.Height = e.Height
	ret.TxHash = e.TxHash
	return ret
}

func convertEvent(e txResultsDamask.Event) nodeapi.Event {
	switch {
	case e.Staking != nil:
		return convertStakingEvent(*e.Staking)
	case e.Registry != nil:
		return convertRegistryEvent(*e.Registry)
	case e.RootHash != nil:
		return convertRoothashEvent(*e.RootHash)
	case e.Governance != nil:
		return convertGovernanceEvent(*e.Governance)
	default:
		return nodeapi.Event{}
	}
}

func convertTxResult(r txResultsDamask.Result) nodeapi.TxResult {
	events := make([]nodeapi.Event, len(r.Events))
	for i, e := range r.Events {
		events[i] = convertEvent(*e)
	}

	return nodeapi.TxResult{
		Error:  nodeapi.TxError(r.Error),
		Events: events,
	}
}
