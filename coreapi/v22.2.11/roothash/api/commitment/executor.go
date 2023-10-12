// Package commitment defines a roothash commitment.
package commitment

import (
	"github.com/oasisprotocol/nexus/coreapi/v22.2.11/roothash/api/message"
	"github.com/oasisprotocol/oasis-core/go/common/crypto/hash"
	"github.com/oasisprotocol/oasis-core/go/common/crypto/signature"
)

// removed var block

// ComputeResultsHeader is the header of a computed batch output by a runtime. This
// header is a compressed representation (e.g., hashes instead of full content) of
// the actual results.
//
// These headers are signed by RAK inside the runtime and included in executor
// commitments.
//
// Keep the roothash RAK validation in sync with changes to this structure.
type ComputeResultsHeader struct {
	Round        uint64    `json:"round"`
	PreviousHash hash.Hash `json:"previous_hash"`

	// Optional fields (may be absent for failure indication).

	IORoot       *hash.Hash `json:"io_root,omitempty"`
	StateRoot    *hash.Hash `json:"state_root,omitempty"`
	MessagesHash *hash.Hash `json:"messages_hash,omitempty"`

	// InMessagesHash is the hash of processed incoming messages.
	InMessagesHash *hash.Hash `json:"in_msgs_hash,omitempty"`
	// InMessagesCount is the number of processed incoming messages.
	InMessagesCount uint32 `json:"in_msgs_count,omitempty"`
}

// IsParentOf returns true iff the header is the parent of a child header.
// removed func

// EncodedHash returns the encoded cryptographic hash of the header.
// removed func

// ExecutorCommitmentFailure is the executor commitment failure reason.
type ExecutorCommitmentFailure uint8

const (
	// FailureNone indicates that no failure has occurred.
	FailureNone ExecutorCommitmentFailure = 0
	// FailureUnknown indicates a generic failure.
	FailureUnknown ExecutorCommitmentFailure = 1
	// FailureStateUnavailable indicates that batch processing failed due to the state being
	// unavailable.
	FailureStateUnavailable ExecutorCommitmentFailure = 2
)

// ExecutorCommitmentHeader is the header of an executor commitment.
type ExecutorCommitmentHeader struct {
	ComputeResultsHeader

	Failure ExecutorCommitmentFailure `json:"failure,omitempty"`

	// Optional fields (may be absent for failure indication).

	RAKSignature *signature.RawSignature `json:"rak_sig,omitempty"`
}

// SetFailure sets failure reason and clears any fields that should be clear
// in a failure indicating commitment.
// removed func

// Sign signs the executor commitment header.
// removed func

// VerifyRAK verifies the RAK signature.
// removed func

// MostlyEqual compares against another executor commitment header for equality.
//
// The RAKSignature field is not compared.
// removed func

// ExecutorCommitment is a commitment to results of processing a proposed runtime block.
type ExecutorCommitment struct {
	// NodeID is the public key of the node that generated this commitment.
	NodeID signature.PublicKey `json:"node_id"`

	// Header is the commitment header.
	Header ExecutorCommitmentHeader `json:"header"`

	// Signature is the commitment header signature.
	Signature signature.RawSignature `json:"sig"`

	// Messages are the messages emitted by the runtime.
	//
	// This field is only present in case this commitment belongs to the proposer. In case of
	// the commitment being submitted as equivocation evidence, this field should be omitted.
	Messages []message.Message `json:"messages,omitempty"`
}

// Sign signs the executor commitment header and sets the signature on the commitment.
// removed func

// Verify verifies that the header signature is valid.
// removed func

// ValidateBasic performs basic executor commitment validity checks.
// removed func

// MostlyEqual returns true if the commitment is mostly equal to another
// specified commitment as per discrepancy detection criteria.
// removed func

// IsIndicatingFailure returns true if this commitment indicates a failure.
// removed func

// ToVote returns a hash that represents a vote for this commitment as
// per discrepancy resolution criteria.
// removed func

// ToDDResult returns a commitment-specific result after discrepancy
// detection.
// removed func
