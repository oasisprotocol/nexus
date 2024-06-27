package eden

import (
	"strings"

	"github.com/oasisprotocol/oasis-core/go/common/cbor"
	"github.com/oasisprotocol/oasis-core/go/common/crypto/signature"
	"github.com/oasisprotocol/oasis-core/go/common/quantity"

	coreCommon "github.com/oasisprotocol/oasis-core/go/common"

	"github.com/oasisprotocol/nexus/coreapi/v22.2.11/common/node"
	genesis "github.com/oasisprotocol/nexus/coreapi/v22.2.11/genesis/api"
	governance "github.com/oasisprotocol/nexus/coreapi/v22.2.11/governance/api"
	registry "github.com/oasisprotocol/nexus/coreapi/v22.2.11/registry/api"
	"github.com/oasisprotocol/nexus/coreapi/v22.2.11/roothash/api/message"
	scheduler "github.com/oasisprotocol/nexus/coreapi/v22.2.11/scheduler/api"
	staking "github.com/oasisprotocol/nexus/coreapi/v22.2.11/staking/api"
	upgrade "github.com/oasisprotocol/nexus/coreapi/v22.2.11/upgrade/api"

	apiTypes "github.com/oasisprotocol/nexus/api/v1/types"
	"github.com/oasisprotocol/nexus/common"
	"github.com/oasisprotocol/nexus/storage/oasis/nodeapi"

	// data types for Eden gRPC APIs.
	nodeEden "github.com/oasisprotocol/nexus/coreapi/v24.0/common/node"
	txResultsEden "github.com/oasisprotocol/nexus/coreapi/v24.0/consensus/api/transaction/results"
	genesisEden "github.com/oasisprotocol/nexus/coreapi/v24.0/genesis/api"
	governanceEden "github.com/oasisprotocol/nexus/coreapi/v24.0/governance/api"
	registryEden "github.com/oasisprotocol/nexus/coreapi/v24.0/registry/api"
	roothashEden "github.com/oasisprotocol/nexus/coreapi/v24.0/roothash/api"
	schedulerEden "github.com/oasisprotocol/nexus/coreapi/v24.0/scheduler/api"
	stakingEden "github.com/oasisprotocol/nexus/coreapi/v24.0/staking/api"
)

func convertProposal(p *governanceEden.Proposal) *governance.Proposal {
	results := make(map[governance.Vote]quantity.Quantity)
	for k, v := range p.Results {
		results[governance.Vote(k)] = v
	}

	var up *governance.UpgradeProposal
	if p.Content.Upgrade != nil {
		up = &governance.UpgradeProposal{
			Descriptor: upgrade.Descriptor{
				Versioned: p.Content.Upgrade.Descriptor.Versioned,
				Handler:   upgrade.HandlerName(p.Content.Upgrade.Descriptor.Handler),
				Target:    p.Content.Upgrade.Descriptor.Target,
				Epoch:     p.Content.Upgrade.Descriptor.Epoch,
			},
		}
	}

	return &governance.Proposal{
		ID:        p.ID,
		Submitter: p.Submitter,
		State:     governance.ProposalState(p.State),
		Deposit:   p.Deposit,
		Content: governance.ProposalContent{
			Upgrade:          up,
			CancelUpgrade:    (*governance.CancelUpgradeProposal)(p.Content.CancelUpgrade),
			ChangeParameters: (*governance.ChangeParametersProposal)(p.Content.ChangeParameters),
		},
		CreatedAt:    p.CreatedAt,
		ClosesAt:     p.ClosesAt,
		Results:      results,
		InvalidVotes: p.InvalidVotes,
	}
}

