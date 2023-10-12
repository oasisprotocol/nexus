package api

import (
	"fmt"
	"time"

	beacon "github.com/oasisprotocol/nexus/coreapi/v22.2.11/beacon/api"
	scheduler "github.com/oasisprotocol/nexus/coreapi/v22.2.11/scheduler/api"
	staking "github.com/oasisprotocol/nexus/coreapi/v22.2.11/staking/api"
	"github.com/oasisprotocol/oasis-core/go/common"
	"github.com/oasisprotocol/oasis-core/go/common/cbor"
	"github.com/oasisprotocol/oasis-core/go/common/crypto/hash"
	"github.com/oasisprotocol/oasis-core/go/common/crypto/signature"
	"github.com/oasisprotocol/oasis-core/go/common/node"
	"github.com/oasisprotocol/oasis-core/go/common/quantity"
	"github.com/oasisprotocol/oasis-core/go/common/version"
)

// removed var block

// RuntimeKind represents the runtime functionality.
type RuntimeKind uint32

const (
	// KindInvalid is an invalid runtime and should never be explicitly set.
	KindInvalid RuntimeKind = 0

	// KindCompute is a generic compute runtime.
	KindCompute RuntimeKind = 1

	// KindKeyManager is a key manager runtime.
	KindKeyManager RuntimeKind = 2

	kindInvalid    = "invalid"
	kindCompute    = "compute"
	kindKeyManager = "keymanager"
)

// String returns a string representation of a runtime kind.
func (k RuntimeKind) String() string {
	switch k {
	case KindInvalid:
		return kindInvalid
	case KindCompute:
		return kindCompute
	case KindKeyManager:
		return kindKeyManager
	default:
		return "[unsupported runtime kind]"
	}
}

// FromString deserializes a string into a RuntimeKind.
// removed func

// ExecutorParameters are parameters for the executor committee.
type ExecutorParameters struct {
	// GroupSize is the size of the committee.
	GroupSize uint16 `json:"group_size"`

	// GroupBackupSize is the size of the discrepancy resolution group.
	GroupBackupSize uint16 `json:"group_backup_size"`

	// AllowedStragglers is the number of allowed stragglers.
	AllowedStragglers uint16 `json:"allowed_stragglers"`

	// RoundTimeout is the round timeout in consensus blocks.
	RoundTimeout int64 `json:"round_timeout"`

	// MaxMessages is the maximum number of messages that can be emitted by the runtime in a
	// single round.
	MaxMessages uint32 `json:"max_messages"`

	// MinLiveRoundsPercent is the minimum percentage of rounds in an epoch that a node must
	// participate in positively in order to be considered live. Nodes not satisfying this may be
	// penalized.
	MinLiveRoundsPercent uint8 `json:"min_live_rounds_percent,omitempty"`

	// MinLiveRoundsForEvaluation is the minimum number of live rounds in an epoch for the liveness
	// calculations to be considered for evaluation.
	MinLiveRoundsForEvaluation uint64 `json:"min_live_rounds_eval,omitempty"`

	// MaxLivenessFailures is the maximum number of liveness failures that are tolerated before
	// suspending and/or slashing the node. Zero means unlimited.
	MaxLivenessFailures uint8 `json:"max_liveness_fails,omitempty"`
}

// ValidateBasic performs basic executor parameter validity checks.
// removed func

// TxnSchedulerParameters are parameters for the runtime transaction scheduler.
type TxnSchedulerParameters struct {
	// BatchFlushTimeout denotes, if using the "simple" algorithm, how long to
	// wait for a scheduled batch.
	BatchFlushTimeout time.Duration `json:"batch_flush_timeout"`

	// MaxBatchSize denotes what is the max size of a scheduled batch.
	MaxBatchSize uint64 `json:"max_batch_size"`

	// MaxBatchSizeBytes denote what is the max size of a scheduled batch in bytes.
	MaxBatchSizeBytes uint64 `json:"max_batch_size_bytes"`

	// MaxInMessages specifies the maximum size of the incoming message queue.
	MaxInMessages uint32 `json:"max_in_messages,omitempty"`

	// ProposerTimeout denotes the timeout (in consensus blocks) for scheduler
	// to propose a batch.
	ProposerTimeout int64 `json:"propose_batch_timeout"`
}

// ValidateBasic performs basic transaction scheduler parameter validity checks.
// removed func

// StorageParameters are parameters for the storage committee.
type StorageParameters struct {
	// CheckpointInterval is the expected runtime state checkpoint interval (in rounds).
	CheckpointInterval uint64 `json:"checkpoint_interval"`

	// CheckpointNumKept is the expected minimum number of checkpoints to keep.
	CheckpointNumKept uint64 `json:"checkpoint_num_kept"`

	// CheckpointChunkSize is the chunk size parameter for checkpoint creation.
	CheckpointChunkSize uint64 `json:"checkpoint_chunk_size"`
}

