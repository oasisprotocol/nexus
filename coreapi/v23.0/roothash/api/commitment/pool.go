package commitment

import (
	"github.com/oasisprotocol/nexus/coreapi/v23.0/roothash/api/message"
)

// moduleName is the module name used for namespacing errors.
const moduleName = "roothash/commitment"

// nolint: revive
// removed var block

// LogEventDiscrepancyMajorityFailure is a log event value that dependency resolution with majority failure.
const LogEventDiscrepancyMajorityFailure = "pool/discrepancy_majority_failure"

// removed var statement

// NodeLookup is an interface for looking up registry node descriptors.
// removed interface

// MessageValidator is an arbitrary function that validates messages for validity. It can be used
// for gas accounting.
type MessageValidator func(msgs []message.Message) error

// Pool is a serializable pool of scheduler commitments that can be used to perform discrepancy
// detection and resolution.
//
// The pool is not safe for concurrent use.
type Pool struct {
	// HighestRank is the rank of the highest-ranked scheduler among those who have submitted
	// a commitment for their own proposal. The maximum value indicates that no scheduler
	// has submitted a commitment.
	HighestRank uint64 `json:"highest_rank,omitempty"`
	// SchedulerCommitments is a map that groups scheduler commitments and worker votes
	// by the scheduler's rank.
	SchedulerCommitments map[uint64]*SchedulerCommitment `json:"scheduler_commitments,omitempty"`
	// Discrepancy is a flag signalling that a discrepancy has been detected.
	Discrepancy bool `json:"discrepancy,omitempty"`
}

// NewPool creates a new pool without any commitments and with .
// removed func

// VerifyExecutorCommitment verifies the given executor commitment.
// removed func

// AddVerifiedExecutorCommitment adds a verified executor commitment to the pool.
// removed func

// ProcessCommitments performs discrepancy detection or resolution.
// removed func

// removed func
