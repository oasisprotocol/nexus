package cobalt

import (
	"github.com/oasisprotocol/oasis-core/go/common/quantity"
	genesis "github.com/oasisprotocol/oasis-core/go/genesis/api"
	governance "github.com/oasisprotocol/oasis-core/go/governance/api"
	registry "github.com/oasisprotocol/oasis-core/go/registry/api"
	staking "github.com/oasisprotocol/oasis-core/go/staking/api"
	genesisCobalt "github.com/oasisprotocol/oasis-indexer/coreapi/v21.1.1/genesis/api"
	governanceCobalt "github.com/oasisprotocol/oasis-indexer/coreapi/v21.1.1/governance/api"
	registryCobalt "github.com/oasisprotocol/oasis-indexer/coreapi/v21.1.1/registry/api"
	stakingCobalt "github.com/oasisprotocol/oasis-indexer/coreapi/v21.1.1/staking/api"
)

func convertProposal(p *governanceCobalt.Proposal) *governance.Proposal {
	results := make(map[governance.Vote]quantity.Quantity)
	for k, v := range p.Results {
		results[governance.Vote(k)] = v
	}

	return &governance.Proposal{
		ID:        p.ID,
		Submitter: staking.Address(p.Submitter),
		State:     governance.ProposalState(p.State),
		Deposit:   p.Deposit,
		Content: governance.ProposalContent{
			Upgrade:          (*governance.UpgradeProposal)(p.Content.Upgrade),
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
// indexer-internal (= current oasis-core) format.
// WARNING: This is a partial conversion, only the fields that are used by
// the indexer are filled in the output document.
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
				Voter: staking.Address(ve.Voter),
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