func convertAccount(a *stakingEden.Account) *staking.Account {
	rateSteps := make([]staking.CommissionRateStep, len(a.Escrow.CommissionSchedule.Rates))
	for i, r := range a.Escrow.CommissionSchedule.Rates {
		rateSteps[i] = staking.CommissionRateStep(r)
	}
	rateBoundSteps := make([]staking.CommissionRateBoundStep, len(a.Escrow.CommissionSchedule.Bounds))
	for i, r := range a.Escrow.CommissionSchedule.Bounds {
		rateBoundSteps[i] = staking.CommissionRateBoundStep(r)
	}
	return &staking.Account{
		General: staking.GeneralAccount{
			Balance:    a.General.Balance,
			Nonce:      a.General.Nonce,
			Allowances: a.General.Allowances,
		},
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

func convertRuntime(r *registryEden.Runtime) *registry.Runtime {
	return &registry.Runtime{
		ID:          r.ID,
		EntityID:    r.EntityID,
		Kind:        registry.RuntimeKind(r.Kind),
		KeyManager:  r.KeyManager,
		TEEHardware: node.TEEHardware(r.TEEHardware), // identical enums
	}
}

func convertNodeAddresses(addrs []nodeEden.Address) []node.Address {
	ret := make([]node.Address, len(addrs))
	for i, a := range addrs {
		ret[i] = node.Address(a)
	}
	return ret
}

func convertNodeConsensusAddresses(addrs []nodeEden.ConsensusAddress) []node.ConsensusAddress {
	ret := make([]node.ConsensusAddress, len(addrs))
	for i, a := range addrs {
		ret[i] = node.ConsensusAddress{
			ID:      a.ID,
			Address: node.Address(a.Address),
		}
	}
	return ret
}

func convertNode(signedNode *nodeEden.MultiSignedNode) (*node.Node, error) {
	var n nodeEden.Node
	if err := cbor.Unmarshal(signedNode.Blob, &n); err != nil {
		return nil, err
	}

	runtimes := make([]*node.Runtime, len(n.Runtimes))
	for i, r := range n.Runtimes {
		runtimes[i] = &node.Runtime{
			ID: r.ID,
		}
	}
	return &node.Node{
		ID:         n.ID,
		EntityID:   n.EntityID,
		Expiration: n.Expiration,
		TLS: node.TLSInfo{
			PubKey: n.TLS.PubKey,
			// `NextPubKey` and `Addresses` are not present in Eden.
		},
		P2P: node.P2PInfo{
			ID:        n.P2P.ID,
			Addresses: convertNodeAddresses(n.P2P.Addresses),
		},
		Consensus: node.ConsensusInfo{
			ID:        n.Consensus.ID,
			Addresses: convertNodeConsensusAddresses(n.Consensus.Addresses),
		},
		VRF:             (*node.VRFInfo)(&n.VRF),
		Runtimes:        runtimes,
		Roles:           node.RolesMask(n.Roles), // identical enums
		SoftwareVersion: string(n.SoftwareVersion),
	}, nil
}

// ConvertGenesis converts a genesis document from the Eden format to the
// nexus-internal format.
// WARNING: This is a partial conversion, only the fields that are used by
// Nexus are filled in the output document.
func ConvertGenesis(d genesisEden.Document) (*genesis.Document, error) {
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

	nodes := make([]*node.MultiSignedNode, len(d.Registry.Nodes))
	for i, n := range d.Registry.Nodes {
		n2, err := convertNode(n)
		if err != nil {
			return nil, err
		}
		// XXX: We "sign" the converted node with 0 signatures. Then nexus code deliberately ignores the signatures.
		// This is all a silly dance so we're able to reuse the existing oasis-core types (which enforce signing) inside nexus.
		// If nexus introduces its own internal types for nodes and genesis, this can be simplified.
		nodes[i] = &node.MultiSignedNode{
			MultiSigned: signature.MultiSigned{
				Blob: cbor.Marshal(n2),
			},
		}
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
			Runtimes:          runtimes,
			SuspendedRuntimes: suspendedRuntimes,
			Nodes:             nodes,
		},
		Staking: staking.Genesis{
			CommonPool:           d.Staking.CommonPool,
			LastBlockFees:        d.Staking.LastBlockFees,
			GovernanceDeposits:   d.Staking.GovernanceDeposits,
			Ledger:               ledger,
			Delegations:          delegations,
			DebondingDelegations: debondingDelegations,
		},
	}, nil
}

func convertStakingEvent(e stakingEden.Event) nodeapi.Event {
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
					NewShares: &e.Escrow.Add.NewShares,
				},
				RawBody: common.TryAsJSON(e.Escrow.Add),
				Type:    apiTypes.ConsensusEventTypeStakingEscrowAdd,
			}
		case e.Escrow.Take != nil:
			ret = nodeapi.Event{
				StakingTakeEscrow: &nodeapi.TakeEscrowEvent{
					Owner:           e.Escrow.Take.Owner,
					Amount:          e.Escrow.Take.Amount,
					DebondingAmount: &e.Escrow.Take.DebondingAmount,
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
				StakingDebondingStart: &nodeapi.DebondingStartEscrowEvent{
					Owner:           e.Escrow.DebondingStart.Owner,
					Escrow:          e.Escrow.DebondingStart.Escrow,
					Amount:          e.Escrow.DebondingStart.Amount,
					ActiveShares:    e.Escrow.DebondingStart.ActiveShares,
					DebondingShares: e.Escrow.DebondingStart.DebondingShares,
					DebondEndTime:   e.Escrow.DebondingStart.DebondEndTime,
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

func convertRegistryEvent(e registryEden.Event) nodeapi.Event {
	ret := nodeapi.Event{}
	switch {
	case e.RuntimeStartedEvent != nil:
		ret = nodeapi.Event{
			RegistryRuntimeStarted: &nodeapi.RuntimeStartedEvent{
				ID:          e.RuntimeStartedEvent.Runtime.ID,
				EntityID:    e.RuntimeStartedEvent.Runtime.EntityID,
				Kind:        e.RuntimeStartedEvent.Runtime.Kind.String(),
				KeyManager:  e.RuntimeStartedEvent.Runtime.KeyManager,
				TEEHardware: e.RuntimeStartedEvent.Runtime.TEEHardware.String(),
			},
			RawBody: common.TryAsJSON(e.RuntimeStartedEvent),
			Type:    apiTypes.ConsensusEventTypeRegistryRuntime,
		}
	case e.RuntimeSuspendedEvent != nil:
		ret = nodeapi.Event{
			RegistryRuntimeSuspended: &nodeapi.RuntimeSuspendedEvent{
				RuntimeID: e.RuntimeSuspendedEvent.RuntimeID,
			},
			RawBody: common.TryAsJSON(e.RuntimeSuspendedEvent),
			Type:    apiTypes.ConsensusEventTypeRegistryRuntimeSuspended,
		}
	case e.EntityEvent != nil:
		ret = nodeapi.Event{
			RegistryEntity: (*nodeapi.EntityEvent)(e.EntityEvent),
			RawBody:        common.TryAsJSON(e.EntityEvent),
			Type:           apiTypes.ConsensusEventTypeRegistryEntity,
		}
	case e.NodeEvent != nil:
		var vrfID *signature.PublicKey
		vrfID = &e.NodeEvent.Node.VRF.ID
		runtimeIDs := make([]coreCommon.Namespace, len(e.NodeEvent.Node.Runtimes))
		for i, r := range e.NodeEvent.Node.Runtimes {
			runtimeIDs[i] = r.ID
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
				TLSAddresses:       []string{}, // Not used any more in Eden.
				TLSPubKey:          e.NodeEvent.Node.TLS.PubKey,
				TLSNextPubKey:      signature.PublicKey{}, // Not used any more in Eden.
				P2PID:              e.NodeEvent.Node.P2P.ID,
				P2PAddresses:       p2pAddresses,
				RuntimeIDs:         runtimeIDs,
				ConsensusID:        e.NodeEvent.Node.Consensus.ID,
				ConsensusAddresses: consensusAddresses,
				IsRegistration:     e.NodeEvent.IsRegistration,
				Roles:              strings.Split(e.NodeEvent.Node.Roles.String(), ","),
				SoftwareVersion:    string(e.NodeEvent.Node.SoftwareVersion),
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

func convertRoothashEvent(e roothashEden.Event) nodeapi.Event {
	ret := nodeapi.Event{}
	switch {
	case e.ExecutorCommitted != nil:
		messages := make([]message.Message, len(e.ExecutorCommitted.Commit.Messages))
		for i, messageEden := range e.ExecutorCommitted.Commit.Messages {
			messageCBOR := cbor.Marshal(messageEden)
			if err := cbor.Unmarshal(messageCBOR, &messages[i]); err != nil {
				logger.Error("convert event: roothash executor committed: commit message error unmarshaling",
					"event", e,
					"message_index", i,
					"err", err,
				)
			}
		}
		ret = nodeapi.Event{
			RoothashExecutorCommitted: &nodeapi.ExecutorCommittedEvent{
				RuntimeID: e.RuntimeID,
				Round:     e.ExecutorCommitted.Commit.Header.Header.Round,
				NodeID:    &e.ExecutorCommitted.Commit.NodeID,
				Messages:  messages,
			},
			RawBody: common.TryAsJSON(e.ExecutorCommitted),
			Type:    apiTypes.ConsensusEventTypeRoothashExecutorCommitted,
		}
	case e.ExecutionDiscrepancyDetected != nil:
		ret = nodeapi.Event{
			RoothashMisc: &nodeapi.RoothashEvent{
				RuntimeID: e.RuntimeID,
				Round:     e.ExecutionDiscrepancyDetected.Round,
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

func convertGovernanceEvent(e governanceEden.Event) nodeapi.Event {
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
				// The enum is compatible between Eden and Damask.
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

func convertEvent(e txResultsEden.Event) nodeapi.Event {
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

func convertTxResult(r txResultsEden.Result) nodeapi.TxResult {
	events := make([]nodeapi.Event, len(r.Events))
	for i, e := range r.Events {
		events[i] = convertEvent(*e)
	}

	return nodeapi.TxResult{
		Error:  nodeapi.TxError(r.Error),
		Events: events,
	}
}

func convertCommittee(c schedulerEden.Committee) nodeapi.Committee {
	members := make([]*scheduler.CommitteeNode, len(c.Members))
	for j, m := range c.Members {
		members[j] = &scheduler.CommitteeNode{
			PublicKey: m.PublicKey,
			Role:      scheduler.Role(m.Role), // The enum is compatible between Eden and Damask.
		}
	}
	return nodeapi.Committee{
		Kind:      nodeapi.CommitteeKind(c.Kind), // The enum is compatible between Eden and Damask.
		Members:   members,
		RuntimeID: c.RuntimeID,
		ValidFor:  c.ValidFor,
	}
}
