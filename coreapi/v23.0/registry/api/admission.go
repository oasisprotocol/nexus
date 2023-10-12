package api

import (
	"github.com/oasisprotocol/oasis-core/go/common/crypto/signature"

	"github.com/oasisprotocol/nexus/coreapi/v23.0/common/node"
)

// RuntimeAdmissionPolicy is a specification of which nodes are allowed to register for a runtime.
type RuntimeAdmissionPolicy struct {
	AnyNode         *AnyNodeRuntimeAdmissionPolicy         `json:"any_node,omitempty"`
	EntityWhitelist *EntityWhitelistRuntimeAdmissionPolicy `json:"entity_whitelist,omitempty"`

	// PerRole is a per-role admission policy that must be satisfied in addition to the global
	// admission policy for a specific role.
	PerRole map[node.RolesMask]PerRoleAdmissionPolicy `json:"per_role,omitempty"`
}

// ValidateBasic performs basic runtime admission policy validity checks.
// removed func

// Verify ensures the runtime admission policy is satisfied, returning an error otherwise.
// removed func

// AnyNodeRuntimeAdmissionPolicy allows any node to register.
type AnyNodeRuntimeAdmissionPolicy struct{}

// EntityWhitelistRuntimeAdmissionPolicy allows only whitelisted entities' nodes to register.
type EntityWhitelistRuntimeAdmissionPolicy struct {
	Entities map[signature.PublicKey]EntityWhitelistConfig `json:"entities"`
}

// ValidateBasic performs basic runtime admission policy validity checks.
// removed func

// Verify ensures the runtime admission policy is satisfied, returning an error otherwise.
// removed func

// EntityWhitelistConfig is a per-entity whitelist configuration.
type EntityWhitelistConfig struct {
	// MaxNodes is the maximum number of nodes that an entity can register under
	// the given runtime for a specific role. If the map is empty or absent, the
	// number of nodes is unlimited. If the map is present and non-empty, the
	// the number of nodes is restricted to the specified maximum (where zero
	// means no nodes allowed), any missing roles imply zero nodes.
	MaxNodes map[node.RolesMask]uint16 `json:"max_nodes,omitempty"`
}

// PerRoleAdmissionPolicy is a per-role admission policy.
type PerRoleAdmissionPolicy struct {
	EntityWhitelist *EntityWhitelistRoleAdmissionPolicy `json:"entity_whitelist,omitempty"`
}

// ValidateBasic performs basic runtime admission policy validity checks.
// removed func

// Verify ensures the runtime admission policy is satisfied, returning an error otherwise.
// removed func

// EntityWhitelistRoleAdmissionPolicy is a per-role entity whitelist policy.
type EntityWhitelistRoleAdmissionPolicy struct {
	Entities map[signature.PublicKey]EntityWhitelistRoleConfig `json:"entities"`
}

// ValidateBasic performs basic runtime admission policy validity checks.
// removed func

// Verify ensures the runtime admission policy is satisfied, returning an error otherwise.
// removed func

// EntityWhitelistRoleConfig is a per-entity whitelist configuration for a given role.
type EntityWhitelistRoleConfig struct {
	MaxNodes uint16 `json:"max_nodes,omitempty"`
}

// verifyNodeCountWithRoleForRuntime verifies that the number of nodes registered by the specified
// entity for the specified runtime with the specified role is at most the specified maximum.
// removed func
