package churp

import (
	beacon "github.com/oasisprotocol/nexus/coreapi/v24.0/beacon/api"
	"github.com/oasisprotocol/nexus/coreapi/v24.0/consensus/api/transaction"
	"github.com/oasisprotocol/oasis-core/go/common"
	"github.com/oasisprotocol/oasis-core/go/common/crypto/hash"
	"github.com/oasisprotocol/oasis-core/go/common/crypto/signature"
)

const (
	// HandoffsDisabled is the epoch of the next handoff when handoffs
	// are disabled, i.e., when the handoff interval is set to zero.
	HandoffsDisabled = beacon.EpochInvalid
)

const (
	// NistP384Sha3_384 represents the NIST P-384 elliptic curve group with
	// the SHA3-384 hash function used to encode arbitrary-length byte strings
	// to elements of the underlying prime field or elliptic curve points.
	NistP384Sha3_384 uint8 = iota
)

// HandoffKind represents the kind of a handoff.
type HandoffKind int

const (
	// HandoffKindDealingPhase represents the initial setup phase.
	HandoffKindDealingPhase HandoffKind = iota
	// HandoffKindCommitteeUnchanged represents a handoff where the committee
	// doesn't change.
	HandoffKindCommitteeUnchanged
	// HandoffKindCommitteeChanged represents a handoff where the committee
	// changes.
	HandoffKindCommitteeChanged
)

// String returns the string representation of the HandoffKind.
func (h HandoffKind) String() string {
	switch h {
	case HandoffKindDealingPhase:
		return "dealing phase"
	case HandoffKindCommitteeUnchanged:
		return "committee unchanged"
	case HandoffKindCommitteeChanged:
		return "committee changed"
	default:
		return "unknown"
	}
}

// ConsensusParameters are the key manager CHURP consensus parameters.
type ConsensusParameters struct {
	GasCosts transaction.Costs `json:"gas_costs,omitempty"`
}

// Identity uniquely identifies a CHURP instance.
type Identity struct {
	// ID is a unique CHURP identifier within the key manager runtime.
	ID uint8 `json:"id"`

	// RuntimeID is the identifier of the key manager runtime.
	RuntimeID common.Namespace `json:"runtime_id"`
}

// Status represents the current state of a CHURP instance.
type Status struct {
	Identity

	// SuiteID is the identifier of a cipher suite used for verifiable secret
	// sharing and key derivation.
	SuiteID uint8 `json:"suite_id"`

	// Threshold represents the degree of the secret-sharing polynomial.
	//
	// In a (t,n) secret-sharing scheme, where t represents the threshold,
	// any combination of t+1 or more shares can reconstruct the secret,
	// while losing n-t or fewer shares still allows the secret to be
	// recovered.
	Threshold uint8 `json:"threshold"`

	// ExtraShares represents the minimum number of shares that can be lost
	// to render the secret unrecoverable.
	//
	// If t and e represent the threshold and extra shares, respectively,
	// then the minimum size of the committee is t+e+1.
	ExtraShares uint8 `json:"extra_shares"`

	// HandoffInterval is the time interval in epochs between handoffs.
	//
	// A zero value disables handoffs.
	HandoffInterval beacon.EpochTime `json:"handoff_interval"`

	// Policy is a signed SGX access control policy.
	Policy SignedPolicySGX `json:"policy"`

	// Handoff is the epoch of the last successfully completed handoff.
	//
	// The zero value indicates that no handoffs have been completed so far.
	// Note that the first handoff is special and is called the dealer phase,
	// in which nodes do not reshare or randomize shares but instead construct
	// the secret and shares.
	Handoff beacon.EpochTime `json:"handoff"`

	// The hash of the verification matrix from the last successfully completed
	// handoff.
	Checksum *hash.Hash `json:"checksum,omitempty"`

	// Committee is a vector of nodes holding a share of the secret
	// in the active handoff.
	//
	// A client needs to obtain more than a threshold number of key shares
	// from the nodes in this vector to construct the key.
	Committee []signature.PublicKey `json:"committee,omitempty"`

	// NextHandoff defines the epoch in which the next handoff will occur.
	//
	// If an insufficient number of applications is received, the next handoff
	// will be delayed by one epoch.
	NextHandoff beacon.EpochTime `json:"next_handoff"`

	// NextChecksum is the hash of the verification matrix from the current
	// handoff.
	//
	// The first candidate to confirm share reconstruction is the source
	// of truth for the checksum. All other candidates need to confirm
	// with the same checksum; otherwise, the applications will be annulled,
	// and the nodes will need to apply for the new committee again.
	NextChecksum *hash.Hash `json:"next_checksum,omitempty"`

	// Applications is a map of nodes that wish to form the new committee.
	//
	// Candidates are expected to generate a random bivariate polynomial,
	// construct a verification matrix, compute its checksum, and submit
	// an application one epoch in advance of the next scheduled handoff.
	// Subsequently, upon the arrival of the handoff epoch, nodes must execute
	// the handoff protocol and confirm the reconstruction of its share.
	Applications map[signature.PublicKey]Application `json:"applications,omitempty"`
}

// HandoffKind returns the type of the next handoff depending on which nodes
// submitted an application to form the next committee.
// removed func

// MinCommitteeSize returns the minimum number of nodes in the committee.
// removed func

// MinApplicants returns the minimum number of nodes that must participate
// in a handoff.
// removed func

// HandoffsDisabled returns true iff handoffs are disabled, i.e., when
// the handoff interval is set to zero.
// removed func

// Application represents a node's application to form a new committee.
type Application struct {
	// Checksum is the hash of the random verification matrix.
	//
	// In all handoffs, except in the dealer phase, the verification matrix
	// needs to be zero-hole.
	Checksum hash.Hash `json:"checksum"`

	// Reconstructed is true if and only if the node verified all matrices
	// and successfully reconstructed its share during the handoff.
	Reconstructed bool `json:"reconstructed"`
}
