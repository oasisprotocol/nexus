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
	scheduler "github.com/oasisprotocol/nexus/coreapi/v22.2.11/scheduler/api"
	staking "github.com/oasisprotocol/nexus/coreapi/v22.2.11/staking/api"
	upgrade "github.com/oasisprotocol/nexus/coreapi/v22.2.11/upgrade/api"

	apiTypes "github.com/oasisprotocol/nexus/api/v1/types"
	"github.com/oasisprotocol/nexus/common"
	"github.com/oasisprotocol/nexus/storage/oasis/nodeapi"

	// data types for Eden gRPC APIs.
	nodeEden "github.com/oasisprotocol/nexus/coreapi/v23.0/common/node"
	txResultsEden "github.com/oasisprotocol/nexus/coreapi/v23.0/consensus/api/transaction/results"
	genesisEden "github.com/oasisprotocol/nexus/coreapi/v23.0/genesis/api"
	governanceEden "github.com/oasisprotocol/nexus/coreapi/v23.0/governance/api"
	registryEden "github.com/oasisprotocol/nexus/coreapi/v23.0/registry/api"
	schedulerEden "github.com/oasisprotocol/nexus/coreapi/v23.0/scheduler/api"
	stakingEden "github.com/oasisprotocol/nexus/coreapi/v23.0/staking/api"
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
		InvalidVotes: 0,
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
					Shares: v3.Shares,
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

func convertEvent(e txResultsEden.Event) nodeapi.Event {
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
					StakingAddEscrow: &nodeapi.AddEscrowEvent{
						Owner:     e.Staking.Escrow.Add.Owner,
						Escrow:    e.Staking.Escrow.Add.Escrow,
						Amount:    e.Staking.Escrow.Add.Amount,
						NewShares: &e.Staking.Escrow.Add.NewShares,
					},
					RawBody: common.TryAsJSON(e.Staking.Escrow.Add),
					Type:    apiTypes.ConsensusEventTypeStakingEscrowAdd,
				}
			case e.Staking.Escrow.Take != nil:
				ret = nodeapi.Event{
					StakingTakeEscrow: &nodeapi.TakeEscrowEvent{
						Owner:           e.Staking.Escrow.Take.Owner,
						Amount:          e.Staking.Escrow.Take.Amount,
						DebondingAmount: &e.Staking.Escrow.Take.DebondingAmount,
					},
					RawBody: common.TryAsJSON(e.Staking.Escrow.Take),
					Type:    apiTypes.ConsensusEventTypeStakingEscrowTake,
				}
			case e.Staking.Escrow.Reclaim != nil:
				ret = nodeapi.Event{
					StakingReclaimEscrow: (*nodeapi.ReclaimEscrowEvent)(e.Staking.Escrow.Reclaim),
					RawBody:              common.TryAsJSON(e.Staking.Escrow.Reclaim),
					Type:                 apiTypes.ConsensusEventTypeStakingEscrowReclaim,
				}
			case e.Staking.Escrow.DebondingStart != nil:
				ret = nodeapi.Event{
					StakingDebondingStart: &nodeapi.DebondingStartEscrowEvent{
						Owner:           e.Staking.Escrow.DebondingStart.Owner,
						Escrow:          e.Staking.Escrow.DebondingStart.Escrow,
						Amount:          e.Staking.Escrow.DebondingStart.Amount,
						ActiveShares:    e.Staking.Escrow.DebondingStart.ActiveShares,
						DebondingShares: e.Staking.Escrow.DebondingStart.DebondingShares,
						DebondEndTime:   e.Staking.Escrow.DebondingStart.DebondEndTime,
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
		case e.Registry.RuntimeStartedEvent != nil:
			ret = nodeapi.Event{
				RegistryRuntimeStarted: &nodeapi.RuntimeStartedEvent{
					ID:          e.Registry.RuntimeStartedEvent.Runtime.ID,
					EntityID:    e.Registry.RuntimeStartedEvent.Runtime.EntityID,
					Kind:        e.Registry.RuntimeStartedEvent.Runtime.Kind.String(),
					KeyManager:  e.Registry.RuntimeStartedEvent.Runtime.KeyManager,
					TEEHardware: e.Registry.RuntimeStartedEvent.Runtime.TEEHardware.String(),
				},
				RawBody: common.TryAsJSON(e.Registry.RuntimeStartedEvent),
				Type:    apiTypes.ConsensusEventTypeRegistryRuntime,
			}
		case e.Registry.RuntimeSuspendedEvent != nil:
			ret = nodeapi.Event{
				RegistryRuntimeSuspended: &nodeapi.RuntimeSuspendedEvent{
					RuntimeID: e.Registry.RuntimeSuspendedEvent.RuntimeID,
				},
				RawBody: common.TryAsJSON(e.Registry.RuntimeSuspendedEvent),
				Type:    apiTypes.ConsensusEventTypeRegistryRuntimeSuspended,
			}
		case e.Registry.EntityEvent != nil:
			ret = nodeapi.Event{
				RegistryEntity: (*nodeapi.EntityEvent)(e.Registry.EntityEvent),
				RawBody:        common.TryAsJSON(e.Registry.EntityEvent),
				Type:           apiTypes.ConsensusEventTypeRegistryEntity,
			}
		case e.Registry.NodeEvent != nil:
			var vrfID *signature.PublicKey
			vrfID = &e.Registry.NodeEvent.Node.VRF.ID
			runtimeIDs := make([]coreCommon.Namespace, len(e.Registry.NodeEvent.Node.Runtimes))
			for i, r := range e.Registry.NodeEvent.Node.Runtimes {
				runtimeIDs[i] = r.ID
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
					TLSAddresses:       []string{}, // Not used any more in Eden.
					TLSPubKey:          e.Registry.NodeEvent.Node.TLS.PubKey,
					TLSNextPubKey:      signature.PublicKey{}, // Not used any more in Eden.
					P2PID:              e.Registry.NodeEvent.Node.P2P.ID,
					P2PAddresses:       p2pAddresses,
					RuntimeIDs:         runtimeIDs,
					ConsensusID:        e.Registry.NodeEvent.Node.Consensus.ID,
					ConsensusAddresses: consensusAddresses,
					IsRegistration:     e.Registry.NodeEvent.IsRegistration,
					Roles:              strings.Split(e.Registry.NodeEvent.Node.Roles.String(), ","),
					SoftwareVersion:    string(e.Registry.NodeEvent.Node.SoftwareVersion),
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
		case e.RootHash.InMsgProcessed != nil:
			ret = nodeapi.Event{
				RawBody: common.TryAsJSON(e.RootHash.InMsgProcessed),
				Type:    apiTypes.ConsensusEventTypeRoothashInMsgProcessed,
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
					// The enum is compatible between Eden and Damask.
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
