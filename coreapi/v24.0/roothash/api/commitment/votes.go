package commitment

import (
	"github.com/oasisprotocol/oasis-core/go/common/crypto/hash"
	"github.com/oasisprotocol/oasis-core/go/common/crypto/signature"
)

// SchedulerCommitment is a structure for storing scheduler commitment and its votes.
type SchedulerCommitment struct {
	// Commitment is a verified scheduler's Commitment for which votes are being collected.
	Commitment *ExecutorCommitment `json:"commitment,omitempty"`

	// Votes is a map that collects Votes from nodes in the form of commitment hashes.
	//
	// A nil vote indicates a failure.
	Votes map[signature.PublicKey]*hash.Hash `json:"votes,omitempty"`
}

// Add converts the provided executor commitment into a vote and adds it to the votes map.
//
// It returns an error if the node has already submitted a vote.
// removed func
