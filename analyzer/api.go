package analyzer

import (
	"github.com/oasislabs/oasis-indexer/storage"
)

// Analyzer is a worker that analyzes a subset of the Oasis Network.
type Analyzer interface {
	// SetRange sets configuration for the range of blocks to process
	// to the analyzer.
	SetRange(RangeConfig)

	// Start starts the analyzer.
	Start()

	// Name returns the name of the analyzer.
	Name() string
}

// RangeConfig specifies configuration parameters
// for processing a range of blocks.
type RangeConfig struct {
	// From is the first block to process in this range, inclusive.
	From int64

	// To is the last block to process in this range, inclusive.
	To int64

	// Source is the storage source from which to fetch block data
	// when processing blocks in this range.
	Source storage.SourceStorage
}

// Backend is a consensus backend.
type Backend uint

const (
	// BackendStaking is the consensus staking backend.
	BackendStaking Backend = iota
	// BackendRegistry is the consensus staking backend.
	BackendRegistry
	// BackendRoothash is the consensus staking backend.
	BackendRoothash
	// BackendGovernance is the consensus staking backend.
	BackendGovernance
	// BackendUnknown is an unknown staking backend.
	BackendUnknown = 255
)

// String returns the string representation of a Backend.
func (b *Backend) String() string {
	switch *b {
	case BackendStaking:
		return "staking"
	case BackendRegistry:
		return "registry"
	case BackendRoothash:
		return "roothash"
	case BackendGovernance:
		return "governance"
	default:
		return "unknown"
	}
}

// Event is a consensus event.
type Event uint

const (
	// EventStakingTransfer is a transfer event.
	EventStakingTransfer Event = iota
	// EventStakingBurn is a burn event.
	EventStakingBurn
	// EventStakingAddEscrow is a add escrow event.
	EventStakingAddEscrow
	// EventStakingTakeEscrow is a take escrow event.
	EventStakingTakeEscrow
	// EventStakingDebondingStart is a debonding start event.
	EventStakingDebondingStart
	// EventStakingReclaimEscrow is a reclaim escrow event.
	EventStakingReclaimEscrow
	// EventStakingAllowanceChange is an allowance change event.
	EventStakingAllowanceChange
	// EventRegistryRuntimeRegistration is a runtime registration event.
	EventRegistryRuntimeRegistration
	// EventRegistryEntityRegistration is an entity registration event.
	EventRegistryEntityRegistration
	// EventRegistryNodeRegistration is a node registration event.
	EventRegistryNodeRegistration
	// EventRegistryNodeUnfrozenEvent is a node unfrozen event.
	EventRegistryNodeUnfrozenEvent
	// EventRoothashExecutorCommittedEvent is an executor committed event.
	EventRoothashExecutorCommittedEvent
	// EventRoothashDiscrepancyDetectedEvent is a discrepancy detected event.
	EventRoothashDiscrepancyDetectedEvent
	// EventRoothashFinalizedEvent is a roothash finalization event.
	EventRoothashFinalizedEvent
	// EventGovernanceProposalSubmitted is a proposal submission event.
	EventGovernanceProposalSubmitted
	// EventGovernanceProposalExecuted is a proposal execution event.
	EventGovernanceProposalExecuted
	// EventGovernanceProposalFinalized is a proposal finalization event.
	EventGovernanceProposalFinalized
	// EventGovernanceVote is a proposal vote event.
	EventGovernanceVote
	// EventUnknown is an unknown event type.
	EventUnknown
)

// String returns the string representation of an Event.
func (e *Event) String() string {
	switch *e {
	case EventStakingTransfer:
		return "Transfer"
	case EventStakingBurn:
		return "Burn"
	case EventStakingAddEscrow:
		return "AddEscrow"
	case EventStakingTakeEscrow:
		return "TakeEscrow"
	case EventStakingDebondingStart:
		return "DebondingStart"
	case EventStakingReclaimEscrow:
		return "ReclaimEscrow"
	case EventStakingAllowanceChange:
		return "AllowanceChange"
	case EventRegistryRuntimeRegistration:
		return "RuntimeRegistration"
	case EventRegistryEntityRegistration:
		return "EntityRegistration"
	case EventRegistryNodeRegistration:
		return "NodeRegistration"
	case EventRegistryNodeUnfrozenEvent:
		return "NodeUnfrozen"
	case EventRoothashExecutorCommittedEvent:
		return "ExecutorCommitted"
	case EventRoothashDiscrepancyDetectedEvent:
		return "DiscrepancyDetected"
	case EventRoothashFinalizedEvent:
		return "Finalized"
	case EventGovernanceProposalSubmitted:
		return "ProposalSubmitted"
	case EventGovernanceProposalExecuted:
		return "ProposalExecuted"
	case EventGovernanceProposalFinalized:
		return "ProposalFinalized"
	case EventGovernanceVote:
		return "Vote"
	default:
		return "Unknown"
	}
}

// ChainID is the ID of a chain.
type ChainID string

// FromHeight returns the ChainID for the provided height.
func FromHeight(height int64) ChainID {
	switch {
	case 0 < height && height < 702000:
		return "mainnet_beta_2020_10_01_1601568000"
	case 702000 <= height && height < 3027601:
		return "oasis_1"
	case 3027601 <= height && height < 8048956:
		return "oasis_2"
	}
	return "oasis_3"
}

// String returns the string representation of a ChainID.
func (c ChainID) String() string {
	return string(c)
}
