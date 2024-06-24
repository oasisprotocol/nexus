// Package api implements the root hash backend API and common datastructures.
package api

import (
	"math"

	"github.com/oasisprotocol/nexus/coreapi/v24.0/consensus/api/transaction"
	registry "github.com/oasisprotocol/nexus/coreapi/v24.0/registry/api"
	"github.com/oasisprotocol/nexus/coreapi/v24.0/roothash/api/block"
	"github.com/oasisprotocol/nexus/coreapi/v24.0/roothash/api/commitment"
	scheduler "github.com/oasisprotocol/nexus/coreapi/v24.0/scheduler/api"
	staking "github.com/oasisprotocol/nexus/coreapi/v24.0/staking/api"
	"github.com/oasisprotocol/oasis-core/go/common"
	"github.com/oasisprotocol/oasis-core/go/common/cbor"
	"github.com/oasisprotocol/oasis-core/go/common/crypto/hash"
	"github.com/oasisprotocol/oasis-core/go/common/quantity"
)

const (
	// ModuleName is a unique module name for the roothash module.
	ModuleName = "roothash"

	// RoundInvalid is a special round number that refers to an invalid round.
	RoundInvalid uint64 = math.MaxUint64
	// TimeoutNever is the timeout value that never expires.
	TimeoutNever int64 = 0

	// LogEventExecutionDiscrepancyDetected is a log event value that signals
	// an execution discrepancy has been detected.
	LogEventExecutionDiscrepancyDetected = "roothash/execution_discrepancy_detected"
	// LogEventTimerFired is a log event value that signals a timer has fired.
	LogEventTimerFired = "roothash/timer_fired"
	// LogEventRoundFailed is a log event value that signals a round has failed.
	LogEventRoundFailed = "roothash/round_failed"
	// LogEventMessageUnsat is a log event value that signals a roothash message was not satisfactory.
	LogEventMessageUnsat = "roothash/message_unsat"
	// LogEventHistoryReindexing is a log event value that signals a roothash runtime reindexing
	// was run.
	LogEventHistoryReindexing = "roothash/history_reindexing"
)

// removed var block

// Backend is a root hash implementation.
// removed interface

// RuntimeRequest is a generic roothash get request for a specific runtime.
type RuntimeRequest struct {
	RuntimeID common.Namespace `json:"runtime_id"`
	Height    int64            `json:"height"`
}

// RoundRootsRequest is a request for a specific runtime and round's state and I/O roots.
type RoundRootsRequest struct {
	RuntimeID common.Namespace `json:"runtime_id"`
	Height    int64            `json:"height"`
	Round     uint64           `json:"round"`
}

// InMessageQueueRequest is a request for queued incoming messages.
type InMessageQueueRequest struct {
	RuntimeID common.Namespace `json:"runtime_id"`
	Height    int64            `json:"height"`

	Offset uint64 `json:"offset,omitempty"`
	Limit  uint32 `json:"limit,omitempty"`
}

// ExecutorCommit is the argument set for the ExecutorCommit method.
type ExecutorCommit struct {
	ID      common.Namespace                `json:"id"`
	Commits []commitment.ExecutorCommitment `json:"commits"`
}

// NewExecutorCommitTx creates a new executor commit transaction.
// removed func

// SubmitMsg is the argument set for the SubmitMsg method.
type SubmitMsg struct {
	// ID is the destination runtime ID.
	ID common.Namespace `json:"id"`
	// Tag is an optional tag provided by the caller which is ignored and can be used to match
	// processed incoming message events later.
	Tag uint64 `json:"tag,omitempty"`
	// Fee is the fee sent into the runtime as part of the message being sent. The fee is
	// transferred before the message is processed by the runtime.
	Fee quantity.Quantity `json:"fee,omitempty"`
	// Tokens are any tokens sent into the runtime as part of the message being sent. The tokens are
	// transferred before the message is processed by the runtime.
	Tokens quantity.Quantity `json:"tokens,omitempty"`
	// Data is arbitrary runtime-dependent data.
	Data []byte `json:"data,omitempty"`
}

// NewSubmitMsgTx creates a new incoming runtime message submission transaction.
// removed func

// EvidenceKind is the evidence kind.
type EvidenceKind uint8

const (
	// EvidenceKindEquivocation is the evidence kind for equivocation.
	EvidenceKindEquivocation = 1
)

// Evidence is an evidence of node misbehaviour.
type Evidence struct {
	ID common.Namespace `json:"id"`

	EquivocationExecutor *EquivocationExecutorEvidence `json:"equivocation_executor,omitempty"`
	EquivocationProposal *EquivocationProposalEvidence `json:"equivocation_prop,omitempty"`
}

// Hash computes the evidence hash.
//
// Hash is derived by hashing the evidence kind and the public key of the signer.
// Assumes evidence has been validated.
// removed func

// ValidateBasic performs basic evidence validity checks.
// removed func

