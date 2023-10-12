// Package api implements the governance APIs.
package api

import (
	beacon "github.com/oasisprotocol/nexus/coreapi/v23.0/beacon/api"
	"github.com/oasisprotocol/nexus/coreapi/v23.0/consensus/api/transaction"
	staking "github.com/oasisprotocol/nexus/coreapi/v23.0/staking/api"
	upgrade "github.com/oasisprotocol/nexus/coreapi/v23.0/upgrade/api"
	"github.com/oasisprotocol/oasis-core/go/common/cbor"
	"github.com/oasisprotocol/oasis-core/go/common/crypto/hash"
	"github.com/oasisprotocol/oasis-core/go/common/quantity"
)

// ModuleName is a unique module name for the governance backend.
const ModuleName = "governance"

// ProposalContentInvalidText is the textual representation of an invalid
// ProposalContent.
const ProposalContentInvalidText = "(invalid)"

// removed var block

// ProposalContent is a consensus layer governance proposal content.
type ProposalContent struct {
	Upgrade          *UpgradeProposal          `json:"upgrade,omitempty"`
	CancelUpgrade    *CancelUpgradeProposal    `json:"cancel_upgrade,omitempty"`
	ChangeParameters *ChangeParametersProposal `json:"change_parameters,omitempty"`
}

// ValidateBasic performs basic proposal content validity checks.
// removed func

// Equals checks if proposal contents are equal.
//
// Note: this assumes valid proposals where each proposals will have
// exactly one field set.
// removed func

// PrettyPrint writes a pretty-printed representation of ProposalContent to the
// given writer.
// removed func

// PrettyType returns a representation of ProposalContent that can be used for
// pretty printing.
// removed func

// UpgradeProposal is an upgrade proposal.
type UpgradeProposal struct {
	upgrade.Descriptor
}

// Equals checks if upgrade proposals are equal.
// removed func

// PrettyPrint writes a pretty-printed representation of UpgradeProposal to the
// given writer.
// removed func

// PrettyType returns a representation of UpgradeProposal that can be used for
// pretty printing.
// removed func

// CancelUpgradeProposal is an upgrade cancellation proposal.
type CancelUpgradeProposal struct {
	// ProposalID is the identifier of the pending upgrade proposal.
	ProposalID uint64 `json:"proposal_id"`
}

// Equals checks if cancel upgrade proposals are equal.
// removed func

// PrettyPrint writes a pretty-printed representation of CancelUpgradeProposal
// to the given writer.
// removed func

// PrettyType returns a representation of CancelUpgradeProposal that can be used
// for pretty printing.
// removed func

// ChangeParametersProposal is a consensus change parameters proposal.
type ChangeParametersProposal struct {
	// Module identifies the consensus backend module to which changes should be applied.
	Module string `json:"module"`
	// Changes are consensus parameter changes that should be applied to the module.
	Changes cbor.RawMessage `json:"changes"`
}

// Equals checks if change parameters proposals are equal.
// removed func

// PrettyPrint writes a pretty-printed representation of ChangeParametersProposal to the given
// writer.
// removed func

// PrettyType returns a representation of ChangeParametersProposal that can be used for pretty
// printing.
// removed func

// ValidateBasic performs a basic validation on the change parameters proposal.
// removed func

// ProposalVote is a vote for a proposal.
type ProposalVote struct {
	// ID is the unique identifier of a proposal.
	ID uint64 `json:"id"`
	// Vote is the vote.
	Vote Vote `json:"vote"`
}

// PrettyPrint writes a pretty-printed representation of ProposalVote to the
// given writer.
// removed func

// PrettyType returns a representation of ProposalVote that can be used for
// pretty printing.
// removed func

// Backend is a governance implementation.
// removed interface

// ProposalQuery is a proposal query.
type ProposalQuery struct {
	Height     int64  `json:"height"`
	ProposalID uint64 `json:"id"`
}

// VoteEntry contains data about a cast vote.
type VoteEntry struct {
	Voter staking.Address `json:"voter"`
	Vote  Vote            `json:"vote"`
}

// Genesis is the initial governance state for use in the genesis block.
//
// Note: PendingProposalUpgrades are not included in genesis, but are instead
// computed at InitChain from accepted proposals.
type Genesis struct {
	// Parameters are the genesis consensus parameters.
	Parameters ConsensusParameters `json:"params"`

	// Proposals are the governance proposals.
	Proposals []*Proposal `json:"proposals,omitempty"`

	// VoteEntries are the governance proposal vote entries.
	VoteEntries map[uint64][]*VoteEntry `json:"vote_entries,omitempty"`
}

