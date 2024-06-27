package churp

import (
	"github.com/oasisprotocol/oasis-core/go/common/crypto/signature"
	"github.com/oasisprotocol/oasis-core/go/common/sgx"
)

// PolicySGXSignatureContext is the context used to sign PolicySGX documents.
// removed var statement

// PolicySGX represents an SGX access control policy used to authenticate
// key manager enclaves during handoffs.
type PolicySGX struct {
	Identity

	// Serial is the monotonically increasing policy serial number.
	Serial uint32 `json:"serial"`

	// MayShare is the vector of enclave identities from which a share can be
	// obtained during handouts.
	MayShare []sgx.EnclaveIdentity `json:"may_share"`

	// MayJoin is the vector of enclave identities that may form the new
	// committee in the next handoffs.
	MayJoin []sgx.EnclaveIdentity `json:"may_join"`
}

// SanityCheck verifies the validity of the policy.
// removed func

// SignedPolicySGX represents a signed SGX access control policy.
//
// The runtime extension will accept the policy only if all signatures are
// valid, and a sufficient number of trusted policy signers have signed it.
type SignedPolicySGX struct {
	// Policy is an SGX access control policy.
	Policy PolicySGX `json:"policy"`

	// Signatures is a vector of signatures.
	Signatures []signature.Signature `json:"signatures,omitempty"`
}

// SanityCheck verifies the validity of the policy and the signatures.
// removed func

// Sign signs the policy with the given signer and appends the signatures.
// removed func
