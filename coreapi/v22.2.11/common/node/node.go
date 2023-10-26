// Package node implements common node identity routines.
package node

import (
	"fmt"
	"strings"

	"github.com/oasisprotocol/oasis-core/go/common"
	"github.com/oasisprotocol/oasis-core/go/common/cbor"
	"github.com/oasisprotocol/oasis-core/go/common/crypto/signature"
	"github.com/oasisprotocol/oasis-core/go/common/version"
)

// removed var block

const (
	// LatestNodeDescriptorVersion is the latest node descriptor version that should be used for all
	// new descriptors. Using earlier versions may be rejected.
	LatestNodeDescriptorVersion = 2

	// Minimum and maximum descriptor versions that are allowed.
	minNodeDescriptorVersion = 1
	maxNodeDescriptorVersion = LatestNodeDescriptorVersion
)

// Node represents public connectivity information about an Oasis node.
type Node struct { // nolint: maligned
	cbor.Versioned

	// ID is the public key identifying the node.
	ID signature.PublicKey `json:"id"`

	// EntityID is the public key identifying the Entity controlling
	// the node.
	EntityID signature.PublicKey `json:"entity_id"`

	// Expiration is the epoch in which this node's commitment expires.
	Expiration uint64 `json:"expiration"`

	// TLS contains information for connecting to this node via TLS.
	TLS TLSInfo `json:"tls"`

	// P2P contains information for connecting to this node via P2P.
	P2P P2PInfo `json:"p2p"`

	// Consensus contains information for connecting to this node as a
	// consensus member.
	Consensus ConsensusInfo `json:"consensus"`

	// VRF contains information for this node's participation in VRF
	// based elections.
	VRF *VRFInfo `json:"vrf,omitempty"`

	// DeprecatedBeacon contains information for this node's
	// participation in the old PVSS based random beacon protocol.
	DeprecatedBeacon cbor.RawMessage `json:"beacon,omitempty"`

	// Runtimes are the node's runtimes.
	Runtimes []*Runtime `json:"runtimes"`

	// Roles is a bitmask representing the node roles.
	Roles RolesMask `json:"roles"`

	// SoftwareVersion is the node's oasis-node software version.
	SoftwareVersion string `json:"software_version,omitempty"`
}

// RolesMask is Oasis node roles bitmask.
type RolesMask uint32

const (
	// RoleComputeWorker is the compute worker role.
	RoleComputeWorker RolesMask = 1 << 0
	// roleReserved2 is the reserved role (storage role in v1 descriptors).
	roleReserved2 RolesMask = 1 << 1
	// RoleKeyManager is the the key manager role.
	RoleKeyManager RolesMask = 1 << 2
	// RoleValidator is the validator role.
	RoleValidator RolesMask = 1 << 3
	// RoleConsensusRPC is the public consensus RPC services worker role.
	RoleConsensusRPC RolesMask = 1 << 4
	// RoleStorageRPC is the public storage RPC services worker role.
	RoleStorageRPC RolesMask = 1 << 5

	// RoleReserved are all the bits of the Oasis node roles bitmask
	// that are reserved and must not be used.
	RoleReserved RolesMask = ((1<<32)-1) & ^((RoleStorageRPC<<1)-1) | roleReserved2

	// Human friendly role names:

	RoleComputeWorkerName = "compute"
	RoleKeyManagerName    = "key-manager"
	RoleValidatorName     = "validator"
	RoleConsensusRPCName  = "consensus-rpc"
	RoleStorageRPCName    = "storage-rpc"

	rolesMaskStringSep = ","
)

// Roles returns a list of available valid roles.
// removed func

// IsSingleRole returns true if RolesMask encodes a single valid role.
// removed func

