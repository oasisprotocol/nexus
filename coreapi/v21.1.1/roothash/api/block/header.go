package block

import (
	"github.com/oasisprotocol/oasis-core/go/common"
	"github.com/oasisprotocol/oasis-core/go/common/crypto/hash"
	"github.com/oasisprotocol/oasis-core/go/common/crypto/signature"
)

// ErrInvalidVersion is the error returned when a version is invalid.
// removed var statement

// HeaderType is the type of header.
type HeaderType uint8

const (
	// Invalid is an invalid header type and should never be stored.
	Invalid HeaderType = 0

	// Normal is a normal header.
	Normal HeaderType = 1

	// RoundFailed is a header resulting from a failed round. Such a
	// header contains no transactions but advances the round as normal
	// to prevent replays of old commitments.
	RoundFailed HeaderType = 2

	// EpochTransition is a header resulting from an epoch transition.
	//
	// Such a header contains no transactions but advances the round as
	// normal.
	// TODO: Consider renaming this to CommitteeTransition.
	EpochTransition HeaderType = 3

	// Suspended is a header resulting from the runtime being suspended.
	//
	// Such a header contains no transactions but advances the round as
	// normal.
	Suspended HeaderType = 4
)

// Header is a block header.
//
// Keep this in sync with /runtime/src/common/roothash.rs.
type Header struct { // nolint: maligned
	// Version is the protocol version number.
	Version uint16 `json:"version"`

	// Namespace is the header's chain namespace.
	Namespace common.Namespace `json:"namespace"`

	// Round is the block round.
	Round uint64 `json:"round"`

	// Timestamp is the block timestamp (POSIX time).
	Timestamp uint64 `json:"timestamp"`

	// HeaderType is the header type.
	HeaderType HeaderType `json:"header_type"`

	// PreviousHash is the previous block hash.
	PreviousHash hash.Hash `json:"previous_hash"`

	// IORoot is the I/O merkle root.
	IORoot hash.Hash `json:"io_root"`

	// StateRoot is the state merkle root.
	StateRoot hash.Hash `json:"state_root"`

	// MessagesHash is the hash of emitted runtime messages.
	MessagesHash hash.Hash `json:"messages_hash"`

	// StorageSignatures are the storage receipt signatures for the merkle
	// roots.
	StorageSignatures []signature.Signature `json:"storage_signatures"`
}

// IsParentOf returns true iff the header is the parent of a child header.
// removed func

// MostlyEqual compares vs another header for equality, omitting the
// StorageSignatures field as it is not universally guaranteed to be present.
//
// Locations where this matter should do the comparison manually.
// removed func

// EncodedHash returns the encoded cryptographic hash of the header.
func (h *Header) EncodedHash() hash.Hash {
	return hash.NewFrom(h)
}

// StorageRoots returns the storage roots contained in this header.
// removed func

// RootsForStorageReceipt gets the merkle roots that must be part of
// a storage receipt.
// removed func

// RootTypesForStorageReceipt gets the storage root type sequence for the roots
// returned by RootsForStorageReceipt.
// removed func

// VerifyStorageReceiptSignatures validates that the storage receipt signatures
// match the signatures for the current merkle roots.
//
// Note: Ensuring that the signatures are signed by keypair(s) that are
// expected is the responsibility of the caller.
// removed func

// VerifyStorageReceipt validates that the provided storage receipt
// matches the header.
// removed func
