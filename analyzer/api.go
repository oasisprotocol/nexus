package analyzer

import (
	"errors"
	"time"

	"github.com/oasisprotocol/oasis-indexer/storage"
)

var (
	// ErrOutOfRange is returned if the current block does not fall within tge
	// analyzer's analysis range.
	ErrOutOfRange = errors.New("range not found. no data source available")

	// ErrLatestBlockNotFound is returned if the analyzer has not indexed any
	// blocks yet. This indicates to begin from the start of its range.
	ErrLatestBlockNotFound = errors.New("latest block not found")
)

// Analyzer is a worker that analyzes a subset of the Oasis Network.
type Analyzer interface {
	// Start starts the analyzer.
	Start()

	// Name returns the name of the analyzer.
	Name() string
}

// ConsensusConfig specifies configuration parameters for
// for processing the consensus layer.
type ConsensusConfig struct {
	// ChainID is the chain ID for the underlying network.
	ChainID string

	// Range is the range of blocks to process.
	// If this is set, the analyzer analyzes blocks in the provided range.
	Range BlockRange

	// Interval is the interval at which to process.
	// If this is set, the analyzer runs once per interval.
	Interval time.Duration

	// Source is the storage source from which to fetch block data
	// when processing blocks in this range.
	Source storage.ConsensusSourceStorage
}

// BlockRange is a range of blocks.
type BlockRange struct {
	// From is the first block to process in this range, inclusive.
	From int64

	// To is the last block to process in this range, inclusive.
	To int64
}

// RuntimeConfig specifies configuration parameters for
// for processing the runtime layer.
type RuntimeConfig struct {
	// ChainID is the chain ID for the underlying network.
	ChainID string

	// Range is the range of rounds to process.
	// If this is set, the analyzer analyzes rounds in the provided range.
	Range RoundRange

	// Interval is the interval at which to process.
	// If this is set, the analyzer runs once per interval.
	Interval time.Duration

	// Source is the storage source from which to fetch block data
	// when processing blocks in this range.
	Source storage.RuntimeSourceStorage
}

// RoundRange is a range of blocks.
type RoundRange struct {
	// From is the first block to process in this range, inclusive.
	From uint64

	// To is the last block to process in this range, inclusive.
	To uint64
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

// Event is a an event emitted by the consensus layer.
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
	// EventRegistryRuntime is a runtime registration event.
	EventRegistryRuntime
	// EventRegistryEntity is an entity registration event.
	EventRegistryEntity
	// EventRegistryNode is a node registration event.
	EventRegistryNode
	// EventRegistryNodeUnfrozen is a node unfrozen event.
	EventRegistryNodeUnfrozen
	// EventRoothashExecutorCommitted is an executor committed event.
	EventRoothashExecutorCommitted
	// EventRoothashDiscrepancyDetected is a discrepancy detected event.
	EventRoothashDiscrepancyDetected
	// EventRoothashFinalizedEvent is a roothash finalization event.
	EventRoothashFinalized
	// EventGovernanceProposalSubmitted is a proposal submission event.
	EventGovernanceProposalSubmitted
	// EventGovernanceProposalExecuted is a proposal execution event.
	EventGovernanceProposalExecuted
	// EventGovernanceProposalFinalized is a proposal finalization event.
	EventGovernanceProposalFinalized
	// EventGovernanceVote is a proposal vote event.
	EventGovernanceVote
	// EventUnknown is an unknown event type.
	EventUnknown = 255
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
	case EventRegistryRuntime:
		return "Runtime"
	case EventRegistryEntity:
		return "Entity"
	case EventRegistryNode:
		return "Node"
	case EventRegistryNodeUnfrozen:
		return "NodeUnfrozen"
	case EventRoothashExecutorCommitted:
		return "ExecutorCommitted"
	case EventRoothashDiscrepancyDetected:
		return "DiscrepancyDetected"
	case EventRoothashFinalized:
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
	case height < 702000:
		return "mainnet_beta_2020_10_01_1601568000"
	case height < 3027601:
		return "oasis_1"
	case height < 8048956:
		return "oasis_2"
	}
	return "oasis_3"
}

// String returns the string representation of a ChainID.
func (c ChainID) String() string {
	return string(c)
}