// EquivocationExecutorEvidence is evidence of executor commitment equivocation.
type EquivocationExecutorEvidence struct {
	CommitA commitment.ExecutorCommitment `json:"commit_a"`
	CommitB commitment.ExecutorCommitment `json:"commit_b"`
}

// ValidateBasic performs stateless executor evidence validation checks.
//
// Particularly evidence is not verified to not be expired as this requires stateful checks.
// removed func

// EquivocationProposalEvidence is evidence of executor proposed batch equivocation.
type EquivocationProposalEvidence struct {
	ProposalA commitment.Proposal `json:"prop_a"`
	ProposalB commitment.Proposal `json:"prop_b"`
}

// ValidateBasic performs stateless batch evidence validation checks.
//
// Particularly evidence is not verified to not be expired as this requires stateful checks.
// removed func

// NewEvidenceTx creates a new evidence transaction.
// removed func

// RuntimeState is the per-runtime state.
type RuntimeState struct {
	// Runtime is the latest per-epoch runtime descriptor.
	Runtime *registry.Runtime `json:"runtime"`
	// Suspended is a flag indicating whether the runtime is currently suspended.
	Suspended bool `json:"suspended,omitempty"`

	// GenesisBlock is the runtime's first block.
	GenesisBlock *block.Block `json:"genesis_block"`

	// LastBlock is the runtime's most recently generated block.
	LastBlock *block.Block `json:"last_block"`
	// LastBlockHeight is the height at which the runtime's most recent block was generated.
	LastBlockHeight int64 `json:"last_block_height"`

	// LastNormalRound is the runtime round which was normally processed by the runtime. This is
	// also the round that contains the message results for the last processed runtime messages.
	LastNormalRound uint64 `json:"last_normal_round"`
	// LastNormalHeight is the consensus block height corresponding to LastNormalRound.
	LastNormalHeight int64 `json:"last_normal_height"`

	// Committee is the committee the executor pool is collecting commitments for.
	Committee *scheduler.Committee `json:"committee,omitempty"`
	// CommitmentPool collects the executor commitments.
	CommitmentPool *commitment.Pool `json:"commitment_pool,omitempty"`
	// NextTimeout is the time at which the round is scheduled for forced finalization.
	NextTimeout int64 `json:"timeout,omitempty"`

	// LivenessStatistics contains the liveness statistics for the current epoch.
	LivenessStatistics *LivenessStatistics `json:"liveness_stats,omitempty"`
}

// AnnotatedBlock is an annotated roothash block.
type AnnotatedBlock struct {
	// Height is the underlying roothash backend's block height that
	// generated this block.
	Height int64 `json:"consensus_height"`

	// Block is the roothash block.
	Block *block.Block `json:"block"`
}

// ExecutorCommittedEvent is an event emitted each time an executor node commits.
type ExecutorCommittedEvent struct {
	// Commit is the executor commitment.
	Commit commitment.ExecutorCommitment `json:"commit"`
}

// EventKind returns a string representation of this event's kind.
// removed func

// ExecutionDiscrepancyDetectedEvent is an execute discrepancy detected event.
type ExecutionDiscrepancyDetectedEvent struct {
	// Round is the round in which the discrepancy was detected.
	Round *uint64 `json:"round"`
	// Rank is the rank of the transaction scheduler.
	Rank uint64 `json:"rank"`
	// Timeout signals whether the discrepancy was due to a timeout.
	Timeout bool `json:"timeout"`
}

// EventKind returns a string representation of this event's kind.
// removed func

// removed var statement

// RuntimeIDAttribute is the event attribute for specifying runtime ID.
// ID is base64 encoded runtime ID.
type RuntimeIDAttribute struct {
	ID common.Namespace
}

// EventKind returns a string representation of this event's kind.
// removed func

// EventValue returns a string representation of this event's kind.
// removed func

// DecodeValue decodes the attribute event value.
// removed func

// FinalizedEvent is a finalized event.
type FinalizedEvent struct {
	// Round is the round that was finalized.
	Round uint64 `json:"round"`
}

// EventKind returns a string representation of this event's kind.
// removed func

// InMsgProcessedEvent is an event of a specific incoming message being processed.
//
// In order to see details one needs to query the runtime at the specified round.
type InMsgProcessedEvent struct {
	// ID is the unique incoming message identifier.
	ID uint64 `json:"id"`
	// Round is the round where the incoming message was processed.
	Round uint64 `json:"round"`
	// Caller is the incoming message submitter address.
	Caller staking.Address `json:"caller"`
	// Tag is an optional tag provided by the caller.
	Tag uint64 `json:"tag,omitempty"`
}

// EventKind returns a string representation of this event's kind.
// removed func

// MessageEvent is a runtime message processed event.
type MessageEvent struct {
	Module string `json:"module,omitempty"`
	Code   uint32 `json:"code,omitempty"`
	Index  uint32 `json:"index,omitempty"`

	// Result contains CBOR-encoded message execution result for successfully executed messages.
	Result cbor.RawMessage `json:"result,omitempty"`
}

