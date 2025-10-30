package cobalt

import (
	"strings"

	"github.com/oasisprotocol/oasis-core/go/common/cbor"
	"github.com/oasisprotocol/oasis-core/go/common/crypto/hash"
	"github.com/oasisprotocol/oasis-core/go/common/crypto/signature"
	"github.com/oasisprotocol/oasis-core/go/common/quantity"

	// nexus-internal data types.

	registry "github.com/oasisprotocol/nexus/coreapi/v22.2.11/registry/api"
	"github.com/oasisprotocol/nexus/coreapi/v22.2.11/roothash/api/message"
	scheduler "github.com/oasisprotocol/nexus/coreapi/v22.2.11/scheduler/api"
	staking "github.com/oasisprotocol/nexus/coreapi/v22.2.11/staking/api"
	consensus "github.com/oasisprotocol/nexus/coreapi/v24.0/consensus/api"
	governance "github.com/oasisprotocol/nexus/coreapi/v24.0/governance/api"
	upgrade "github.com/oasisprotocol/nexus/coreapi/v24.0/upgrade/api"

	apiTypes "github.com/oasisprotocol/nexus/api/v1/types"
	"github.com/oasisprotocol/nexus/common"
	"github.com/oasisprotocol/nexus/storage/oasis/nodeapi"

	// data types for Cobalt gRPC APIs.
	consensusCobalt "github.com/oasisprotocol/nexus/coreapi/v21.1.1/consensus/api"
	txResultsCobalt "github.com/oasisprotocol/nexus/coreapi/v21.1.1/consensus/api/transaction/results"
	genesisCobalt "github.com/oasisprotocol/nexus/coreapi/v21.1.1/genesis/api"
	governanceCobalt "github.com/oasisprotocol/nexus/coreapi/v21.1.1/governance/api"
	registryCobalt "github.com/oasisprotocol/nexus/coreapi/v21.1.1/registry/api"
	roothashCobalt "github.com/oasisprotocol/nexus/coreapi/v21.1.1/roothash/api"
	commitmentCobalt "github.com/oasisprotocol/nexus/coreapi/v21.1.1/roothash/api/commitment"
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
			Metadata: nil, // Not present in Damask.
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
		InvalidVotes: p.InvalidVotes,
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
func ConvertGenesis(d genesisCobalt.Document) *nodeapi.GenesisDocument {
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
					Shares:        v3.Shares,
					DebondEndTime: v3.DebondEndTime,
				}
			}
		}
	}

	runtimes := make([]*registry.Runtime, len(d.Registry.Runtimes))
	for i, r := range d.Registry.Runtimes {
		runtimes[i] = convertRuntime(r)
	}

	suspendedRuntimes := make([]*registry.Runtime, len(d.Registry.SuspendedRuntimes))
	for i, r := range d.Registry.SuspendedRuntimes {
		suspendedRuntimes[i] = convertRuntime(r)
	}

	return &nodeapi.GenesisDocument{
		Height:    d.Height,
		Time:      d.Time,
		ChainID:   d.ChainID,
		BaseEpoch: uint64(d.Beacon.Base),
		Governance: governance.Genesis{
			Proposals:   proposals,
			VoteEntries: voteEntries,
		},
		Registry: registry.Genesis{
			Entities:          d.Registry.Entities,
			Runtimes:          runtimes,
			SuspendedRuntimes: suspendedRuntimes,
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

func convertStakingEvent(e stakingCobalt.Event) nodeapi.Event {
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
				StakingAddEscrow: &nodeapi.AddEscrowEvent{
					Owner:     e.Escrow.Add.Owner,
					Escrow:    e.Escrow.Add.Escrow,
					Amount:    e.Escrow.Add.Amount,
					NewShares: nil, // Only present in a part of Cobalt. Pretend it never exists in Cobalt because we cannot distinguish between 0 and not-present.
				},
				RawBody: common.TryAsJSON(e.Escrow.Add),
				Type:    apiTypes.ConsensusEventTypeStakingEscrowAdd,
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
		case e.Escrow.DebondingStart != nil: // Note: Event started appearing mid-Cobalt.
			ret = nodeapi.Event{
				StakingDebondingStart: &nodeapi.DebondingStartEscrowEvent{
					Owner:           e.Escrow.DebondingStart.Owner,
					Escrow:          e.Escrow.DebondingStart.Escrow,
					Amount:          e.Escrow.DebondingStart.Amount,
					ActiveShares:    e.Escrow.DebondingStart.ActiveShares,
					DebondingShares: e.Escrow.DebondingStart.DebondingShares,
					DebondEndTime:   0, // Not provided in Cobalt; added in core v22.0, i.e. the testnet-only Damask precursor.
				},
				RawBody: common.TryAsJSON(e.Escrow.DebondingStart),
				Type:    apiTypes.ConsensusEventTypeStakingEscrowDebondingStart,
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

func convertRegistryEvent(e registryCobalt.Event) nodeapi.Event {
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
		tlsAddresses := make([]string, len(e.NodeEvent.Node.TLS.Addresses))
		for i, a := range e.NodeEvent.Node.TLS.Addresses {
			tlsAddresses[i] = a.String()
		}
		p2pAddresses := make([]string, len(e.NodeEvent.Node.P2P.Addresses))
		for i, a := range e.NodeEvent.Node.P2P.Addresses {
			p2pAddresses[i] = a.String()
		}
		runtimes := make([]*nodeapi.NodeRuntime, len(e.NodeEvent.Node.Runtimes))
		for i, r := range e.NodeEvent.Node.Runtimes {
			capabilities := cbor.Marshal(r.Capabilities)
			runtimes[i] = &nodeapi.NodeRuntime{
				ID:              r.ID,
				Version:         r.Version.String(),
				RawCapabilities: capabilities,
				ExtraInfo:       r.ExtraInfo,
			}
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
				Runtimes:           runtimes,
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

func convertRoothashEvent(e roothashCobalt.Event) nodeapi.Event {
	ret := nodeapi.Event{}
	switch {
	case e.ExecutorCommitted != nil:
		var computeBody commitmentCobalt.ComputeBody
		if err := cbor.Unmarshal(e.ExecutorCommitted.Commit.Blob, &computeBody); err != nil {
			logger.Error("convert event: roothash executor committed: commit error unmarshaling",
				"event", e,
				"err", err,
			)
		}
		messages := make([]message.Message, len(computeBody.Messages))
		for i, messageCobalt := range computeBody.Messages {
			messageCBOR := cbor.Marshal(messageCobalt)
			if err := cbor.Unmarshal(messageCBOR, &messages[i]); err != nil {
				logger.Error("convert event: roothash executor committed: compute body message error unmarshaling",
					"event", e,
					"message_index", i,
					"err", err,
				)
			}
		}
		ret = nodeapi.Event{
			RoothashExecutorCommitted: &nodeapi.ExecutorCommittedEvent{
				RuntimeID: e.RuntimeID,
				Round:     computeBody.Header.Round,
				NodeID:    &e.ExecutorCommitted.Commit.Signature.PublicKey,
				Messages:  messages,
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
	case e.Message != nil:
		ret = nodeapi.Event{
			RoothashMessage: &nodeapi.MessageEvent{
				RuntimeID: e.RuntimeID,
				Module:    e.Message.Module,
				Code:      e.Message.Code,
				Index:     e.Message.Index,
			},
			RawBody: common.TryAsJSON(e.Message),
			Type:    apiTypes.ConsensusEventTypeRoothashMessage,
		}
	}
	ret.Height = e.Height
	ret.TxHash = e.TxHash
	return ret
}

func convertGovernanceEvent(e governanceCobalt.Event) nodeapi.Event {
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
			GovernanceProposalFinalized: &nodeapi.ProposalFinalizedEvent{
				ID: e.ProposalFinalized.ID,
				// This assumes that the ProposalState enum is backwards-compatible
				State: governance.ProposalState(e.ProposalFinalized.State),
			},
			RawBody: common.TryAsJSON(e.ProposalFinalized),
			Type:    apiTypes.ConsensusEventTypeGovernanceProposalFinalized,
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

func convertEvent(e txResultsCobalt.Event) nodeapi.Event {
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

func convertTxResult(r txResultsCobalt.Result) nodeapi.TxResult {
	events := make([]nodeapi.Event, len(r.Events))
	for i, e := range r.Events {
		events[i] = convertEvent(*e)
	}

	return nodeapi.TxResult{
		Error:   nodeapi.TxError(r.Error),
		Events:  events,
		GasUsed: 0, // Not present in Cobalt.,
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
