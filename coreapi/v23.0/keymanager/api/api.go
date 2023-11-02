// Package api implements the key manager management API and common data types.
package api

import (
	"github.com/oasisprotocol/curve25519-voi/primitives/x25519"

	"github.com/oasisprotocol/oasis-core/go/common"
	"github.com/oasisprotocol/oasis-core/go/common/crypto/signature"

	beacon "github.com/oasisprotocol/nexus/coreapi/v23.0/beacon/api"
	"github.com/oasisprotocol/nexus/coreapi/v23.0/consensus/api/transaction"
)

const (
	// ModuleName is a unique module name for the keymanager module.
	ModuleName = "keymanager"

	// ChecksumSize is the length of checksum in bytes.
	ChecksumSize = 32

	// KeyPairIDSize is the size of a key pair ID in bytes.
	KeyPairIDSize = 32
)

// removed var block

const (
	// GasOpUpdatePolicy is the gas operation identifier for policy updates
	// costs.
	GasOpUpdatePolicy transaction.Op = "update_policy"
	// GasOpPublishMasterSecret is the gas operation identifier for publishing
	// key manager master secret.
	GasOpPublishMasterSecret transaction.Op = "publish_master_secret"
	// GasOpPublishEphemeralSecret is the gas operation identifier for publishing
	// key manager ephemeral secret.
	GasOpPublishEphemeralSecret transaction.Op = "publish_ephemeral_secret"
)

// XXX: Define reasonable default gas costs.

// DefaultGasCosts are the "default" gas costs for operations.
// removed var statement

// KeyPairID is a 256-bit key pair identifier.
type KeyPairID [KeyPairIDSize]byte

// Status is the current key manager status.
type Status struct {
	// ID is the runtime ID of the key manager.
	ID common.Namespace `json:"id"`

	// IsInitialized is true iff the key manager is done initializing.
	IsInitialized bool `json:"is_initialized"`

	// IsSecure is true iff the key manager is secure.
	IsSecure bool `json:"is_secure"`

	// Generation is the generation of the latest master secret.
	Generation uint64 `json:"generation,omitempty"`

	// RotationEpoch is the epoch of the last master secret rotation.
	RotationEpoch beacon.EpochTime `json:"rotation_epoch,omitempty"`

	// Checksum is the key manager master secret verification checksum.
	Checksum []byte `json:"checksum"`

	// Nodes is the list of currently active key manager node IDs.
	Nodes []signature.PublicKey `json:"nodes"`

	// Policy is the key manager policy.
	Policy *SignedPolicySGX `json:"policy"`

	// RSK is the runtime signing key of the key manager.
	RSK *signature.PublicKey `json:"rsk,omitempty"`
}

// NextGeneration returns the generation of the next master secret.
// removed func

// VerifyRotationEpoch verifies if rotation can be performed in the given epoch.
// removed func

// Backend is a key manager management implementation.
// removed interface

// NewUpdatePolicyTx creates a new policy update transaction.
// removed func

// NewPublishMasterSecretTx creates a new publish master secret transaction.
// removed func

// NewPublishEphemeralSecretTx creates a new publish ephemeral secret transaction.
// removed func

// InitRequest is the initialization RPC request, sent to the key manager
// enclave.
type InitRequest struct {
	Status      *Status `json:"status,omitempty"`       // TODO: Change in PR-5205.
	Checksum    []byte  `json:"checksum,omitempty"`     // TODO: Remove in PR-5205.
	Policy      []byte  `json:"policy,omitempty"`       // TODO: Remove in PR-5205.
	MayGenerate bool    `json:"may_generate,omitempty"` // TODO: Remove in PR-5205.
}

// InitResponse is the initialization RPC response, returned as part of a
// SignedInitResponse from the key manager enclave.
type InitResponse struct {
	IsSecure       bool                 `json:"is_secure"`
	Checksum       []byte               `json:"checksum"`
	NextChecksum   []byte               `json:"next_checksum,omitempty"`
	PolicyChecksum []byte               `json:"policy_checksum"`
	RSK            *signature.PublicKey `json:"rsk,omitempty"`
	NextRSK        *signature.PublicKey `json:"next_rsk,omitempty"`
}

// SignedInitResponse is the signed initialization RPC response, returned
// from the key manager enclave.
type SignedInitResponse struct {
	InitResponse InitResponse `json:"init_response"`
	Signature    []byte       `json:"signature"`
}