func (m RolesMask) String() string {
	if m&RoleReserved != 0 {
		return "[invalid roles]"
	}

	var ret []string
	if m&RoleComputeWorker != 0 {
		ret = append(ret, RoleComputeWorkerName)
	}
	if m&RoleKeyManager != 0 {
		ret = append(ret, RoleKeyManagerName)
	}
	if m&RoleValidator != 0 {
		ret = append(ret, RoleValidatorName)
	}
	if m&RoleConsensusRPC != 0 {
		ret = append(ret, RoleConsensusRPCName)
	}
	if m&RoleStorageRPC != 0 {
		ret = append(ret, RoleStorageRPCName)
	}

	return strings.Join(ret, rolesMaskStringSep)
}

// MarshalText encodes a RolesMask into text form.
func (m RolesMask) MarshalText() ([]byte, error) {
	return []byte(m.String()), nil
}

func checkDuplicateRole(newRole RolesMask, curRoles RolesMask) error {
	if curRoles&newRole != 0 {
		return fmt.Errorf("node: duplicate role: '%s'", newRole)
	}
	return nil
}

// UnmarshalText decodes a text slice into a RolesMask.
func (m *RolesMask) UnmarshalText(text []byte) error {
	*m = 0
	roles := strings.Split(string(text), rolesMaskStringSep)
	for _, role := range roles {
		switch role {
		case RoleComputeWorkerName:
			if err := checkDuplicateRole(RoleComputeWorker, *m); err != nil {
				return err
			}
			*m |= RoleComputeWorker
		case RoleKeyManagerName:
			if err := checkDuplicateRole(RoleKeyManager, *m); err != nil {
				return err
			}
			*m |= RoleKeyManager
		case RoleValidatorName:
			if err := checkDuplicateRole(RoleValidator, *m); err != nil {
				return err
			}
			*m |= RoleValidator
		case RoleConsensusRPCName:
			if err := checkDuplicateRole(RoleConsensusRPC, *m); err != nil {
				return err
			}
			*m |= RoleConsensusRPC
		case RoleStorageRPCName:
			if err := checkDuplicateRole(RoleStorageRPC, *m); err != nil {
				return err
			}
			*m |= RoleStorageRPC
		default:
			return fmt.Errorf("node: invalid role: '%s'", role)
		}
	}
	return nil
}

// UnmarshalCBOR is a custom deserializer that handles both v1 and v2 Node structures.
func (n *Node) UnmarshalCBOR(data []byte) error {
	// Determine Entity structure version.
	v, err := cbor.GetVersion(data)
	if err != nil {
		return err
	}
	switch v {
	case 1:
		// Old version had an extra supported role (the storage role).
		type nv2 Node
		if err := cbor.Unmarshal(data, (*nv2)(n)); err != nil {
			return err
		}

		// Convert into new format.
		n.Versioned = cbor.NewVersioned(2)
		n.Roles = n.Roles & ^roleReserved2 // Clear old storage role.
		return nil
	case 2:
		// New version, call the default unmarshaler.
		type nv2 Node
		return cbor.Unmarshal(data, (*nv2)(n))
	default:
		return fmt.Errorf("invalid node descriptor version: %d", v)
	}
}

// ValidateBasic performs basic descriptor validity checks.
// removed func

// AddRoles adds a new node role to the existing roles mask.
// removed func

// HasRoles checks if the node has the specified roles.
// removed func

// OnlyHasRoles checks if the node only has the specified roles and no others.
// removed func

// IsExpired returns true if the node expiration epoch is strictly smaller
// than the passed (current) epoch.
// removed func

// HasRuntime returns true iff the node supports a runtime (ignoring version).
// removed func

// GetRuntime searches for an existing supported runtime descriptor
// in Runtimes with the specified version and returns it.
// removed func

// AddOrUpdateRuntime searches for an existing supported runtime descriptor
// in Runtimes with the specified version and returns it. In case a
// runtime descriptor for the given runtime and version doesn't exist yet,
// a new one is created appended to the list of supported runtimes and
// returned.
// removed func