// ConsensusParameters are the governance consensus parameters.
type ConsensusParameters struct {
	// GasCosts are the governance transaction gas costs.
	GasCosts transaction.Costs `json:"gas_costs,omitempty"`

	// MinProposalDeposit is the number of base units that are deposited when
	// creating a new proposal.
	MinProposalDeposit quantity.Quantity `json:"min_proposal_deposit,omitempty"`

	// VotingPeriod is the number of epochs after which the voting for a proposal
	// is closed and the votes are tallied.
	VotingPeriod beacon.EpochTime `json:"voting_period,omitempty"`

	// StakeThreshold is the minimum percentage of VoteYes votes in terms
	// of total voting power when the proposal expires in order for a
	// proposal to be accepted.  This value has a lower bound of 67.
	StakeThreshold uint8 `json:"stake_threshold,omitempty"`

	// UpgradeMinEpochDiff is the minimum number of epochs between the current
	// epoch and the proposed upgrade epoch for the upgrade proposal to be valid.
	// This is also the minimum number of epochs between two pending upgrades.
	UpgradeMinEpochDiff beacon.EpochTime `json:"upgrade_min_epoch_diff,omitempty"`

	// UpgradeCancelMinEpochDiff is the minimum number of epochs between the current
	// epoch and the proposed upgrade epoch for the upgrade cancellation proposal to be valid.
	UpgradeCancelMinEpochDiff beacon.EpochTime `json:"upgrade_cancel_min_epoch_diff,omitempty"`

	// EnableChangeParametersProposal is true iff change parameters proposals are allowed.
	EnableChangeParametersProposal bool `json:"enable_change_parameters_proposal,omitempty"`
}

// ConsensusParameterChanges are allowed governance consensus parameter changes.
type ConsensusParameterChanges struct {
	// GasCosts are the new gas costs.
	GasCosts transaction.Costs `json:"gas_costs,omitempty"`

	// MinProposalDeposit is the new minimal proposal deposit.
	MinProposalDeposit *quantity.Quantity `json:"min_proposal_deposit,omitempty"`

	// VotingPeriod is the new voting period.
	VotingPeriod *beacon.EpochTime `json:"voting_period,omitempty"`

	// StakeThreshold is the new stake threshold.
	StakeThreshold *uint8 `json:"stake_threshold,omitempty"`

	// UpgradeMinEpochDiff is the new minimal epoch difference between two pending upgrades.
	UpgradeMinEpochDiff *beacon.EpochTime `json:"upgrade_min_epoch_diff,omitempty"`

	// UpgradeCancelMinEpochDiff is the new minimal epoch difference for the upgrade cancellation
	// proposal to be valid.
	UpgradeCancelMinEpochDiff *beacon.EpochTime `json:"upgrade_cancel_min_epoch_diff,omitempty"`

	// EnableChangeParametersProposal is the new enable change parameters proposal flag.
	EnableChangeParametersProposal *bool `json:"enable_change_parameters_proposal,omitempty"`
}

// Apply applies changes to the given consensus parameters.
// removed func

// Event signifies a governance event, returned via GetEvents.
type Event struct {
	Height int64     `json:"height,omitempty"`
	TxHash hash.Hash `json:"tx_hash,omitempty"`

	ProposalSubmitted *ProposalSubmittedEvent `json:"proposal_submitted,omitempty"`
	ProposalExecuted  *ProposalExecutedEvent  `json:"proposal_executed,omitempty"`
	ProposalFinalized *ProposalFinalizedEvent `json:"proposal_finalized,omitempty"`
	Vote              *VoteEvent              `json:"vote,omitempty"`
}

// ProposalSubmittedEvent is the event emitted when a new proposal is submitted.
type ProposalSubmittedEvent struct {
	// ID is the unique identifier of a proposal.
	ID uint64 `json:"id"`
	// Submitter is the staking account address of the submitter.
	Submitter staking.Address `json:"submitter"`
}

// EventKind returns a string representation of this event's kind.
// removed func

// ProposalExecutedEvent is emitted when a proposal is executed.
type ProposalExecutedEvent struct {
	// ID is the unique identifier of a proposal.
	ID uint64 `json:"id"`
}

// EventKind returns a string representation of this event's kind.
// removed func

// ProposalFinalizedEvent is the event emitted when a proposal is finalized.
type ProposalFinalizedEvent struct {
	// ID is the unique identifier of a proposal.
	ID uint64 `json:"id"`
	// State is the new proposal state.
	State ProposalState `json:"state"`
}

// EventKind returns a string representation of this event's kind.
// removed func

// VoteEvent is the event emitted when a vote is cast.
type VoteEvent struct {
	// ID is the unique identifier of a proposal.
	ID uint64 `json:"id"`
	// Submitter is the staking account address of the vote submitter.
	Submitter staking.Address `json:"submitter"`
	// Vote is the cast vote.
	Vote Vote `json:"vote"`
}

// EventKind returns a string representation of this event's kind.
// removed func

// NewSubmitProposalTx creates a new submit proposal transaction.
// removed func

// NewCastVoteTx creates a new cast vote transaction.
// removed func

const (
	// GasOpSubmitProposal is the gas operation identifier for submitting proposal.
	GasOpSubmitProposal transaction.Op = "submit_proposal"
	// GasOpCastVote is the gas operation identifier for casting vote.
	GasOpCastVote transaction.Op = "cast_vote"
)

// DefaultGasCosts are the "default" gas costs for operations.
// removed var statement
