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
	txResultsDamask "github.com/oasisprotocol/oasis-core/go/consensus/api/transaction/results"
)

func convertEvent(e txResultsDamask.Event) nodeapi.Event {
	ret := nodeapi.Event{}
	switch {
	case e.Staking != nil:
		switch {
		case e.Staking.Transfer != nil:
			ret = nodeapi.Event{
				StakingTransfer: (*nodeapi.TransferEvent)(e.Staking.Transfer),
				RawBody:         common.TryAsJSON(e.Staking.Transfer),
				Type:            apiTypes.ConsensusEventTypeStakingTransfer,
			}
		case e.Staking.Burn != nil:
			ret = nodeapi.Event{
				StakingBurn: (*nodeapi.BurnEvent)(e.Staking.Burn),
				RawBody:     common.TryAsJSON(e.Staking.Burn),
				Type:        apiTypes.ConsensusEventTypeStakingBurn,
			}
		case e.Staking.Escrow != nil:
			switch {
			case e.Staking.Escrow.Add != nil:
				ret = nodeapi.Event{
					StakingAddEscrow: (*nodeapi.AddEscrowEvent)(e.Staking.Escrow.Add),
					RawBody:          common.TryAsJSON(e.Staking.Escrow.Add),
					Type:             apiTypes.ConsensusEventTypeStakingEscrowAdd,
				}
			case e.Staking.Escrow.Take != nil:
				ret = nodeapi.Event{
					StakingTakeEscrow: (*nodeapi.TakeEscrowEvent)(e.Staking.Escrow.Take),
					RawBody:           common.TryAsJSON(e.Staking.Escrow.Take),
					Type:              apiTypes.ConsensusEventTypeStakingEscrowTake,
				}
			case e.Staking.Escrow.Reclaim != nil:
				ret = nodeapi.Event{
					StakingReclaimEscrow: (*nodeapi.ReclaimEscrowEvent)(e.Staking.Escrow.Reclaim),
					RawBody:              common.TryAsJSON(e.Staking.Escrow.Reclaim),
					Type:                 apiTypes.ConsensusEventTypeStakingEscrowReclaim,
				}
			case e.Staking.Escrow.DebondingStart != nil:
				ret = nodeapi.Event{
					StakingDebondingStart: (*nodeapi.DebondingStartEscrowEvent)(e.Staking.Escrow.DebondingStart),
					RawBody:               common.TryAsJSON(e.Staking.Escrow.DebondingStart),
					Type:                  apiTypes.ConsensusEventTypeStakingEscrowDebondingStart,
				}
			}
		case e.Staking.AllowanceChange != nil:
			ret = nodeapi.Event{
				StakingAllowanceChange: (*nodeapi.AllowanceChangeEvent)(e.Staking.AllowanceChange),
				RawBody:                common.TryAsJSON(e.Staking.AllowanceChange),
				Type:                   apiTypes.ConsensusEventTypeStakingAllowanceChange,
			}
		}
		ret.Height = e.Staking.Height
		ret.TxHash = e.Staking.TxHash
		// End Staking.
	case e.Registry != nil:
		switch {
		case e.Registry.RuntimeEvent != nil && e.Registry.RuntimeEvent.Runtime != nil:
			ret = nodeapi.Event{
				RegistryRuntimeRegistered: &nodeapi.RuntimeRegisteredEvent{
					ID:          e.Registry.RuntimeEvent.Runtime.ID,
					EntityID:    e.Registry.RuntimeEvent.Runtime.EntityID,
					Kind:        e.Registry.RuntimeEvent.Runtime.Kind.String(),
					KeyManager:  e.Registry.RuntimeEvent.Runtime.KeyManager,
					TEEHardware: e.Registry.RuntimeEvent.Runtime.TEEHardware.String(),
				},
				RawBody: common.TryAsJSON(e.Registry.RuntimeEvent),
				Type:    apiTypes.ConsensusEventTypeRegistryRuntime,
			}
		case e.Registry.EntityEvent != nil:
			ret = nodeapi.Event{
				RegistryEntity: (*nodeapi.EntityEvent)(e.Registry.EntityEvent),
				RawBody:        common.TryAsJSON(e.Registry.EntityEvent),
				Type:           apiTypes.ConsensusEventTypeRegistryEntity,
			}
		case e.Registry.NodeEvent != nil:
			var vrfID *signature.PublicKey
			if e.Registry.NodeEvent.Node.VRF != nil {
				vrfID = &e.Registry.NodeEvent.Node.VRF.ID
			}
			runtimeIDs := make([]coreCommon.Namespace, len(e.Registry.NodeEvent.Node.Runtimes))
			for i, r := range e.Registry.NodeEvent.Node.Runtimes {
				runtimeIDs[i] = r.ID
			}
			tlsAddresses := make([]string, len(e.Registry.NodeEvent.Node.TLS.Addresses))
			for i, a := range e.Registry.NodeEvent.Node.TLS.Addresses {
				tlsAddresses[i] = a.String()
			}
			p2pAddresses := make([]string, len(e.Registry.NodeEvent.Node.P2P.Addresses))
			for i, a := range e.Registry.NodeEvent.Node.P2P.Addresses {
				p2pAddresses[i] = a.String()
			}
			consensusAddresses := make([]string, len(e.Registry.NodeEvent.Node.Consensus.Addresses))
			for i, a := range e.Registry.NodeEvent.Node.Consensus.Addresses {
				consensusAddresses[i] = a.String()
			}
			ret = nodeapi.Event{
				RegistryNode: &nodeapi.NodeEvent{
					NodeID:             e.Registry.NodeEvent.Node.ID,
					EntityID:           e.Registry.NodeEvent.Node.EntityID,
					Expiration:         e.Registry.NodeEvent.Node.Expiration,
					VRFPubKey:          vrfID,
					TLSAddresses:       tlsAddresses,
					TLSPubKey:          e.Registry.NodeEvent.Node.TLS.PubKey,
					TLSNextPubKey:      e.Registry.NodeEvent.Node.TLS.NextPubKey,
					P2PID:              e.Registry.NodeEvent.Node.P2P.ID,
					P2PAddresses:       p2pAddresses,
					RuntimeIDs:         runtimeIDs,
					ConsensusID:        e.Registry.NodeEvent.Node.Consensus.ID,
					ConsensusAddresses: consensusAddresses,
					IsRegistration:     e.Registry.NodeEvent.IsRegistration,
					Roles:              strings.Split(e.Registry.NodeEvent.Node.Roles.String(), ","),
					SoftwareVersion:    e.Registry.NodeEvent.Node.SoftwareVersion,
				},
				RawBody: common.TryAsJSON(e.Registry.NodeEvent),
				Type:    apiTypes.ConsensusEventTypeRegistryNode,
			}
		case e.Registry.NodeUnfrozenEvent != nil:
			ret = nodeapi.Event{
				RegistryNodeUnfrozen: (*nodeapi.NodeUnfrozenEvent)(e.Registry.NodeUnfrozenEvent),
				RawBody:              common.TryAsJSON(e.Registry.NodeUnfrozenEvent),
				Type:                 apiTypes.ConsensusEventTypeRegistryNodeUnfrozen,
			}
		}
		ret.Height = e.Registry.Height
		ret.TxHash = e.Registry.TxHash
		// End Registry.
	case e.RootHash != nil:
		switch {
		case e.RootHash.ExecutorCommitted != nil:
			ret = nodeapi.Event{
				RoothashExecutorCommitted: &nodeapi.ExecutorCommittedEvent{
					NodeID: &e.RootHash.ExecutorCommitted.Commit.NodeID,
				},
				RawBody: common.TryAsJSON(e.RootHash.ExecutorCommitted),
				Type:    apiTypes.ConsensusEventTypeRoothashExecutorCommitted,
			}
		case e.RootHash.ExecutionDiscrepancyDetected != nil:
			ret = nodeapi.Event{
				RawBody: common.TryAsJSON(e.RootHash.ExecutionDiscrepancyDetected),
				Type:    apiTypes.ConsensusEventTypeRoothashExecutionDiscrepancy,
			}
		case e.RootHash.Finalized != nil:
			ret = nodeapi.Event{
				RawBody: common.TryAsJSON(e.RootHash.Finalized),
				Type:    apiTypes.ConsensusEventTypeRoothashFinalized,
			}
		}
		ret.Height = e.RootHash.Height
		ret.TxHash = e.RootHash.TxHash
		// End RootHash.
	case e.Governance != nil:
		switch {
		case e.Governance.ProposalSubmitted != nil:
			ret = nodeapi.Event{
				GovernanceProposalSubmitted: (*nodeapi.ProposalSubmittedEvent)(e.Governance.ProposalSubmitted),
				RawBody:                     common.TryAsJSON(e.Governance.ProposalSubmitted),
				Type:                        apiTypes.ConsensusEventTypeGovernanceProposalSubmitted,
			}
		case e.Governance.ProposalExecuted != nil:
			ret = nodeapi.Event{
				GovernanceProposalExecuted: (*nodeapi.ProposalExecutedEvent)(e.Governance.ProposalExecuted),
				RawBody:                    common.TryAsJSON(e.Governance.ProposalExecuted),
				Type:                       apiTypes.ConsensusEventTypeGovernanceProposalExecuted,
			}
		case e.Governance.ProposalFinalized != nil:
			ret = nodeapi.Event{
				GovernanceProposalFinalized: (*nodeapi.ProposalFinalizedEvent)(e.Governance.ProposalFinalized),
				RawBody:                     common.TryAsJSON(e.Governance.ProposalFinalized),
				Type:                        apiTypes.ConsensusEventTypeGovernanceProposalFinalized,
			}
		case e.Governance.Vote != nil:
			ret = nodeapi.Event{
				GovernanceVote: &nodeapi.VoteEvent{
					ID:        e.Governance.Vote.ID,
					Submitter: e.Governance.Vote.Submitter,
					Vote:      e.Governance.Vote.Vote.String(),
				},
				RawBody: common.TryAsJSON(e.Governance.Vote),
				Type:    apiTypes.ConsensusEventTypeGovernanceVote,
			}
		}
		ret.Height = e.Governance.Height
		ret.TxHash = e.Governance.TxHash
		// End Governance.
	}
	return ret
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
