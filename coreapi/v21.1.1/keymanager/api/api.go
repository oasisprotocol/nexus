// Package api implements the key manager management API and common data types.
package api

import (
	"github.com/oasisprotocol/oasis-core/go/common"
	"github.com/oasisprotocol/oasis-core/go/common/crypto/signature"
)

const (
	// ModuleName is a unique module name for the keymanager module.
	ModuleName = "keymanager"

	// ChecksumSize is the length of checksum in bytes.
	ChecksumSize = 32

	// EnclaveRPCEndpoint is the name of the key manager EnclaveRPC endpoint.
	EnclaveRPCEndpoint = "key-manager"
)

// removed var block

// Status is the current key manager status.
type Status struct {
	// ID is the runtime ID of the key manager.
	ID common.Namespace `json:"id"`

	// IsInitialized is true iff the key manager is done initializing.
	IsInitialized bool `json:"is_initialized"`

	// IsSecure is true iff the key manager is secure.
	IsSecure bool `json:"is_secure"`

	// Checksum is the key manager master secret verification checksum.
	Checksum []byte `json:"checksum"`

	// Nodes is the list of currently active key manager node IDs.
	Nodes []signature.PublicKey `json:"nodes"`

	// Policy is the key manager policy.
	Policy *SignedPolicySGX `json:"policy"`
}

// Backend is a key manager management implementation.
// removed interface

// NewUpdatePolicyTx creates a new policy update transaction.
// removed func

// InitResponse is the initialization RPC response, returned as part of a
// SignedInitResponse from the key manager enclave.
type InitResponse struct {
	IsSecure       bool   `json:"is_secure"`
	Checksum       []byte `json:"checksum"`
	PolicyChecksum []byte `json:"policy_checksum"`
}

// SignedInitResponse is the signed initialization RPC response, returned
// from the key manager enclave.
type SignedInitResponse struct {
	InitResponse InitResponse `json:"init_response"`
	Signature    []byte       `json:"signature"`
}

// removed func

// VerifyExtraInfo verifies and parses the per-node + per-runtime ExtraInfo
// blob for a key manager.
// removed func

// Genesis is the key manager management genesis state.
type Genesis struct {
	Statuses []*Status `json:"statuses,omitempty"`
}

// SanityCheckStatuses examines the statuses table.
// removed func

// SanityCheck does basic sanity checking on the genesis state.
// removed func

// removed func
