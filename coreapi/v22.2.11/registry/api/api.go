// Package api implements the runtime and entity registry APIs.
package api

import (
	"github.com/oasisprotocol/nexus/coreapi/v22.2.11/common/node"
	"github.com/oasisprotocol/nexus/coreapi/v22.2.11/consensus/api/transaction"
	"github.com/oasisprotocol/oasis-core/go/common"
	"github.com/oasisprotocol/oasis-core/go/common/crypto/hash"
	"github.com/oasisprotocol/oasis-core/go/common/crypto/signature"
	"github.com/oasisprotocol/oasis-core/go/common/entity"
	original "github.com/oasisprotocol/oasis-core/go/registry/api"
)

// ModuleName is a unique module name for the registry module.
const ModuleName = "registry"

var (
	// RegisterEntitySignatureContext is the context used for entity
	// registration.
	RegisterEntitySignatureContext = original.RegisterEntitySignatureContext

	// RegisterGenesisEntitySignatureContext is the context used for
	// entity registration in the genesis document.
	//
	// Note: This is identical to non-gensis registrations to support
	// migrating existing registrations into a new genesis document.
	RegisterGenesisEntitySignatureContext = RegisterEntitySignatureContext

	// RegisterNodeSignatureContext is the context used for node
	// registration.
	RegisterNodeSignatureContext = original.RegisterNodeSignatureContext
)

// Backend is a registry implementation.
// removed interface

// IDQuery is a registry query by ID.
type IDQuery struct {
	Height int64               `json:"height"`
	ID     signature.PublicKey `json:"id"`
}

// NamespaceQuery is a registry query by namespace (Runtime ID).
type NamespaceQuery struct {
	Height int64            `json:"height"`
	ID     common.Namespace `json:"id"`
}

// GetRuntimeQuery is a registry query by namespace (Runtime ID).
type GetRuntimeQuery struct {
	Height           int64            `json:"height"`
	ID               common.Namespace `json:"id"`
	IncludeSuspended bool             `json:"include_suspended,omitempty"`
}

// GetRuntimesQuery is a registry get runtimes query.
type GetRuntimesQuery struct {
	Height           int64 `json:"height"`
	IncludeSuspended bool  `json:"include_suspended"`
}

// ConsensusAddressQuery is a registry query by consensus address.
// The nature and format of the consensus address depends on the specific
// consensus backend implementation used.
type ConsensusAddressQuery struct {
	Height  int64  `json:"height"`
	Address []byte `json:"address"`
}

// DeregisterEntity is a request to deregister an entity.
type DeregisterEntity struct{}

// NewRegisterEntityTx creates a new register entity transaction.
// removed func

// NewDeregisterEntityTx creates a new deregister entity transaction.
// removed func

// NewRegisterNodeTx creates a new register node transaction.
// removed func

// NewUnfreezeNodeTx creates a new unfreeze node transaction.
// removed func

// NewRegisterRuntimeTx creates a new register runtime transaction.
// removed func

// NewProveFreshnessTx creates a new prove freshness transaction.
// removed func

// EntityEvent is the event that is returned via WatchEntities to signify
// entity registration changes and updates.
type EntityEvent struct {
	Entity         *entity.Entity `json:"entity"`
	IsRegistration bool           `json:"is_registration"`
}

// EventKind returns a string representation of this event's kind.
// removed func

// NodeEvent is the event that is returned via WatchNodes to signify node
// registration changes and updates.
type NodeEvent struct {
	Node           *node.Node `json:"node"`
	IsRegistration bool       `json:"is_registration"`
}

// EventKind returns a string representation of this event's kind.
// removed func

// RuntimeEvent signifies new runtime registration.
type RuntimeEvent struct {
	Runtime *Runtime `json:"runtime"`
}

// EventKind returns a string representation of this event's kind.
// removed func

// NodeUnfrozenEvent signifies when node becomes unfrozen.
type NodeUnfrozenEvent struct {
	NodeID signature.PublicKey `json:"node_id"`
}

// EventKind returns a string representation of this event's kind.
// removed func

// removed var statement

// NodeListEpochEvent is the per epoch node list event.
type NodeListEpochEvent struct{}

// EventKind returns a string representation of this event's kind.
// removed func

// EventValue returns a string representation of this event's kind.
// removed func

// DecodeValue decodes the attribute event value.
// removed func

