package churp

import (
	beacon "github.com/oasisprotocol/nexus/coreapi/v24.0/beacon/api"
	"github.com/oasisprotocol/oasis-core/go/common/crypto/signature"
)

// removed var block

// HandoffRequest represents a handoff request.
type HandoffRequest struct {
	Identity

	// Epoch is the epoch of the handoff.
	Epoch beacon.EpochTime `json:"epoch,omitempty"`
}

// FetchRequest is a fetch handoff data request.
type FetchRequest struct {
	Identity

	// Epoch is the epoch of the handoff.
	Epoch beacon.EpochTime `json:"epoch,omitempty"`

	// NodeIDs contains the public keys of nodes from which to fetch data.
	NodeIDs []signature.PublicKey `json:"node_ids"`
}

// FetchResponse is a fetch handoff data response.
type FetchResponse struct {
	// Completed indicates whether the data fetching was completed.
	Completed bool `json:"completed,omitempty"`

	// Succeeded contains the public keys of nodes from which data was
	// successfully fetched.
	Succeeded []signature.PublicKey `json:"succeeded,omitempty"`

	// Failed contains the public keys of nodes from which data failed
	// to be fetched.
	Failed []signature.PublicKey `json:"failed,omitempty"`
}
