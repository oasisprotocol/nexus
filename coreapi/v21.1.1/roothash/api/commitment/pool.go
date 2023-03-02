package commitment

import (
	"github.com/oasisprotocol/oasis-core/go/common/crypto/signature"
	registry "github.com/oasisprotocol/oasis-indexer/coreapi/v21.1.1/registry/api"
	"github.com/oasisprotocol/oasis-indexer/coreapi/v21.1.1/roothash/api/message"
	scheduler "github.com/oasisprotocol/oasis-indexer/coreapi/v21.1.1/scheduler/api"
)

// moduleName is the module name used for namespacing errors.
const moduleName = "roothash/commitment"

// removed var block

const (
	// TimeoutNever is the timeout value that never expires.
	TimeoutNever = 0

	// Backup worker round timeout stretch factor (15/10 = 1.5).
	backupWorkerTimeoutFactorNumerator   = 15
	backupWorkerTimeoutFactorDenominator = 10

	// LogEventDiscrepancyMajorityFailure is a log event value that dependency resoluton with majority failure.
	LogEventDiscrepancyMajorityFailure = "pool/discrepancy_majority_failure"
)

// removed var statement

// SignatureVerifier is an interface for verifying storage and transaction
// scheduler signatures against the active committees.
// removed interface

// NodeLookup is an interface for looking up registry node descriptors.
// removed interface

// MessageValidator is an arbitrary function that validates messages for validity. It can be used
// for gas accounting.
type MessageValidator func(msgs []message.Message) error

// Pool is a serializable pool of commitments that can be used to perform
// discrepancy detection.
//
// The pool is not safe for concurrent use.
type Pool struct {
	// Runtime is the runtime descriptor this pool is collecting the
	// commitments for.
	Runtime *registry.Runtime `json:"runtime"`
	// Committee is the committee this pool is collecting the commitments for.
	Committee *scheduler.Committee `json:"committee"`
	// Round is the current protocol round.
	Round uint64 `json:"round"`
	// ExecuteCommitments are the commitments in the pool iff Committee.Kind
	// is scheduler.KindComputeExecutor.
	ExecuteCommitments map[signature.PublicKey]OpenExecutorCommitment `json:"execute_commitments,omitempty"`
	// Discrepancy is a flag signalling that a discrepancy has been detected.
	Discrepancy bool `json:"discrepancy"`
	// NextTimeout is the time when the next call to TryFinalize(true) should
	// be scheduled to be executed. Zero means that no timeout is to be scheduled.
	NextTimeout int64 `json:"next_timeout"`

	// memberSet is a cached committee member set. It will be automatically
	// constructed based on the passed Committee.
	memberSet map[signature.PublicKey]bool

	// workerSet is a cached committee worker set. It will be automatically
	// constructed based on the passed Committee.
	workerSet map[signature.PublicKey]bool
}

// removed func

// removed func

// removed func

// removed func

// ResetCommitments resets the commitments in the pool, clears the discrepancy flag and the next
// timeout height.
// removed func

// removed func

// removed func

// AddExecutorCommitment verifies and adds a new executor commitment to the pool.
// removed func

// ProcessCommitments performs a single round of commitment checks. If there are enough commitments
// in the pool, it performs discrepancy detection or resolution.
// removed func

// CheckProposerTimeout verifies executor timeout request conditions.
// removed func

// removed func

// TryFinalize attempts to finalize the commitments by performing discrepancy
// detection and discrepancy resolution, based on the state of the pool. It may
// request the caller to schedule timeouts by setting NextTimeout appropriately.
//
// If a timeout occurs and isTimeoutAuthoritative is false, the internal
// discrepancy flag will not be changed but the method will still return the
// ErrDiscrepancyDetected error.
// removed func

// GetExecutorCommitments returns a list of executor commitments in the pool.
// removed func

// IsTimeout returns true if the time is up for pool's TryFinalize to be called.
// removed func
