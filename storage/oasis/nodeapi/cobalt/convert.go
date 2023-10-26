package cobalt

import (
	"strings"

	"github.com/oasisprotocol/oasis-core/go/common/crypto/hash"
	"github.com/oasisprotocol/oasis-core/go/common/crypto/signature"
	"github.com/oasisprotocol/oasis-core/go/common/quantity"

	consensus "github.com/oasisprotocol/nexus/coreapi/v22.2.11/consensus/api"

	// nexus-internal data types.
	coreCommon "github.com/oasisprotocol/oasis-core/go/common"

	genesis "github.com/oasisprotocol/nexus/coreapi/v22.2.11/genesis/api"
	governance "github.com/oasisprotocol/nexus/coreapi/v22.2.11/governance/api"
	registry "github.com/oasisprotocol/nexus/coreapi/v22.2.11/registry/api"
	scheduler "github.com/oasisprotocol/nexus/coreapi/v22.2.11/scheduler/api"
	staking "github.com/oasisprotocol/nexus/coreapi/v22.2.11/staking/api"
	upgrade "github.com/oasisprotocol/nexus/coreapi/v22.2.11/upgrade/api"

	apiTypes "github.com/oasisprotocol/nexus/api/v1/types"
	"github.com/oasisprotocol/nexus/common"
	"github.com/oasisprotocol/nexus/storage/oasis/nodeapi"

	// data types for Cobalt gRPC APIs.
	consensusCobalt "github.com/oasisprotocol/nexus/coreapi/v21.1.1/consensus/api"
	txResultsCobalt "github.com/oasisprotocol/nexus/coreapi/v21.1.1/consensus/api/transaction/results"
	genesisCobalt "github.com/oasisprotocol/nexus/coreapi/v21.1.1/genesis/api"
	governanceCobalt "github.com/oasisprotocol/nexus/coreapi/v21.1.1/governance/api"
	registryCobalt "github.com/oasisprotocol/nexus/coreapi/v21.1.1/registry/api"
	schedulerCobalt "github.com/oasisprotocol/nexus/coreapi/v21.1.1/scheduler/api"
	stakingCobalt "github.com/oasisprotocol/nexus/coreapi/v21.1.1/staking/api"
)

func convertProposal(p *governanceCobalt.Proposal) *governance.Proposal {
	results := make(map[governance.Vote]quantity.Quantity)
	for k, v := range p.Results {
		results[governance.Vote(k)] = v
	}

	return &governance.Proposal{
		ID:        p.ID,
		Submitter: p.Submitter,
		State:     governance.ProposalState(p.State),
		Deposit:   p.Deposit,
		Content: governance.ProposalContent{
			Upgrade: &governance.UpgradeProposal{
				Descriptor: upgrade.Descriptor{
					Versioned: p.Content.Upgrade.Descriptor.Versioned,
					Handler:   upgrade.HandlerName(p.Content.Upgrade.Descriptor.Handler),
					Target:    p.Content.Upgrade.Descriptor.Target,
					Epoch:     p.Content.Upgrade.Descriptor.Epoch,
				},
			},
			CancelUpgrade:    (*governance.CancelUpgradeProposal)(p.Content.CancelUpgrade),
			ChangeParameters: nil, // not present in cobalt
		},
		CreatedAt:    p.CreatedAt,
		ClosesAt:     p.ClosesAt,
		Results:      results,
		InvalidVotes: 0,
	}
}

func convertAccount(a *stakingCobalt.Account) *staking.Account {
	rateSteps := make([]staking.CommissionRateStep, len(a.Escrow.CommissionSchedule.Rates))
	for i, r := range a.Escrow.CommissionSchedule.Rates {
		rateSteps[i] = staking.CommissionRateStep(r)
	}
	rateBoundSteps := make([]staking.CommissionRateBoundStep, len(a.Escrow.CommissionSchedule.Bounds))
	for i, r := range a.Escrow.CommissionSchedule.Bounds {
		rateBoundSteps[i] = staking.CommissionRateBoundStep(r)
	}
	return &staking.Account{
		General: staking.GeneralAccount(a.General),
		Escrow: staking.EscrowAccount{
			Active:    staking.SharePool(a.Escrow.Active),
			Debonding: staking.SharePool(a.Escrow.Debonding),
			CommissionSchedule: staking.CommissionSchedule{
				Rates:  rateSteps,
				Bounds: rateBoundSteps,
			},
		},
	}
}

func convertRuntime(r *registryCobalt.Runtime) *registry.Runtime {
	return &registry.Runtime{
		ID:          r.ID,
		EntityID:    r.EntityID,
		Kind:        registry.RuntimeKind(r.Kind),
		KeyManager:  r.KeyManager,
		TEEHardware: r.TEEHardware,
	}
}

