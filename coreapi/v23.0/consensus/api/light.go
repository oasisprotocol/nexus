package api

import (
	"time"

	"github.com/oasisprotocol/oasis-core/go/common/crypto/hash"

	"github.com/oasisprotocol/nexus/coreapi/v23.0/consensus/genesis"
)

// LightService is a consensus light client service.
// removed interface

// LightClient is a consensus light client interface.
// removed interface

// LightBlock is a light consensus block suitable for syncing light clients.
type LightBlock struct {
	// Height contains the block height.
	Height int64 `json:"height"`
	// Meta contains the consensus backend specific light block.
	Meta []byte `json:"meta"`
}

// Parameters are the consensus backend parameters.
type Parameters struct {
	// Height contains the block height these consensus parameters are for.
	Height int64 `json:"height"`
	// Parameters are the backend agnostic consensus parameters.
	Parameters genesis.Parameters `json:"parameters"`
	// Meta contains the consensus backend specific consensus parameters.
	Meta []byte `json:"meta"`
}

// Evidence is evidence of a node's Byzantine behavior.
type Evidence struct {
	// Meta contains the consensus backend specific evidence.
	Meta []byte `json:"meta"`
}

// LightClientStatus is the current light client status overview.
type LightClientStatus struct {
	// LatestHeight is the height of the latest block.
	LatestHeight int64 `json:"latest_height"`
	// LatestHash is the hash of the latest block.
	LatestHash hash.Hash `json:"latest_hash"`
	// LatestTime is the timestamp of the latest block.
	LatestTime time.Time `json:"latest_time"`

	// OldestHeight is the height of the oldest block.
	OldestHeight int64 `json:"oldest_height"`
	// LatestHash is the hash of the oldest block.
	OldestHash hash.Hash `json:"oldest_hash"`
	// OldestTime is the timestamp of the oldest block.
	OldestTime time.Time `json:"oldest_time"`

	// PeersIDs are the light client provider peer identifiers.
	PeerIDs []string `json:"peer_ids"`
}
