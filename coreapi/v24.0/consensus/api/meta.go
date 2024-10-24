package api

import (
	"github.com/oasisprotocol/oasis-core/go/common/crypto/hash"
)

// MethodMeta is the method name for the special block metadata transaction.
// removed var statement

// BlockMetadataMaxSize is the maximum size of a fully populated and signed block metadata
// transaction.
//
// This should be less than any reasonably configured MaxTxSize.
const BlockMetadataMaxSize = 16_384

// BlockMetadata contains additional metadata related to the executing block.
//
// The metadata is included in the form of a special transaction where this structure is the
// transaction body.
type BlockMetadata struct {
	// StateRoot is the state root after executing all logic in the block.
	StateRoot hash.Hash `json:"state_root"`
	// EventsRoot is the provable events root.
	EventsRoot []byte `json:"events_root"`
}

// ValidateBasic performs basic block metadata structure validation.
// removed func

// NewBlockMetadataTx creates a new block metadata transaction.
// removed func