// Event is a registry event returned via GetEvents.
type Event struct {
	Height int64     `json:"height,omitempty"`
	TxHash hash.Hash `json:"tx_hash,omitempty"`

	RuntimeEvent      *RuntimeEvent      `json:"runtime,omitempty"`
	EntityEvent       *EntityEvent       `json:"entity,omitempty"`
	NodeEvent         *NodeEvent         `json:"node,omitempty"`
	NodeUnfrozenEvent *NodeUnfrozenEvent `json:"node_unfrozen,omitempty"`
}

// NodeList is a per-epoch immutable node list.
type NodeList struct {
	Nodes []*node.Node `json:"nodes"`
}

// NodeLookup interface implements various ways for the verification
// functions to look-up nodes in the registry's state.
// removed interface

// RuntimeLookup interface implements various ways for the verification
// functions to look-up runtimes in the registry's state.
// removed interface

// VerifyRegisterEntityArgs verifies arguments for RegisterEntity.
// removed func

// VerifyRegisterNodeArgs verifies arguments for RegisterNode.
//
// Returns the node descriptor and a list of runtime descriptors the node is registering for.
// removed func

// VerifyNodeRuntimeEnclaveIDs verifies TEE-specific attributes of the node's runtime.
// removed func

// VerifyAddress verifies a node address.
// removed func

// removed func

// verifyNodeRuntimeChanges verifies node runtime changes.
// removed func

// verifyRuntimeCapabilities verifies node runtime capabilities changes.
// removed func

// VerifyNodeUpdate verifies changes while updating the node.
// removed func

// VerifyRuntime verifies the given runtime.
// removed func

// VerifyRegisterComputeRuntimeArgs verifies compute runtime-specific arguments for RegisterRuntime.
// removed func

// VerifyRuntimeNew verifies a new runtime.
// removed func

// VerifyRuntimeUpdate verifies changes while updating the runtime.
// removed func

// SortNodeList sorts the given node list to ensure a canonical order.
// removed func

// Genesis is the registry genesis state.
type Genesis struct {
	// Parameters are the registry consensus parameters.
	Parameters ConsensusParameters `json:"params"`

	// Entities is the initial list of entities.
	Entities []*entity.SignedEntity `json:"entities,omitempty"`

	// Runtimes is the initial list of runtimes.
	Runtimes []*Runtime `json:"runtimes,omitempty"`
	// SuspendedRuntimes is the list of suspended runtimes.
	SuspendedRuntimes []*Runtime `json:"suspended_runtimes,omitempty"`

	// Nodes is the initial list of nodes.
	Nodes []*node.MultiSignedNode `json:"nodes,omitempty"`

	// NodeStatuses is a set of node statuses.
	NodeStatuses map[signature.PublicKey]*NodeStatus `json:"node_statuses,omitempty"`
}

// ConsensusParameters are the registry consensus parameters.
type ConsensusParameters struct {
	// DebugAllowUnroutableAddresses is true iff node registration should
	// allow unroutable addresses.
	DebugAllowUnroutableAddresses bool `json:"debug_allow_unroutable_addresses,omitempty"`

	// DebugAllowTestRuntimes is true iff test runtimes should be allowed to
	// be registered.
	DebugAllowTestRuntimes bool `json:"debug_allow_test_runtimes,omitempty"`

	// DebugBypassStake is true iff the registry should bypass all of the staking
	// related checks and operations.
	DebugBypassStake bool `json:"debug_bypass_stake,omitempty"`

	// DebugDeployImmediately is true iff runtime registrations should
	// allow immediate deployment.
	DebugDeployImmediately bool `json:"debug_deploy_immediately,omitempty"`

	// DisableRuntimeRegistration is true iff runtime registration should be
	// disabled outside of the genesis block.
	DisableRuntimeRegistration bool `json:"disable_runtime_registration,omitempty"`

	// DisableKeyManagerRuntimeRegistration is true iff key manager runtime registration should be
	// disabled outside of the genesis block.
	DisableKeyManagerRuntimeRegistration bool `json:"disable_km_runtime_registration,omitempty"`

	// GasCosts are the registry transaction gas costs.
	GasCosts transaction.Costs `json:"gas_costs,omitempty"`

	// MaxNodeExpiration is the maximum number of epochs relative to the epoch
	// at registration time that a single node registration is valid for.
	MaxNodeExpiration uint64 `json:"max_node_expiration,omitempty"`

	// EnableRuntimeGovernanceModels is a set of enabled runtime governance models.
	EnableRuntimeGovernanceModels map[RuntimeGovernanceModel]bool `json:"enable_runtime_governance_models,omitempty"`

	// TEEFeatures contains the configuration of supported TEE features.
	TEEFeatures *node.TEEFeatures `json:"tee_features,omitempty"`

	// MaxRuntimeDeployments is the maximum number of runtime deployments.
	MaxRuntimeDeployments uint8 `json:"max_runtime_deployments,omitempty"`
}

