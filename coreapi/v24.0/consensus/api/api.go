// Package api provides the implementation agnostic consensus API.
package api

import (
	"context"
	"fmt"
	"strings"
	"time"

	beacon "github.com/oasisprotocol/nexus/coreapi/v24.0/beacon/api"
	"github.com/oasisprotocol/nexus/coreapi/v24.0/common/node"
	"github.com/oasisprotocol/nexus/coreapi/v24.0/consensus/api/transaction"
	"github.com/oasisprotocol/nexus/coreapi/v24.0/consensus/api/transaction/results"
	staking "github.com/oasisprotocol/nexus/coreapi/v24.0/staking/api"
	"github.com/oasisprotocol/oasis-core/go/common/cbor"
	"github.com/oasisprotocol/oasis-core/go/common/crypto/hash"
	"github.com/oasisprotocol/oasis-core/go/common/crypto/signature"
	"github.com/oasisprotocol/oasis-core/go/common/version"
	mkvsNode "github.com/oasisprotocol/oasis-core/go/storage/mkvs/node"
)

const (
	// ModuleName is the module name used for error definitions.
	ModuleName = "consensus"

	// HeightLatest is the height that represents the most recent block height.
	HeightLatest int64 = 0
)

// removed var block

// FeatureMask is the consensus backend feature bitmask.
type FeatureMask uint8

const (
	// FeatureServices indicates support for communicating with consensus services.
	FeatureServices FeatureMask = 1 << 0

	// FeatureFullNode indicates that the consensus backend is independently fully verifying all
	// consensus-layer blocks.
	FeatureFullNode FeatureMask = 1 << 1

	// FeatureArchiveNode indicates that the node is an archive node.
	FeatureArchiveNode FeatureMask = 1 << 2
)

// String returns a string representation of the consensus backend feature bitmask.
func (m FeatureMask) String() string {
	var ret []string
	if m&FeatureServices != 0 {
		ret = append(ret, "consensus services")
	}
	if m&FeatureFullNode != 0 {
		ret = append(ret, "full node")
	}
	if m&FeatureArchiveNode != 0 {
		ret = append(ret, "archive node")
	}

	return strings.Join(ret, ",")
}

// Has checks whether the feature bitmask includes specific features.
// removed func

// ClientBackend is a consensus interface used by clients that connect to the local full node.
// removed interface

// Block is a consensus block.
//
// While some common fields are provided, most of the structure is dependent on
// the actual backend implementation.
type Block struct {
	// Height contains the block height.
	Height int64 `json:"height"`
	// Hash contains the block header hash.
	Hash hash.Hash `json:"hash"`
	// Time is the second-granular consensus time.
	Time time.Time `json:"time"`
	// StateRoot is the Merkle root of the consensus state tree.
	StateRoot mkvsNode.Root `json:"state_root"`
	// Size is the size of the block in bytes.
	Size uint64 `json:"size,omitempty"` // Added in 24.3.
	// Meta contains the consensus backend specific block metadata.
	Meta cbor.RawMessage `json:"meta"`
}

// NextBlockState has the state of the next block being voted on by validators.
type NextBlockState struct {
	Height int64 `json:"height"`

	NumValidators uint64 `json:"num_validators"`
	VotingPower   uint64 `json:"voting_power"`

	Prevotes   Votes `json:"prevotes"`
	Precommits Votes `json:"precommits"`
}

// Votes are the votes for the next block.
type Votes struct {
	VotingPower uint64  `json:"voting_power"`
	Ratio       float64 `json:"ratio"`
	Votes       []Vote  `json:"votes"`
}

// Vote contains metadata about a vote for the next block.
type Vote struct {
	NodeID        signature.PublicKey `json:"node_id"`
	EntityID      signature.PublicKey `json:"entity_id"`
	EntityAddress staking.Address     `json:"entity_address"`
	VotingPower   uint64              `json:"voting_power"`
}

// StatusState is the concise status state of the consensus backend.
type StatusState uint8

var (
	// StatusStateReady is the ready status state.
	StatusStateReady StatusState
	// StatusStateSyncing is the syncing status state.
	StatusStateSyncing StatusState = 1
)

// String returns a string representation of a status state.
func (s StatusState) String() string {
	switch s {
	case StatusStateReady:
		return "ready"
	case StatusStateSyncing:
		return "syncing"
	default:
		return "[invalid status state]"
	}
}

