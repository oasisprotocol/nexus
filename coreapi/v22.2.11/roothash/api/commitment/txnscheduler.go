package commitment

import (
	"github.com/oasisprotocol/oasis-core/go/common/crypto/hash"
	"github.com/oasisprotocol/oasis-core/go/common/crypto/signature"
)

// ProposalSignatureContext is the context used for signing propose batch dispatch messages.
// removed var block

// ProposalHeader is the header of the batch proposal.
type ProposalHeader struct {
	// Round is the proposed round number.
	Round uint64 `json:"round"`

	// PreviousHash is the hash of the block header on which the batch should be based.
	PreviousHash hash.Hash `json:"previous_hash"`

	// BatchHash is the hash of the content of the batch.
	BatchHash hash.Hash `json:"batch_hash"`
}

// Sign signs the proposal header.
// removed func

// Equal compares against another proposal header for equality.
// removed func

// Proposal is a batch proposal.
type Proposal struct {
	// NodeID is the public key of the node that generated this proposal.
	NodeID signature.PublicKey `json:"node_id"`

	// Header is the proposal header.
	Header ProposalHeader `json:"header"`

	// Signature is the proposal header signature.
	Signature signature.RawSignature `json:"sig"`

	// Batch is an ordered list of all transaction hashes that should be in a batch. In case of
	// the proposal being submitted as equivocation evidence, this field should be omitted.
	Batch []hash.Hash `json:"batch,omitempty"`
}

// Sign signs the proposal header and sets the signature on the proposal.
// removed func

// Verify verifies that the header signature is valid.
// removed func

// GetTransactionScheduler returns the transaction scheduler of the provided
// committee based on the provided round.
// removed func