// ValidateBasic performs basic storage parameter validity checks.
// removed func

// AnyNodeRuntimeAdmissionPolicy allows any node to register.
type AnyNodeRuntimeAdmissionPolicy struct{}

// EntityWhitelistRuntimeAdmissionPolicy allows only whitelisted entities' nodes to register.
type EntityWhitelistRuntimeAdmissionPolicy struct {
	Entities map[signature.PublicKey]EntityWhitelistConfig `json:"entities"`
}

type EntityWhitelistConfig struct {
	// MaxNodes is the maximum number of nodes that an entity can register under
	// the given runtime for a specific role. If the map is empty or absent, the
	// number of nodes is unlimited. If the map is present and non-empty, the
	// the number of nodes is restricted to the specified maximum (where zero
	// means no nodes allowed), any missing roles imply zero nodes.
	MaxNodes map[node.RolesMask]uint16 `json:"max_nodes,omitempty"`
}

// RuntimeAdmissionPolicy is a specification of which nodes are allowed to register for a runtime.
type RuntimeAdmissionPolicy struct {
	AnyNode         *AnyNodeRuntimeAdmissionPolicy         `json:"any_node,omitempty"`
	EntityWhitelist *EntityWhitelistRuntimeAdmissionPolicy `json:"entity_whitelist,omitempty"`
}

// SchedulingConstraints are the node scheduling constraints.
//
// Multiple fields may be set in which case the ALL the constraints must be satisfied.
type SchedulingConstraints struct {
	ValidatorSet *ValidatorSetConstraint `json:"validator_set,omitempty"`
	MaxNodes     *MaxNodesConstraint     `json:"max_nodes,omitempty"`
	MinPoolSize  *MinPoolSizeConstraint  `json:"min_pool_size,omitempty"`
}

// ValidatorSetConstraint specifies that the entity must have a node that is part of the validator
// set. No other options can currently be specified.
type ValidatorSetConstraint struct{}

// MaxNodesConstraint specifies that only the given number of nodes may be eligible per entity.
type MaxNodesConstraint struct {
	Limit uint16 `json:"limit"`
}

// MinPoolSizeConstraint is the minimum required candidate pool size constraint.
type MinPoolSizeConstraint struct {
	Limit uint16 `json:"limit"`
}

// RuntimeStakingParameters are the stake-related parameters for a runtime.
type RuntimeStakingParameters struct {
	// Thresholds are the minimum stake thresholds for a runtime. These per-runtime thresholds are
	// in addition to the global thresholds. May be left unspecified.
	//
	// In case a node is registered for multiple runtimes, it will need to satisfy the maximum
	// threshold of all the runtimes.
	Thresholds map[staking.ThresholdKind]quantity.Quantity `json:"thresholds,omitempty"`

	// Slashing are the per-runtime misbehavior slashing parameters.
	Slashing map[staking.SlashReason]staking.Slash `json:"slashing,omitempty"`

	// RewardSlashEquvocationRuntimePercent is the percentage of the reward obtained when slashing
	// for equivocation that is transferred to the runtime's account.
	RewardSlashEquvocationRuntimePercent uint8 `json:"reward_equivocation,omitempty"`

	// RewardSlashBadResultsRuntimePercent is the percentage of the reward obtained when slashing
	// for incorrect results that is transferred to the runtime's account.
	RewardSlashBadResultsRuntimePercent uint8 `json:"reward_bad_results,omitempty"`

	// MinInMessageFee specifies the minimum fee that the incoming message must include for the
	// message to be queued.
	MinInMessageFee quantity.Quantity `json:"min_in_message_fee,omitempty"`
}

// ValidateBasic performs basic descriptor validity checks.
// removed func

const (
	// LatestRuntimeDescriptorVersion is the latest entity descriptor version that should be used
	// for all new descriptors. Using earlier versions may be rejected.
	LatestRuntimeDescriptorVersion = 3

	// Minimum and maximum descriptor versions that are allowed.
	minRuntimeDescriptorVersion = 3
	maxRuntimeDescriptorVersion = LatestRuntimeDescriptorVersion
)