// ConvertGenesis converts a genesis document from the Cobalt format to the
// nexus-internal (= current oasis-core) format.
// WARNING: This is a partial conversion, only the fields that are used by
// Nexus are filled in the output document.
func ConvertGenesis(d genesisCobalt.Document) *genesis.Document {
	proposals := make([]*governance.Proposal, len(d.Governance.Proposals))
	for i, p := range d.Governance.Proposals {
		proposals[i] = convertProposal(p)
	}

	voteEntries := make(map[uint64][]*governance.VoteEntry, len(d.Governance.VoteEntries))
	for k, v := range d.Governance.VoteEntries {
		voteEntries[k] = make([]*governance.VoteEntry, len(v))
		for i, ve := range v {
			voteEntries[k][i] = &governance.VoteEntry{
				Voter: ve.Voter,
				Vote:  governance.Vote(ve.Vote),
			}
		}
	}

	ledger := make(map[staking.Address]*staking.Account, len(d.Staking.Ledger))
	for k, v := range d.Staking.Ledger {
		ledger[k] = convertAccount(v)
	}

	delegations := make(map[staking.Address]map[staking.Address]*staking.Delegation, len(d.Staking.Delegations))
	for k, v := range d.Staking.Delegations {
		delegations[k] = make(map[staking.Address]*staking.Delegation, len(v))
		for k2, v2 := range v {
			delegations[k][k2] = &staking.Delegation{
				Shares: v2.Shares,
			}
		}
	}

	debondingDelegations := make(map[staking.Address]map[staking.Address][]*staking.DebondingDelegation, len(d.Staking.DebondingDelegations))
	for k, v := range d.Staking.DebondingDelegations {
		debondingDelegations[k] = make(map[staking.Address][]*staking.DebondingDelegation, len(v))
		for k2, v2 := range v {
			debondingDelegations[k][k2] = make([]*staking.DebondingDelegation, len(v2))
			for i, v3 := range v2 {
				debondingDelegations[k][k2][i] = &staking.DebondingDelegation{
					Shares: v3.Shares,
				}
			}
		}
	}

	runtimes := make([]*registry.Runtime, len(d.Registry.Runtimes))
	for i, r := range d.Registry.Runtimes {
		runtimes[i] = convertRuntime(r)
	}

	return &genesis.Document{
		Height:  d.Height,
		Time:    d.Time,
		ChainID: d.ChainID,
		Governance: governance.Genesis{
			Proposals:   proposals,
			VoteEntries: voteEntries,
		},
		Registry: registry.Genesis{
			Entities:          d.Registry.Entities,
			Runtimes:          []*registry.Runtime{},
			SuspendedRuntimes: []*registry.Runtime{},
			Nodes:             d.Registry.Nodes,
		},
		Staking: staking.Genesis{
			CommonPool:           d.Staking.CommonPool,
			LastBlockFees:        d.Staking.LastBlockFees,
			GovernanceDeposits:   d.Staking.GovernanceDeposits,
			Ledger:               ledger,
			Delegations:          delegations,
			DebondingDelegations: debondingDelegations,
		},
	}
}

func convertEvent(e txResultsCobalt.Event) nodeapi.Event {
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
			case e.Staking.Escrow.DebondingStart != nil: // Note: Event started appearing mid-Cobalt.
				ret = nodeapi.Event{
					StakingDebondingStart: &nodeapi.DebondingStartEscrowEvent{
						Owner:           e.Staking.Escrow.DebondingStart.Owner,
						Escrow:          e.Staking.Escrow.DebondingStart.Escrow,
						Amount:          e.Staking.Escrow.DebondingStart.Amount,
						ActiveShares:    e.Staking.Escrow.DebondingStart.ActiveShares,
						DebondingShares: e.Staking.Escrow.DebondingStart.DebondingShares,
						DebondEndTime:   0, // Not provided in Cobalt; added in core v22.0, i.e. the testnet-only Damask precursor.
					},
					RawBody: common.TryAsJSON(e.Staking.Escrow.DebondingStart),
					Type:    apiTypes.ConsensusEventTypeStakingEscrowDebondingStart,
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
					NodeID: nil, // Not available in Cobalt.
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
				GovernanceProposalFinalized: &nodeapi.ProposalFinalizedEvent{
					ID: e.Governance.ProposalFinalized.ID,
					// This assumes that the ProposalState enum is backwards-compatible
					State: governance.ProposalState(e.Governance.ProposalFinalized.State),
				},
				RawBody: common.TryAsJSON(e.Governance.ProposalFinalized),
				Type:    apiTypes.ConsensusEventTypeGovernanceProposalFinalized,
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

func convertTxResult(r txResultsCobalt.Result) nodeapi.TxResult {
	events := make([]nodeapi.Event, len(r.Events))
	for i, e := range r.Events {
		events[i] = convertEvent(*e)
	}

	return nodeapi.TxResult{
		Error:  nodeapi.TxError(r.Error),
		Events: events,
	}
}

func convertBlock(block consensusCobalt.Block) *consensus.Block {
	return &consensus.Block{
		Height:    block.Height,
		Hash:      hash.NewFromBytes(block.Hash), // The only "conversion": from []byte to hash.Hash, which is defined as []byte.
		Time:      block.Time,
		StateRoot: block.StateRoot,
		Meta:      block.Meta,
	}
}

func convertCommittee(c schedulerCobalt.Committee) nodeapi.Committee {
	members := make([]*scheduler.CommitteeNode, len(c.Members))
	for j, m := range c.Members {
		members[j] = &scheduler.CommitteeNode{
			PublicKey: m.PublicKey,
			Role:      scheduler.Role(m.Role), // We assume the enum is backwards-compatible.
		}
	}
	return nodeapi.Committee{
		Kind:      nodeapi.CommitteeKind(c.Kind), // The enum is compatible between Cobalt and Damask.
		Members:   members,
		RuntimeID: c.RuntimeID,
		ValidFor:  c.ValidFor,
	}
}
