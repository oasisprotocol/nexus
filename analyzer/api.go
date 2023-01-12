package analyzer

import (
	"errors"
	"strings"

	oasisConfig "github.com/oasisprotocol/oasis-sdk/client-sdk/go/config"

	"github.com/oasisprotocol/oasis-indexer/storage"
)

var (
	// ErrOutOfRange is returned if the current block does not fall within the
	// analyzer's analysis range.
	ErrOutOfRange = errors.New("range not found. no data source available")

	// ErrLatestBlockNotFound is returned if the analyzer has not indexed any
	// blocks yet. This indicates to begin from the start of its range.
	ErrLatestBlockNotFound = errors.New("latest block not found")

	// ErrNetworkUnknown is returned if a chain context does not correspond
	// to a known network identifier.
	ErrNetworkUnknown = errors.New("network unknown")

	// ErrRuntimeUnknown is returned if a chain context does not correspond
	// to a known runtime identifier.
	ErrRuntimeUnknown = errors.New("runtime unknown")
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
// processing the runtime layer.
type RuntimeConfig struct {
	// Range is the range of rounds to process.
	// If this is set, the analyzer analyzes rounds in the provided range.
	Range RoundRange

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
		//nolint:goconst
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
	// EventRoothashExecutionDiscrepancyDetected is a runtime discrepancy detected event.
	EventRoothashExecutionDiscrepancyDetected
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
	name, ok := EventToString[*e]
	if ok {
		return name
	}
	return "unknown"
}

var EventToString map[Event]string = map[Event]string{
	EventStakingTransfer:                      "staking.transfer",
	EventStakingBurn:                          "staking.burn",
	EventStakingAddEscrow:                     "staking.escrow.add",
	EventStakingTakeEscrow:                    "staking.escrow.take",
	EventStakingDebondingStart:                "staking.escrow.debonding_start",
	EventStakingReclaimEscrow:                 "staking.escrow.reclaim",
	EventStakingAllowanceChange:               "staking.allowance_change",
	EventRegistryRuntime:                      "registry.runtime",
	EventRegistryEntity:                       "registry.entity",
	EventRegistryNode:                         "registry.node",
	EventRegistryNodeUnfrozen:                 "registry.node_unfrozen",
	EventRoothashExecutorCommitted:            "roothash.executor_committed",
	EventRoothashExecutionDiscrepancyDetected: "roothash.execution_discrepancy_detected",
	EventRoothashFinalized:                    "roothash.finalized",
	EventGovernanceProposalSubmitted:          "governance.proposal_submitted",
	EventGovernanceProposalExecuted:           "governance.proposal_executed",
	EventGovernanceProposalFinalized:          "governance.proposal_finalized",
	EventGovernanceVote:                       "governance.vote",
}

var StringToEvent map[string]Event = map[string]Event{
	"staking.transfer":                        EventStakingTransfer,
	"staking.burn":                            EventStakingBurn,
	"staking.escrow.add":                      EventStakingAddEscrow,
	"staking.escrow.take":                     EventStakingTakeEscrow,
	"staking.escrow.debonding_start":          EventStakingDebondingStart,
	"staking.escrow.reclaim":                  EventStakingReclaimEscrow,
	"staking.allowance_change":                EventStakingAllowanceChange,
	"registry.runtime":                        EventRegistryRuntime,
	"registry.entity":                         EventRegistryEntity,
	"registry.node":                           EventRegistryNode,
	"registry.node_unfrozen":                  EventRegistryNodeUnfrozen,
	"roothash.executor_committed":             EventRoothashExecutorCommitted,
	"roothash.execution_discrepancy_detected": EventRoothashExecutionDiscrepancyDetected,
	"roothash.finalized":                      EventRoothashFinalized,
	"governance.proposal_submitted":           EventGovernanceProposalSubmitted,
	"governance.proposal_executed":            EventGovernanceProposalExecuted,
	"governance.proposal_finalized":           EventGovernanceProposalFinalized,
	"governance.vote":                         EventGovernanceVote,
}

// ChainID is the ID of a chain.
type ChainID string

// String returns the string representation of a ChainID.
func (c ChainID) String() string {
	return string(c)
}

// Network is an instance of the Oasis Network.
type Network uint8

const (
	// NetworkTestnet is the identifier for testnet.
	NetworkTestnet Network = iota
	// NetworkMainnet is the identifier for mainnet.
	NetworkMainnet
	// NetworkUnknown is the identifier for an unknown network.
	NetworkUnknown = 255
)

// FromChainContext identifies a Network using its ChainContext.
func FromChainContext(chainContext string) (Network, error) {
	var network Network
	for name, nw := range oasisConfig.DefaultNetworks.All {
		if nw.ChainContext == chainContext {
			if err := network.Set(name); err != nil {
				return NetworkUnknown, err
			}
			return network, nil
		}
	}

	return NetworkUnknown, ErrNetworkUnknown
}

// Set sets the Network to the value specified by the provided string.
func (n *Network) Set(s string) error {
	switch strings.ToLower(s) {
	case "mainnet":
		*n = NetworkMainnet
	case "testnet":
		*n = NetworkTestnet
	default:
		return ErrNetworkUnknown
	}

	return nil
}

// String returns the string representation of a network.
func (n Network) String() string {
	switch n {
	case NetworkTestnet:
		return "testnet"
	case NetworkMainnet:
		return "mainnet"
	default:
		return "unknown"
	}
}

// Runtime is an identifier for a runtime on the Oasis Network.
type Runtime uint16

const (
	// RuntimeEmerald is the identifier for the Emerald Runtime.
	RuntimeEmerald Runtime = iota
	// RuntimeCipher is the identifier for the Cipher Runtime.
	RuntimeCipher
	// RuntimeSapphire is the identifier for the Sapphire Runtime.
	RuntimeSapphire
	// RuntimeUnknown is the identifier for an unknown Runtime.
	RuntimeUnknown = 1000
)

// String returns the string representation of a runtime.
func (r Runtime) String() string {
	switch r {
	case RuntimeEmerald:
		return "emerald"
	case RuntimeCipher:
		return "cipher"
	case RuntimeSapphire:
		return "sapphire"
	default:
		return "unknown"
	}
}

// ID returns the ID for a Runtime on the provided network.
func (r Runtime) ID(n Network) (string, error) {
	for nname, nw := range oasisConfig.DefaultNetworks.All {
		if nname == n.String() {
			for pname, pt := range nw.ParaTimes.All {
				if pname == r.String() {
					return pt.ID, nil
				}
			}

			return "", ErrRuntimeUnknown
		}
	}

	return "", ErrRuntimeUnknown
}
