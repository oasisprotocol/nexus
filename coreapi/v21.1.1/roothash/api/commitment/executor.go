// Package commitment defines a roothash commitment.
package commitment

import (
	"github.com/oasisprotocol/oasis-core/go/common/cbor"
	"github.com/oasisprotocol/oasis-core/go/common/crypto/hash"
	"github.com/oasisprotocol/oasis-core/go/common/crypto/signature"

	"github.com/oasisprotocol/nexus/coreapi/v21.1.1/roothash/api/message"
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
	// FailureStorageUnavailable indicates that batch processing failed due to
	// storage being unavailable.
	FailureStorageUnavailable ExecutorCommitmentFailure = 2
)

// ComputeBody holds the data signed in a compute worker commitment.
type ComputeBody struct {
	Header  ComputeResultsHeader      `json:"header"`
	Failure ExecutorCommitmentFailure `json:"failure,omitempty"`

	TxnSchedSig      signature.Signature   `json:"txn_sched_sig"`
	InputRoot        hash.Hash             `json:"input_root"`
	InputStorageSigs []signature.Signature `json:"input_storage_sigs"`

	// Optional fields (may be absent for failure indication).

	StorageSignatures []signature.Signature   `json:"storage_signatures,omitempty"`
	RakSig            *signature.RawSignature `json:"rak_sig,omitempty"`
	Messages          []message.Message       `json:"messages,omitempty"`
}

// SetFailure sets failure reason and clears any fields that should be clear
// in a failure indicating commitment.
// removed func

// VerifyTxnSchedSignature rebuilds the batch dispatch message from the data
// in the ComputeBody struct and verifies if the txn scheduler signature
// matches what we're seeing.
// removed func

// RootsForStorageReceipt gets the merkle roots that must be part of
// a storage receipt.
// removed func

// RootTypesForStorageReceipt gets the storage root types that must be part
// of a storage receipt.
// removed func

// ValidateBasic performs basic executor commitment validity checks.
// removed func

// VerifyStorageReceiptSignatures validates that the storage receipt signatures
// match the signatures for the current merkle roots.
//
// Note: Ensuring that the signature is signed by the keypair(s) that are
// expected is the responsibility of the caller.
// removed func

// VerifyStorageReceipt validates that the provided storage receipt
// matches the header.
// removed func

// ExecutorCommitment is a roothash commitment from an executor worker.
//
// The signed content is ComputeBody.
type ExecutorCommitment struct {
	signature.Signed
}

// Equal compares vs another ExecutorCommitment for equality.
// removed func

// OpenExecutorCommitment is an executor commitment that has been verified and
// deserialized.
//
// The open commitment still contains the original signed commitment.
type OpenExecutorCommitment struct {
	ExecutorCommitment

	Body *ComputeBody `json:"-"` // No need to serialize as it can be reconstructed.
}

// UnmarshalCBOR handles CBOR unmarshalling from passed data.
func (c *OpenExecutorCommitment) UnmarshalCBOR(data []byte) error {
	if err := cbor.Unmarshal(data, &c.ExecutorCommitment); err != nil {
		return err
	}

	c.Body = new(ComputeBody)
	return cbor.Unmarshal(c.Blob, c.Body)
}

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

// Open validates the executor commitment signature, and de-serializes the message.
// This does not validate the RAK signature.
// removed func

// SignExecutorCommitment serializes the message and signs the commitment.
// removed func