// Runtime represents the runtimes supported by a given Oasis node.
type Runtime struct {
	// ID is the public key identifying the runtime.
	ID common.Namespace `json:"id"`

	// Version is the version of the runtime.
	Version version.Version `json:"version"`

	// Capabilities are the node's capabilities for a given runtime.
	Capabilities Capabilities `json:"capabilities"`

	// ExtraInfo is the extra per node + per runtime opaque data associated
	// with the current instance.
	ExtraInfo []byte `json:"extra_info"`
}

// TLSInfo contains information for connecting to this node via TLS.
type TLSInfo struct {
	// PubKey is the public key used for establishing TLS connections.
	PubKey signature.PublicKey `json:"pub_key"`

	// NextPubKey is the public key that will be used for establishing TLS connections after
	// certificate rotation (if enabled).
	NextPubKey signature.PublicKey `json:"next_pub_key,omitempty"`

	// Addresses is the list of addresses at which the node can be reached.
	Addresses []TLSAddress `json:"addresses"`
}

// Equal compares vs another TLSInfo for equality.
// removed func

// P2PInfo contains information for connecting to this node via P2P transport.
type P2PInfo struct {
	// ID is the unique identifier of the node on the P2P transport.
	ID signature.PublicKey `json:"id"`

	// Addresses is the list of addresses at which the node can be reached.
	Addresses []Address `json:"addresses"`
}

// ConsensusInfo contains information for connecting to this node as a
// consensus member.
type ConsensusInfo struct {
	// ID is the unique identifier of the node as a consensus member.
	ID signature.PublicKey `json:"id"`

	// Addresses is the list of addresses at which the node can be reached.
	Addresses []ConsensusAddress `json:"addresses"`
}

// VRFInfo contains information for this node's participation in
// VRF based elections.
type VRFInfo struct {
	// ID is the unique identifier of the node used to generate VRF proofs.
	ID signature.PublicKey `json:"id"`
}

// Capabilities represents a node's capabilities.
type Capabilities struct {
	// TEE is the capability of a node executing batches in a TEE.
	TEE *CapabilityTEE `json:"tee,omitempty"`
}

// TEEHardware is a TEE hardware implementation.
type TEEHardware uint8

// TEE Hardware implementations.
const (
	// TEEHardwareInvalid is a non-TEE implementation.
	TEEHardwareInvalid TEEHardware = 0
	// TEEHardwareIntelSGX is an Intel SGX TEE implementation.
	TEEHardwareIntelSGX TEEHardware = 1

	// TEEHardwareReserved is the first reserved hardware implementation
	// identifier. All equal or greater identifiers are reserved.
	TEEHardwareReserved TEEHardware = TEEHardwareIntelSGX + 1

	teeInvalid  = "invalid"
	teeIntelSGX = "intel-sgx"
)

// String returns the string representation of a TEEHardware.
func (h TEEHardware) String() string {
	switch h {
	case TEEHardwareInvalid:
		return teeInvalid
	case TEEHardwareIntelSGX:
		return teeIntelSGX
	default:
		return "[unsupported TEEHardware]"
	}
}

// FromString deserializes a string into a TEEHardware.
// removed func

// CapabilityTEE represents the node's TEE capability.
type CapabilityTEE struct {
	// TEE hardware type.
	Hardware TEEHardware `json:"hardware"`

	// Runtime attestation key.
	RAK signature.PublicKey `json:"rak"`

	// Attestation.
	Attestation []byte `json:"attestation"`
}

// HashRAK computes the expected report data hash bound to a given public RAK.
// removed func

// Verify verifies the node's TEE capabilities, at the provided timestamp and height.
// removed func

// String returns a string representation of itself.
func (n *Node) String() string {
	return "<Node id=" + n.ID.String() + ">"
}

// MultiSignedNode is a multi-signed blob containing a CBOR-serialized Node.
type MultiSignedNode struct {
	signature.MultiSigned
}

// Open first verifies the blob signatures and then unmarshals the blob.
// removed func

// PrettyPrint writes a pretty-printed representation of the type
// to the given writer.
// removed func

// PrettyType returns a representation of the type that can be used for pretty printing.
// removed func

// MultiSignNode serializes the Node and multi-signs the result.
// removed func