// Verify verifies the signature of the init response using the given key.
// removed func

// SignInitResponse signs the given init response.
// removed func

// LongTermKeyRequest is the long-term key RPC request, sent to the key manager
// enclave.
type LongTermKeyRequest struct {
	Height     *uint64          `json:"height"`
	ID         common.Namespace `json:"runtime_id"`
	KeyPairID  KeyPairID        `json:"key_pair_id"`
	Generation uint64           `json:"generation"`
}

// EphemeralKeyRequest is the ephemeral key RPC request, sent to the key manager
// enclave.
type EphemeralKeyRequest struct {
	Height    *uint64          `json:"height"`
	ID        common.Namespace `json:"runtime_id"`
	KeyPairID KeyPairID        `json:"key_pair_id"`
	Epoch     beacon.EpochTime `json:"epoch"`
}

// SignedPublicKey is the RPC response, returned as part of
// an EphemeralKeyRequest from the key manager enclave.
type SignedPublicKey struct {
	Key        x25519.PublicKey       `json:"key"`
	Checksum   []byte                 `json:"checksum"`
	Signature  signature.RawSignature `json:"signature"`
	Expiration *beacon.EpochTime      `json:"expiration,omitempty"`
}

// GenerateMasterSecretRequest is the generate master secret RPC request,
// sent to the key manager enclave.
type GenerateMasterSecretRequest struct {
	Generation uint64           `json:"generation"`
	Epoch      beacon.EpochTime `json:"epoch"`
}

// GenerateMasterSecretResponse is the RPC response, returned as part of
// a GenerateMasterSecretRequest from the key manager enclave.
type GenerateMasterSecretResponse struct {
	SignedSecret SignedEncryptedMasterSecret `json:"signed_secret"`
}

// GenerateEphemeralSecretRequest is the generate ephemeral secret RPC request,
// sent to the key manager enclave.
type GenerateEphemeralSecretRequest struct {
	Epoch beacon.EpochTime `json:"epoch"`
}

// GenerateEphemeralSecretResponse is the RPC response, returned as part of
// a GenerateEphemeralSecretRequest from the key manager enclave.
type GenerateEphemeralSecretResponse struct {
	SignedSecret SignedEncryptedEphemeralSecret `json:"signed_secret"`
}

// LoadMasterSecretRequest is the load master secret RPC request,
// sent to the key manager enclave.
type LoadMasterSecretRequest struct {
	SignedSecret SignedEncryptedMasterSecret `json:"signed_secret"`
}

// LoadEphemeralSecretRequest is the load ephemeral secret RPC request,
// sent to the key manager enclave.
type LoadEphemeralSecretRequest struct {
	SignedSecret SignedEncryptedEphemeralSecret `json:"signed_secret"`
}

// VerifyExtraInfo verifies and parses the per-node + per-runtime ExtraInfo
// blob for a key manager.
// removed func

// Genesis is the key manager management genesis state.
type Genesis struct {
	// Parameters are the key manager consensus parameters.
	Parameters ConsensusParameters `json:"params"`

	Statuses []*Status `json:"statuses,omitempty"`
}

// ConsensusParameters are the key manager consensus parameters.
type ConsensusParameters struct {
	GasCosts transaction.Costs `json:"gas_costs,omitempty"`
}

// ConsensusParameterChanges are allowed key manager consensus parameter changes.
type ConsensusParameterChanges struct {
	// GasCosts are the new gas costs.
	GasCosts transaction.Costs `json:"gas_costs,omitempty"`
}

// Apply applies changes to the given consensus parameters.
// removed func

// StatusUpdateEvent is the keymanager status update event.
type StatusUpdateEvent struct {
	Statuses []*Status
}

// EventKind returns a string representation of this event's kind.
// removed func

// MasterSecretPublishedEvent is the key manager master secret published event.
type MasterSecretPublishedEvent struct {
	Secret *SignedEncryptedMasterSecret
}

// EventKind returns a string representation of this event's kind.
// removed func

// EphemeralSecretPublishedEvent is the key manager ephemeral secret published event.
type EphemeralSecretPublishedEvent struct {
	Secret *SignedEncryptedEphemeralSecret
}

// EventKind returns a string representation of this event's kind.
// removed func

// removed func