// MarshalText encodes a StatusState into text form.
func (s StatusState) MarshalText() ([]byte, error) {
	switch s {
	case StatusStateReady:
		return []byte(StatusStateReady.String()), nil
	case StatusStateSyncing:
		return []byte(StatusStateSyncing.String()), nil
	default:
		return nil, fmt.Errorf("invalid StatusState: %d", s)
	}
}

// UnmarshalText decodes a text slice into a StatusState.
func (s *StatusState) UnmarshalText(text []byte) error {
	switch string(text) {
	case StatusStateReady.String():
		*s = StatusStateReady
	case StatusStateSyncing.String():
		*s = StatusStateSyncing
	default:
		return fmt.Errorf("invalid StatusState: %s", string(text))
	}
	return nil
}

// Status is the current status overview.
type Status struct { // nolint: maligned
	// Status is an concise status of the consensus backend.
	Status StatusState `json:"status"`

	// Version is the version of the consensus protocol that the node is using.
	Version version.Version `json:"version"`
	// Backend is the consensus backend identifier.
	Backend string `json:"backend"`
	// Features are the indicated consensus backend features.
	Features FeatureMask `json:"features"`

	// LatestHeight is the height of the latest block.
	LatestHeight int64 `json:"latest_height"`
	// LatestHash is the hash of the latest block.
	LatestHash hash.Hash `json:"latest_hash"`
	// LatestTime is the timestamp of the latest block.
	LatestTime time.Time `json:"latest_time"`
	// LatestEpoch is the epoch of the latest block.
	LatestEpoch beacon.EpochTime `json:"latest_epoch"`
	// LatestStateRoot is the Merkle root of the consensus state tree.
	LatestStateRoot mkvsNode.Root `json:"latest_state_root"`
	// LatestBlockSize is the size (in bytes) of the latest block.
	LatestBlockSize uint64 `json:"latest_block_size,omitempty"` // Added in 24.3.

	// GenesisHeight is the height of the genesis block.
	GenesisHeight int64 `json:"genesis_height"`
	// GenesisHash is the hash of the genesis block.
	GenesisHash hash.Hash `json:"genesis_hash"`

	// LastRetainedHeight is the height of the oldest retained block.
	LastRetainedHeight int64 `json:"last_retained_height"`
	// LastRetainedHash is the hash of the oldest retained block.
	LastRetainedHash hash.Hash `json:"last_retained_hash"`

	// ChainContext is the chain domain separation context.
	ChainContext string `json:"chain_context"`

	// IsValidator returns whether the current node is part of the validator set.
	IsValidator bool `json:"is_validator"`

	// P2P is the P2P status of the node.
	P2P *P2PStatus `json:"p2p,omitempty"`
}

// P2PStatus is the P2P status of a node.
type P2PStatus struct {
	// PubKey is the public key used for consensus P2P communication.
	PubKey signature.PublicKey `json:"pub_key"`

	// PeerID is the peer ID derived by hashing peer's public key.
	PeerID string `json:"peer_id"`

	// Addresses is a list of configured P2P addresses used when registering the node.
	Addresses []node.ConsensusAddress `json:"addresses"`

	// Peers is a list of node's peers.
	Peers []string `json:"peers"`
}

// Backend is an interface that a consensus backend must provide.
// removed interface

// HaltHook is a function that gets called when consensus needs to halt for some reason.
type HaltHook func(ctx context.Context, blockHeight int64, epoch beacon.EpochTime, err error)

// ServicesBackend is an interface for consensus backends which indicate support for
// communicating with consensus services.
//
// In case the feature is absent, these methods may return nil or ErrUnsupported.
// removed interface

// TransactionAuthHandler is the interface for handling transaction authentication
// (checking nonces and fees).
// removed interface

// EstimateGasRequest is a EstimateGas request.
type EstimateGasRequest struct {
	Signer      signature.PublicKey      `json:"signer"`
	Transaction *transaction.Transaction `json:"transaction"`
}

// GetSignerNonceRequest is a GetSignerNonce request.
type GetSignerNonceRequest struct {
	AccountAddress staking.Address `json:"account_address"`
	Height         int64           `json:"height"`
}

// TransactionsWithResults is GetTransactionsWithResults response.
//
// Results[i] are the results of executing Transactions[i].
type TransactionsWithResults struct {
	Transactions [][]byte          `json:"transactions"`
	Results      []*results.Result `json:"results"`
}

// TransactionsWithProofs is GetTransactionsWithProofs response.
//
// Proofs[i] is a proof of block inclusion for Transactions[i].
type TransactionsWithProofs struct {
	Transactions [][]byte `json:"transactions"`
	Proofs       [][]byte `json:"proofs"`
}
