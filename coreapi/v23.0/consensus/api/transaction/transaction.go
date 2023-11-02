package transaction

import (
	"github.com/oasisprotocol/oasis-core/go/common/cbor"
	"github.com/oasisprotocol/oasis-core/go/common/crypto/signature"
)

// moduleName is the module name used for error definitions.
const moduleName = "consensus/transaction"

// removed var block

// Transaction is an unsigned consensus transaction.
type Transaction struct {
	// Nonce is a nonce to prevent replay.
	Nonce uint64 `json:"nonce"`
	// Fee is an optional fee that the sender commits to pay to execute this
	// transaction.
	Fee *Fee `json:"fee,omitempty"`

	// Method is the method that should be called.
	Method MethodName `json:"method"`
	// Body is the method call body.
	Body cbor.RawMessage `json:"body,omitempty"`
}

// PrettyPrintBody writes a pretty-printed representation of transaction's body
// to the given writer.
// removed func

// PrettyPrint writes a pretty-printed representation of the transaction to the
// given writer.
// removed func

// PrettyType returns a representation of the type that can be used for pretty printing.
// removed func

// SanityCheck performs a basic sanity check on the transaction.
// removed func

// NewTransaction creates a new transaction.
// removed func

// PrettyTransaction is used for pretty-printing transactions so that the actual content is
// displayed instead of the binary blob.
//
// It should only be used for pretty printing.
type PrettyTransaction struct {
	Nonce  uint64      `json:"nonce"`
	Fee    *Fee        `json:"fee,omitempty"`
	Method MethodName  `json:"method"`
	Body   interface{} `json:"body,omitempty"`
}

// SignedTransaction is a signed consensus transaction.
type SignedTransaction struct {
	signature.Signed
}

// Hash returns the cryptographic hash of the encoded transaction.
// removed func

// PrettyPrint writes a pretty-printed representation of the type
// to the given writer.
// removed func

// PrettyType returns a representation of the type that can be used for pretty printing.
// removed func

// Open first verifies the blob signature and then unmarshals the blob.
// removed func

// Sign signs a transaction.
// removed func

// OpenRawTransactions takes a vector of raw byte-serialized SignedTransactions,
// and deserializes them, returning all of the signing public key and deserialized
// Transaction, for the transactions that have valid signatures.
//
// The index of each of the return values is that of the corresponding raw
// transaction input.
// removed func

// MethodSeparator is the separator used to separate backend name from method name.
const MethodSeparator = "."

// MethodPriority is the method handling priority.
type MethodPriority uint8

const (
	// MethodPriorityNormal is the normal method priority.
	MethodPriorityNormal = 0
	// MethodPriorityCritical is the priority for methods critical to the protocol operation.
	MethodPriorityCritical = 255
)

// MethodMetadata is the method metadata.
type MethodMetadata struct {
	Priority MethodPriority
}

// MethodMetadataProvider is the method metadata provider interface that can be implemented by
// method body types to provide additional method metadata.
// removed interface

// MethodName is a method name.
type MethodName string

// SanityCheck performs a basic sanity check on the method name.
// removed func

// BodyType returns the registered body type associated with this method.
// removed func

// Metadata returns the method metadata.
// removed func

// IsCritical returns true if the method is critical for the operation of the protocol.
// removed func

// NewMethodName creates a new method name.
//
// Module and method pair must be unique. If they are not, this method
// will panic.
// removed func

// Proof is a proof of transaction inclusion in a block.
type Proof struct {
	// Height is the block height at which the transaction was published.
	Height int64 `json:"height"`

	// RawProof is the actual raw proof.
	RawProof []byte `json:"raw_proof"`
}
