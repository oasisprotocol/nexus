// Package api defines the Oasis genesis block.
package api

import (
	"time"

	"github.com/oasisprotocol/oasis-core/go/common/crypto/hash"

	beacon "github.com/oasisprotocol/nexus/coreapi/v22.2.11/beacon/api"
	consensus "github.com/oasisprotocol/nexus/coreapi/v22.2.11/consensus/genesis"
	governance "github.com/oasisprotocol/nexus/coreapi/v22.2.11/governance/api"
	keymanager "github.com/oasisprotocol/nexus/coreapi/v22.2.11/keymanager/api"
	registry "github.com/oasisprotocol/nexus/coreapi/v22.2.11/registry/api"
	roothash "github.com/oasisprotocol/nexus/coreapi/v22.2.11/roothash/api"
	scheduler "github.com/oasisprotocol/nexus/coreapi/v22.2.11/scheduler/api"
	staking "github.com/oasisprotocol/nexus/coreapi/v22.2.11/staking/api"
)

const filePerm = 0o600

// Document is a genesis document.
type Document struct {
	// Height is the block height at which the document was generated.
	Height int64 `json:"height"`
	// Time is the time the genesis block was constructed.
	Time time.Time `json:"genesis_time"`
	// ChainID is the ID of the chain.
	ChainID string `json:"chain_id"`
	// Registry is the registry genesis state.
	Registry registry.Genesis `json:"registry"`
	// RootHash is the roothash genesis state.
	RootHash roothash.Genesis `json:"roothash"`
	// Staking is the staking genesis state.
	Staking staking.Genesis `json:"staking"`
	// KeyManager is the key manager genesis state.
	KeyManager keymanager.Genesis `json:"keymanager"`
	// Scheduler is the scheduler genesis state.
	Scheduler scheduler.Genesis `json:"scheduler"`
	// Beacon is the beacon genesis state.
	Beacon beacon.Genesis `json:"beacon"`
	// Governance is the governance genesis state.
	Governance governance.Genesis `json:"governance"`
	// Consensus is the consensus genesis state.
	Consensus consensus.Genesis `json:"consensus"`
	// HaltEpoch is the epoch height at which the network will stop processing
	// any transactions and will halt.
	HaltEpoch beacon.EpochTime `json:"halt_epoch"`
	// Extra data is arbitrary extra data that is part of the
	// genesis block but is otherwise ignored by the protocol.
	ExtraData map[string][]byte `json:"extra_data"`

	cachedHash *hash.Hash
}

// Hash returns the cryptographic hash of the encoded genesis document.
//
// Calling this method will cause the computed hash to be cached so make sure
// that the document is not modified later.
// removed func

// ChainContext returns a string that can be used as a chain domain separation
// context. Changing this (or any data it is derived from) invalidates all
// signatures that use chain domain separation.
//
// Currently this uses the hex-encoded cryptographic hash of the encoded
// genesis document.
// removed func

// SetChainContext configures the global chain domain separation context.
//
// This method can only be called once during the application's lifetime and
// will panic otherwise.
// removed func

// CanonicalJSON returns the canonical form of the genesis document serialized
// into a file.
//
// This is a pretty-printed JSON file with 2-space indents following Go
// encoding/json package's JSON marshalling rules with a newline at the end.
// removed func

// WriteFileJSON writes the canonical form of genesis document into a file.
// removed func

// Provider is a genesis document provider.
// removed interface
