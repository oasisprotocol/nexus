package churp

import (
	beacon "github.com/oasisprotocol/nexus/coreapi/v24.0/beacon/api"

	"github.com/oasisprotocol/oasis-core/go/common/crypto/hash"
	"github.com/oasisprotocol/oasis-core/go/common/crypto/signature"
)

// maxThreshold is the maximum threshold.
//
// Limiting the threshold ensures that the dimensions of bivariate polynomials
// (t, 2t) never exceed the range of uint8.
const maxThreshold = 127

// removed var block

// CreateRequest contains the initial configuration.
type CreateRequest struct {
	Identity

	// SuiteID is the identifier of a cipher suite used for verifiable secret
	// sharing and key derivation.
	SuiteID uint8 `json:"suite_id,omitempty"`

	// Threshold is the minimum number of distinct shares required
	// to reconstruct a key.
	Threshold uint8 `json:"threshold,omitempty"`

	// ExtraShares represents the minimum number of shares that can be lost
	// to render the secret unrecoverable.
	ExtraShares uint8 `json:"extra_shares,omitempty"`

	// HandoffInterval is the time interval in epochs between handoffs.
	//
	// A zero value disables handoffs.
	HandoffInterval beacon.EpochTime `json:"handoff_interval,omitempty"`

	// Policy is a signed SGX access control policy.
	Policy SignedPolicySGX `json:"policy,omitempty"`
}

// ValidateBasic performs basic config validity checks.
// removed func

// UpdateRequest contains the updated configuration.
type UpdateRequest struct {
	Identity

	// ExtraShares represents the minimum number of shares that can be lost
	// to render the secret unrecoverable.
	ExtraShares *uint8 `json:"extra_shares,omitempty"`

	// HandoffInterval is the time interval in epochs between handoffs.
	//
	// Zero value disables handoffs.
	HandoffInterval *beacon.EpochTime `json:"handoff_interval,omitempty"`

	// Policy is a signed SGX access control policy.
	Policy *SignedPolicySGX `json:"policy,omitempty"`
}

// ValidateBasic performs basic config validity checks.
// removed func

// ApplicationRequest contains node's application to form a new committee.
type ApplicationRequest struct {
	// Identity of the CHRUP scheme.
	Identity

	// Epoch is the epoch of the handoff for which the node would like
	// to register.
	Epoch beacon.EpochTime `json:"epoch"`

	// Checksum is the hash of the verification matrix.
	Checksum hash.Hash `json:"checksum"`
}

// SignedApplicationRequest is an application request signed by the key manager
// enclave using its runtime attestation key (RAK).
type SignedApplicationRequest struct {
	Application ApplicationRequest `json:"application"`

	// Signature is the RAK signature of the application request.
	Signature signature.RawSignature `json:"signature"`
}

// VerifyRAK verifies the runtime attestation key (RAK) signature.
// removed func

// ConfirmationRequest confirms that the node successfully completed
// the handoff.
type ConfirmationRequest struct {
	Identity

	// Epoch is the epoch of the handoff for which the node reconstructed
	// the share.
	Epoch beacon.EpochTime `json:"epoch"`

	// Checksum is the hash of the verification matrix.
	Checksum hash.Hash `json:"checksum"`
}

// SignedConfirmationRequest is a confirmation request signed by the key manager
// enclave using its runtime attestation key (RAK).
type SignedConfirmationRequest struct {
	Confirmation ConfirmationRequest `json:"confirmation"`

	// Signature is the RAK signature of the confirmation request.
	Signature signature.RawSignature `json:"signature"`
}

// VerifyRAK verifies the runtime attestation key (RAK) signature.
// removed func