// ConsensusParameterChanges are allowed registry consensus parameter changes.
type ConsensusParameterChanges struct {
	// DisableRuntimeRegistration is the new disable runtime registration flag.
	DisableRuntimeRegistration *bool `json:"disable_runtime_registration,omitempty"`

	// DisableKeyManagerRuntimeRegistration the new disable key manager runtime registration flag.
	DisableKeyManagerRuntimeRegistration *bool `json:"disable_km_runtime_registration,omitempty"`

	// GasCosts are the new gas costs.
	GasCosts transaction.Costs `json:"gas_costs,omitempty"`

	// MaxNodeExpiration is the maximum node expiration.
	MaxNodeExpiration *uint64 `json:"max_node_expiration,omitempty"`

	// EnableRuntimeGovernanceModels are the new enabled runtime governance models.
	EnableRuntimeGovernanceModels map[RuntimeGovernanceModel]bool `json:"enable_runtime_governance_models,omitempty"`

	// TEEFeatures are the new TEE features.
	TEEFeatures **node.TEEFeatures `json:"tee_features,omitempty"`

	// MaxRuntimeDeployments is the new maximum number of runtime deployments.
	MaxRuntimeDeployments *uint8 `json:"max_runtime_deployments,omitempty"`
}

// Apply applies changes to the given consensus parameters.
// removed func

const (
	// GasOpRegisterEntity is the gas operation identifier for entity registration.
	GasOpRegisterEntity transaction.Op = "register_entity"
	// GasOpDeregisterEntity is the gas operation identifier for entity deregistration.
	GasOpDeregisterEntity transaction.Op = "deregister_entity"
	// GasOpRegisterNode is the gas operation identifier for entity registration.
	GasOpRegisterNode transaction.Op = "register_node"
	// GasOpUnfreezeNode is the gas operation identifier for unfreezing nodes.
	GasOpUnfreezeNode transaction.Op = "unfreeze_node"
	// GasOpRegisterRuntime is the gas operation identifier for runtime registration.
	GasOpRegisterRuntime transaction.Op = "register_runtime"
	// GasOpRuntimeEpochMaintenance is the gas operation identifier for per-epoch
	// runtime maintenance costs.
	GasOpRuntimeEpochMaintenance transaction.Op = "runtime_epoch_maintenance"
	// GasOpUpdateKeyManager is the gas operation identifier for key manager
	// policy updates costs.
	GasOpUpdateKeyManager transaction.Op = "update_keymanager"
	// GasOpProveFreshness is the gas operation identifier for freshness proofs.
	GasOpProveFreshness transaction.Op = "prove_freshness"
)

// XXX: Define reasonable default gas costs.

// DefaultGasCosts are the "default" gas costs for operations.
// removed var statement

const (
	// StakeClaimRegisterEntity is the stake claim identifier used for registering an entity.
	StakeClaimRegisterEntity = "registry.RegisterEntity"
	// StakeClaimRegisterNode is the stake claim template used for registering nodes.
	StakeClaimRegisterNode = "registry.RegisterNode.%s"
	// StakeClaimRegisterRuntime is the stake claim template used for registering runtimes.
	StakeClaimRegisterRuntime = "registry.RegisterRuntime.%s"
)

// StakeClaimForNode generates a new stake claim identifier for a specific node registration.
// removed func

// StakeClaimForRuntime generates a new stake claim for a specific runtime registration.
// removed func

// StakeThresholdsForNode returns the staking thresholds for the given node.
//
// The passed list of runtimes must be unique runtime descriptors for all runtimes that the node is
// registered for.
// removed func

// StakeThresholdsForRuntime returns the staking thresholds for the given runtime.
// removed func
