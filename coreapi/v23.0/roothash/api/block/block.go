// Package block implements the roothash block and header.
package block

// Block is an Oasis block.
//
// Keep this in sync with /runtime/src/consensus/roothash/block.rs.
type Block struct {
	// Header is the block header.
	Header Header `json:"header"`
}

// NewGenesisBlock creates a new empty genesis block given a runtime
// id and POSIX timestamp.
// removed func

// NewEmptyBlock creates a new empty block with a specific type.
// removed func
