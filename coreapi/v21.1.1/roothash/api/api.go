// Package api implements the root hash backend API and common datastructures.
package api

import (
	"math"

	"github.com/oasisprotocol/oasis-core/go/common"
	"github.com/oasisprotocol/oasis-core/go/common/crypto/hash"
	"github.com/oasisprotocol/oasis-core/go/common/crypto/signature"
	"github.com/oasisprotocol/oasis-indexer/coreapi/v21.1.1/consensus/api/transaction"
	registry "github.com/oasisprotocol/oasis-indexer/coreapi/v21.1.1/registry/api"
	"github.com/oasisprotocol/oasis-indexer/coreapi/v21.1.1/roothash/api/block"
	"github.com/oasisprotocol/oasis-indexer/coreapi/v21.1.1/roothash/api/commitment"
)

const (
	// ModuleName is a unique module name for the roothash module.
	ModuleName = "roothash"

	// RoundInvalid is a special round number that refers to an invalid round.
	RoundInvalid uint64 = math.MaxUint64

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

// ExecutorCommit is the argument set for the ExecutorCommit method.
type ExecutorCommit struct {
	ID      common.Namespace                `json:"id"`
	Commits []commitment.ExecutorCommitment `json:"commits"`
}

// NewExecutorCommitTx creates a new executor commit transaction.
// removed func

// ExecutorProposerTimeoutRequest is an executor proposer timeout request.
type ExecutorProposerTimeoutRequest struct {
	ID    common.Namespace `json:"id"`
	Round uint64           `json:"round"`
}

// NewRequestProposerTimeoutTx creates a new request proposer timeout transaction.
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
	EquivocationBatch    *EquivocationBatchEvidence    `json:"equivocation_batch,omitempty"`
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

// EquivocationBatchEvidence is evidence of executor proposed batch equivocation.
type EquivocationBatchEvidence struct {
	BatchA commitment.SignedProposedBatch `json:"batch_a"`
	BatchB commitment.SignedProposedBatch `json:"batch_b"`
}

// ValidateBasic performs stateless batch evidence validation checks.
//
// Particularly evidence is not verified to not be expired as this requires stateful checks.
// removed func

// NewEvidenceTx creates a new evidence transaction.
// removed func

// RuntimeState is the per-runtime state.
type RuntimeState struct {
	Runtime   *registry.Runtime `json:"runtime"`
	Suspended bool              `json:"suspended,omitempty"`

	GenesisBlock *block.Block `json:"genesis_block"`

	CurrentBlock       *block.Block `json:"current_block"`
	CurrentBlockHeight int64        `json:"current_block_height"`

	// LastNormalRound is the runtime round which was normally processed by the runtime. This is
	// also the round that contains the message results for the last processed runtime messages.
	LastNormalRound uint64 `json:"last_normal_round"`
	// LastNormalHeight is the consensus block height corresponding to LastNormalRound.
	LastNormalHeight int64 `json:"last_normal_height"`

	ExecutorPool *commitment.Pool `json:"executor_pool"`
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

// ExecutionDiscrepancyDetectedEvent is an execute discrepancy detected event.
type ExecutionDiscrepancyDetectedEvent struct {
	// Timeout signals whether the discrepancy was due to a timeout.
	Timeout bool `json:"timeout"`
}

// FinalizedEvent is a finalized event.
type FinalizedEvent struct {
	// Round is the round that was finalized.
	Round uint64 `json:"round"`

	// GoodComputeNodes are the public keys of compute nodes that positively contributed to the
	// round by replicating the computation correctly.
	GoodComputeNodes []signature.PublicKey `json:"good_compute_nodes,omitempty"`

	// BadComputeNodes are the public keys of compute nodes that negatively contributed to the round
	// by causing discrepancies.
	BadComputeNodes []signature.PublicKey `json:"bad_compute_nodes,omitempty"`
}

// MessageEvent is a runtime message processed event.
type MessageEvent struct {
	Module string `json:"module,omitempty"`
	Code   uint32 `json:"code,omitempty"`
	Index  uint32 `json:"index,omitempty"`
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
	Message                      *MessageEvent                      `json:"message,omitempty"`
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

	// MaxEvidenceAge is the maximum age of submitted evidence in the number of rounds.
	MaxEvidenceAge uint64 `json:"max_evidence_age"`
}

const (
	// GasOpComputeCommit is the gas operation identifier for compute commits.
	GasOpComputeCommit transaction.Op = "compute_commit"

	// GasOpProposerTimeout is the gas operation identifier for executor propose timeout cost.
	GasOpProposerTimeout transaction.Op = "proposer_timeout"

	// GasOpEvidence is the gas operation identifier for evidence submission transaction cost.
	GasOpEvidence transaction.Op = "evidence"
)

// XXX: Define reasonable default gas costs.

// DefaultGasCosts are the "default" gas costs for operations.
// removed var statement

// SanityCheckBlocks examines the blocks table.
// removed func

// SanityCheck does basic sanity checking on the genesis state.
// removed func

// VerifyRuntimeParameters verifies whether the runtime parameters are valid in the context of the
// roothash service.
// removed func
