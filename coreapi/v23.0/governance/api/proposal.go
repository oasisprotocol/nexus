package api

import (
	"fmt"

	beacon "github.com/oasisprotocol/nexus/coreapi/v23.0/beacon/api"
	staking "github.com/oasisprotocol/nexus/coreapi/v23.0/staking/api"
	"github.com/oasisprotocol/oasis-core/go/common/quantity"
)

// ProposalState is the state of the proposal.
type ProposalState uint8

// Proposal state kinds.
const (
	StateActive   ProposalState = 1
	StatePassed   ProposalState = 2
	StateRejected ProposalState = 3
	StateFailed   ProposalState = 4

	StateActiveName   = "active"
	StatePassedName   = "passed"
	StateRejectedName = "rejected"
	StateFailedName   = "failed"
)

// String returns a string representation of a ProposalState.
func (p ProposalState) String() string {
	switch p {
	case StateActive:
		return StateActiveName
	case StatePassed:
		return StatePassedName
	case StateRejected:
		return StateRejectedName
	case StateFailed:
		return StateFailedName
	default:
		return fmt.Sprintf("[unknown state: %d]", p)
	}
}

// MarshalText encodes a ProposalState into text form.
func (p ProposalState) MarshalText() ([]byte, error) {
	switch p {
	case StateActive:
		return []byte(StateActiveName), nil
	case StatePassed:
		return []byte(StatePassedName), nil
	case StateRejected:
		return []byte(StateRejectedName), nil
	case StateFailed:
		return []byte(StateFailedName), nil
	default:
		return nil, fmt.Errorf("invalid state: %d", p)
	}
}

// UnmarshalText decodes a text slice into a ProposalState.
func (p *ProposalState) UnmarshalText(text []byte) error {
	switch string(text) {
	case StateActiveName:
		*p = StateActive
	case StatePassedName:
		*p = StatePassed
	case StateRejectedName:
		*p = StateRejected
	case StateFailedName:
		*p = StateFailed
	default:
		return fmt.Errorf("invalid state: %s", string(text))
	}
	return nil
}

// removed var statement

// Proposal is a consensus upgrade proposal.
type Proposal struct {
	// ID is the unique identifier of the proposal.
	ID uint64 `json:"id"`
	// Submitter is the address of the proposal submitter.
	Submitter staking.Address `json:"submitter"`
	// State is the state of the proposal.
	State ProposalState `json:"state"`
	// Deposit is the deposit attached to the proposal.
	Deposit quantity.Quantity `json:"deposit"`

	// Content is the content of the proposal.
	Content ProposalContent `json:"content"`

	// CreatedAt is the epoch at which the proposal was created.
	CreatedAt beacon.EpochTime `json:"created_at"`
	// ClosesAt is the epoch at which the proposal will close and votes will
	// be tallied.
	ClosesAt beacon.EpochTime `json:"closes_at"`
	// Results are the final tallied results after the voting period has
	// ended.
	Results map[Vote]quantity.Quantity `json:"results,omitempty"`
	// InvalidVotes is the number of invalid votes after tallying.
	InvalidVotes uint64 `json:"invalid_votes,omitempty"`
}

// VotedSum returns the sum of all votes.
// removed func

// CloseProposal closes an active proposal based on the vote results and
// specified voting parameters.
//
// The proposal is accepted iff the percentage of yes votes relative to
// total voting power is at least `stakeThreshold`.  Otherwise the proposal
// is rejected.
// removed func

// Vote is a governance vote.
type Vote uint8

// Vote kinds.
const (
	VoteYes     Vote = 1
	VoteNo      Vote = 2
	VoteAbstain Vote = 3

	VoteYesName     = "yes"
	VoteNoName      = "no"
	VoteAbstainName = "abstain"
)

// String returns a string representation of a Vote.
func (v Vote) String() string {
	switch v {
	case VoteYes:
		return VoteYesName
	case VoteNo:
		return VoteNoName
	case VoteAbstain:
		return VoteAbstainName
	default:
		return fmt.Sprintf("[unknown vote: %d]", v)
	}
}

// MarshalText encodes a Vote into text form.
func (v Vote) MarshalText() ([]byte, error) {
	switch v {
	case VoteYes:
		return []byte(VoteYesName), nil
	case VoteNo:
		return []byte(VoteNoName), nil
	case VoteAbstain:
		return []byte(VoteAbstainName), nil
	default:
		return nil, fmt.Errorf("invalid vote: %d", v)
	}
}

// UnmarshalText decodes a text slice into a Vote.
func (v *Vote) UnmarshalText(text []byte) error {
	switch string(text) {
	case VoteYesName:
		*v = VoteYes
	case VoteNoName:
		*v = VoteNo
	case VoteAbstainName:
		*v = VoteAbstain
	default:
		return fmt.Errorf("invalid vote: %s", string(text))
	}
	return nil
}

// PendingUpgradesFromProposals computes pending upgrades proposals state.
//
// Returns pending upgrades and corresponding proposal IDs.
// This is useful for initialzing genesis state which doesn't include pending upgrades,
// as these can always be computed from accepted proposals.
// removed func
