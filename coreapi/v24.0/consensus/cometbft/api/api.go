package api

import (
	"fmt"

	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	cmttypes "github.com/cometbft/cometbft/types"
	"github.com/oasisprotocol/oasis-core/go/common/cbor"
)

// BlockMeta is the CometBFT-specific per-block metadata.
type BlockMeta struct {
	// Header is the CometBFT block header.
	Header *cmttypes.Header `json:"header"`
	// LastCommit is the CometBFT last commit info.
	LastCommit *cmttypes.Commit `json:"last_commit"`
}

// TryUnmarshal attempts to unmarshal the given data into a BlockMeta.
//
// It first tries to unmarshal into V1, and if that fails, it tries to
// unmarshal into V2.
//
// We only try to unmarshal into the version of the metadata structure
// starting at Oasis-Core Eden (V1), and into V2 (starting at #6235).
// This may fail on blocks from an incompatible earlier version.
func (b *BlockMeta) TryUnmarshal(data []byte) error {
	// Try to unmarshal into V1 first.
	switch err := cbor.Unmarshal(data, &b); {
	case err == nil:
		return nil
	default:
		// Continue below.
	}

	// Try unmarshal into V2.
	var metaV2 blockMetaV2
	if err := cbor.Unmarshal(data, &metaV2); err != nil {
		return err
	}

	// V2 uses protobuf encoding. Try decoding into BlockMeta.
	var lastCommitProto cmtproto.Commit
	if err := lastCommitProto.Unmarshal(metaV2.LastCommit); err != nil {
		return fmt.Errorf("malformed V2 block meta last commit: %w", err)
	}
	lastCommit, err := cmttypes.CommitFromProto(&lastCommitProto)
	if err != nil {
		return fmt.Errorf("malformed V2 block meta last commit: %w", err)
	}

	var lastHeaderProto cmtproto.Header
	if err := lastHeaderProto.Unmarshal(metaV2.Header); err != nil {
		return fmt.Errorf("malformed V2 block meta last header: %w", err)
	}
	header, err := cmttypes.HeaderFromProto(&lastHeaderProto)
	if err != nil {
		return fmt.Errorf("malformed V2 block meta last header: %w", err)
	}

	b.Header = &header
	b.LastCommit = lastCommit
	return nil
}

// blockMetaV2 is the CometBFT-specific per-block metadata used in:
// https://github.com/oasisprotocol/oasis-core/pull/6235
type blockMetaV2 struct {
	// Header is the CometBFT block header.
	Header []byte `json:"header"`
	// LastCommit is the CometBFT last commit info.
	LastCommit []byte `json:"last_commit"`
}