// Runtime represents a runtime.
type Runtime struct { // nolint: maligned
	cbor.Versioned

	// ID is a globally unique long term identifier of the runtime.
	ID common.Namespace `json:"id"`

	// EntityID is the public key identifying the Entity controlling
	// the runtime.
	EntityID signature.PublicKey `json:"entity_id"`

	// Genesis is the runtime genesis information.
	Genesis RuntimeGenesis `json:"genesis"`

	// Kind is the type of runtime.
	Kind RuntimeKind `json:"kind"`

	// TEEHardware specifies the runtime's TEE hardware requirements.
	TEEHardware node.TEEHardware `json:"tee_hardware"`

	// KeyManager is the key manager runtime ID for this runtime.
	KeyManager *common.Namespace `json:"key_manager,omitempty"`

	// Executor stores parameters of the executor committee.
	Executor ExecutorParameters `json:"executor,omitempty"`

	// TxnScheduler stores transaction scheduling parameters of the executor
	// committee.
	TxnScheduler TxnSchedulerParameters `json:"txn_scheduler,omitempty"`

	// Storage stores parameters of the storage committee.
	Storage StorageParameters `json:"storage,omitempty"`

	// AdmissionPolicy sets which nodes are allowed to register for this runtime.
	// This policy applies to all roles.
	AdmissionPolicy RuntimeAdmissionPolicy `json:"admission_policy"`

	// Constraints are the node scheduling constraints.
	Constraints map[scheduler.CommitteeKind]map[scheduler.Role]SchedulingConstraints `json:"constraints,omitempty"`

	// Staking stores the runtime's staking-related parameters.
	Staking RuntimeStakingParameters `json:"staking,omitempty"`

	// GovernanceModel specifies the runtime governance model.
	GovernanceModel RuntimeGovernanceModel `json:"governance_model"`

	// Deployments specifies the runtime deployments (versions).
	Deployments []*VersionInfo `json:"deployments,omitempty"`
}

// RuntimeGovernanceModel specifies the runtime governance model.
type RuntimeGovernanceModel uint8

const (
	GovernanceInvalid   RuntimeGovernanceModel = 0
	GovernanceEntity    RuntimeGovernanceModel = 1
	GovernanceRuntime   RuntimeGovernanceModel = 2
	GovernanceConsensus RuntimeGovernanceModel = 3

	GovernanceMax = GovernanceConsensus

	gmInvalid   = "invalid"
	gmEntity    = "entity"
	gmRuntime   = "runtime"
	gmConsensus = "consensus"
)

// String returns a string representation of a runtime governance model.
func (gm RuntimeGovernanceModel) String() string {
	model, err := gm.MarshalText()
	if err != nil {
		return "[unsupported runtime governance model]"
	}
	return string(model)
}

func (gm RuntimeGovernanceModel) MarshalText() ([]byte, error) {
	switch gm {
	case GovernanceInvalid:
		return []byte(gmInvalid), nil
	case GovernanceEntity:
		return []byte(gmEntity), nil
	case GovernanceRuntime:
		return []byte(gmRuntime), nil
	case GovernanceConsensus:
		return []byte(gmConsensus), nil
	default:
		return nil, fmt.Errorf("unspported runtime governance model: %d", gm)
	}
}

func (gm *RuntimeGovernanceModel) UnmarshalText(text []byte) error {
	switch string(text) {
	case gmEntity:
		*gm = GovernanceEntity
	case gmRuntime:
		*gm = GovernanceRuntime
	case gmConsensus:
		*gm = GovernanceConsensus
	default:
		return fmt.Errorf("unspported runtime governance model: '%s'", string(text))
	}

	return nil
}

// ValidateBasic performs basic descriptor validity checks.
// removed func

// ActiveDeployment returns the currently active deployment for the specified
// epoch if it exists.
// removed func

// DeploymentForVersion returns the deployment corresponding to the passed version if it exists.
// removed func

// ValidateDeployments validates a runtime descriptor's Deployments field
// at the specified epoch.
// removed func

// String returns a string representation of itself.
func (r Runtime) String() string {
	return "<Runtime id=" + r.ID.String() + ">"
}

// IsCompute returns true iff the runtime is a generic compute runtime.
// removed func

// StakingAddress returns the correct staking address for the runtime based
// on its governance model or nil if there is no staking address under the
// given governance model.
// removed func

// VersionInfo is the per-runtime version information.
type VersionInfo struct {
	// Version of the runtime.
	Version version.Version `json:"version"`

	// ValidFrom stores the epoch at which, this version is valid.
	ValidFrom beacon.EpochTime `json:"valid_from"`

	// TEE is the enclave version information, in an enclave provider specific
	// format if any.
	TEE []byte `json:"tee,omitempty"`
}

// Equal compares vs another VersionInfo for equality.
// removed func

// RuntimeGenesis is the runtime genesis information that is used to
// initialize runtime state in the first block.
type RuntimeGenesis struct {
	// StateRoot is the state root that should be used at genesis time. If
	// the runtime should start with empty state, this must be set to the
	// empty hash.
	StateRoot hash.Hash `json:"state_root"`

	// Round is the runtime round in the genesis.
	Round uint64 `json:"round"`
}

// Equal compares vs another RuntimeGenesis for equality.
// removed func

// SanityCheck does basic sanity checking of RuntimeGenesis.
// isGenesis is true, if it is called during consensus chain init.
// removed func

// RuntimeDescriptorProvider is an interface that provides access to runtime descriptors.
// removed interface
