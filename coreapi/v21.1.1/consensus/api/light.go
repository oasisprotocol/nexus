package api

import (
	"github.com/oasisprotocol/oasis-indexer/coreapi/v21.1.1/consensus/genesis"
)

// LightClientBackend is the limited consensus interface used by light clients.
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