// IsSuccess returns true if the event indicates that the message was successfully processed.
// removed func

// Event is a roothash event.
type Event struct {
	Height int64     `json:"height,omitempty"`
	TxHash hash.Hash `json:"tx_hash,omitempty"`

	RuntimeID common.Namespace `json:"runtime_id"`

	ExecutorCommitted            *ExecutorCommittedEvent            `json:"executor_committed,omitempty"`
	ExecutionDiscrepancyDetected *ExecutionDiscrepancyDetectedEvent `json:"execution_discrepancy,omitempty"`
	Finalized                    *FinalizedEvent                    `json:"finalized,omitempty"`
	InMsgProcessed               *InMsgProcessedEvent               `json:"in_msg_processed,omitempty"`
}

// MetricsMonitorable is the interface exposed by backends capable of
// providing metrics data.
// removed interface

// GenesisRuntimeState contains state for runtimes that are restored in a genesis block.
type GenesisRuntimeState struct {
	registry.RuntimeGenesis

	// MessageResults are the message results emitted at the last processed round.
	MessageResults []*MessageEvent `json:"message_results,omitempty"`
}

// Genesis is the roothash genesis state.
type Genesis struct {
	// Parameters are the roothash consensus parameters.
	Parameters ConsensusParameters `json:"params"`

	// RuntimeStates are the runtime states at genesis.
	RuntimeStates map[common.Namespace]*GenesisRuntimeState `json:"runtime_states,omitempty"`
}

// ConsensusParameters are the roothash consensus parameters.
type ConsensusParameters struct {
	// GasCosts are the roothash transaction gas costs.
	GasCosts transaction.Costs `json:"gas_costs,omitempty"`

	// DebugDoNotSuspendRuntimes is true iff runtimes should not be suspended
	// for lack of paying maintenance fees.
	DebugDoNotSuspendRuntimes bool `json:"debug_do_not_suspend_runtimes,omitempty"`

	// DebugBypassStake is true iff the roothash should bypass all of the staking
	// related checks and operations.
	DebugBypassStake bool `json:"debug_bypass_stake,omitempty"`

	// MaxRuntimeMessages is the maximum number of allowed messages that can be emitted by a runtime
	// in a single round.
	MaxRuntimeMessages uint32 `json:"max_runtime_messages"`

	// MaxInRuntimeMessages is the maximum number of allowed incoming messages that can be queued.
	MaxInRuntimeMessages uint32 `json:"max_in_runtime_messages"`

	// MaxEvidenceAge is the maximum age of submitted evidence in the number of rounds.
	MaxEvidenceAge uint64 `json:"max_evidence_age"`

	// MaxPastRootsStored is the maximum number of past runtime state and I/O
	// roots that are stored in the consensus state.
	MaxPastRootsStored uint64 `json:"max_past_roots_stored,omitempty"`
}

// ConsensusParameterChanges are allowed roothash consensus parameter changes.
type ConsensusParameterChanges struct {
	// GasCosts are the new gas costs.
	GasCosts transaction.Costs `json:"gas_costs,omitempty"`

	// MaxRuntimeMessages is the new maximum number of emitted runtime messages.
	MaxRuntimeMessages *uint32 `json:"max_runtime_messages"`

	// MaxInRuntimeMessages is the new maximum number of incoming queued runtime messages.
	MaxInRuntimeMessages *uint32 `json:"max_in_runtime_messages"`

	// MaxEvidenceAge is the new maximum evidence age.
	MaxEvidenceAge *uint64 `json:"max_evidence_age"`

	// MaxPastRootsStored is the new maximum number of past runtime state and I/O
	// roots that are stored in the consensus state.
	MaxPastRootsStored *uint64 `json:"max_past_roots_stored,omitempty"`
}

// Apply applies changes to the given consensus parameters.
// removed func

const (
	// GasOpComputeCommit is the gas operation identifier for compute commits.
	GasOpComputeCommit transaction.Op = "compute_commit"

	// GasOpProposerTimeout is the gas operation identifier for executor propose timeout cost.
	GasOpProposerTimeout transaction.Op = "proposer_timeout"

	// GasOpEvidence is the gas operation identifier for evidence submission transaction cost.
	GasOpEvidence transaction.Op = "evidence"

	// GasOpSubmitMsg is the gas operation identifier for message submission transaction cost.
	GasOpSubmitMsg transaction.Op = "submit_msg"
)

// XXX: Define reasonable default gas costs.

// DefaultGasCosts are the "default" gas costs for operations.
// removed var statement

// VerifyRuntimeParameters verifies whether the runtime parameters are valid in the context of the
// roothash service.
// removed func

// RoundRoots holds the per-round state and I/O roots that are stored in
// consensus state.
type RoundRoots struct {
	// Serialize this struct as an array with two elements to save space in
	// the consensus state.
	_ struct{} `cbor:",toarray"` //nolint

	StateRoot hash.Hash
	IORoot    hash.Hash
}
