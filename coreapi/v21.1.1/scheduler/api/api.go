// Package api defines the committee scheduler API.
package api

import (
	"fmt"
	"strings"

	"github.com/oasisprotocol/oasis-core/go/common"
	"github.com/oasisprotocol/oasis-core/go/common/crypto/signature"
	"github.com/oasisprotocol/oasis-core/go/common/quantity"

	beacon "github.com/oasisprotocol/nexus/coreapi/v21.1.1/beacon/api"
)

// ModuleName is a unique module name for the scheduler module.
const ModuleName = "scheduler"

// Role is the role a given node plays in a committee.
type Role uint8

const (
	// RoleInvalid is an invalid role (should never appear on the wire).
	RoleInvalid Role = 0
	// RoleWorker indicates the node is a worker.
	RoleWorker Role = 1
	// RoleBackupWorker indicates the node is a backup worker.
	RoleBackupWorker Role = 2

	RoleInvalidName      = "invalid"
	RoleWorkerName       = "worker"
	RoleBackupWorkerName = "backup-worker"
)

// String returns a string representation of a Role.
func (r Role) String() string {
	switch r {
	case RoleInvalid:
		return RoleInvalidName
	case RoleWorker:
		return RoleWorkerName
	case RoleBackupWorker:
		return RoleBackupWorkerName
	default:
		return fmt.Sprintf("[unknown role: %d]", r)
	}
}

// MarshalText encodes a Role into text form.
func (r Role) MarshalText() ([]byte, error) {
	switch r {
	case RoleInvalid:
		return []byte(RoleInvalidName), nil
	case RoleWorker:
		return []byte(RoleWorkerName), nil
	case RoleBackupWorker:
		return []byte(RoleBackupWorkerName), nil
	default:
		return nil, fmt.Errorf("invalid role: %d", r)
	}
}

// UnmarshalText decodes a text slice into a Role.
func (r *Role) UnmarshalText(text []byte) error {
	switch string(text) {
	case RoleWorkerName:
		*r = RoleWorker
	case RoleBackupWorkerName:
		*r = RoleBackupWorker
	default:
		return fmt.Errorf("invalid role: %s", string(text))
	}
	return nil
}

// CommitteeNode is a node participating in a committee.
type CommitteeNode struct {
	// Role is the node's role in a committee.
	Role Role `json:"role"`

	// PublicKey is the node's public key.
	PublicKey signature.PublicKey `json:"public_key"`
}

// CommitteeKind is the functionality a committee exists to provide.
type CommitteeKind uint8

const (
	// KindInvalid is an invalid committee.
	KindInvalid CommitteeKind = 0
	// KindComputeExecutor is an executor committee.
	KindComputeExecutor CommitteeKind = 1
	// KindStorage is a storage committee.
	KindStorage CommitteeKind = 2

	// MaxCommitteeKind is a dummy value used for iterating all committee kinds.
	MaxCommitteeKind = 3

	KindInvalidName         = "invalid"
	KindComputeExecutorName = "executor"
	KindStorageName         = "storage"
)

// MarshalText encodes a CommitteeKind into text form.
func (k CommitteeKind) MarshalText() ([]byte, error) {
	switch k {
	case KindInvalid:
		return []byte(KindInvalidName), nil
	case KindComputeExecutor:
		return []byte(KindComputeExecutorName), nil
	case KindStorage:
		return []byte(KindStorageName), nil
	default:
		return nil, fmt.Errorf("invalid role: %d", k)
	}
}

// UnmarshalText decodes a text slice into a CommitteeKind.
func (k *CommitteeKind) UnmarshalText(text []byte) error {
	switch string(text) {
	case KindComputeExecutorName:
		*k = KindComputeExecutor
	case KindStorageName:
		*k = KindStorage
	default:
		return fmt.Errorf("invalid role: %s", string(text))
	}
	return nil
}

// String returns a string representation of a CommitteeKind.
func (k CommitteeKind) String() string {
	switch k {
	case KindInvalid:
		return KindInvalidName
	case KindComputeExecutor:
		return KindComputeExecutorName
	case KindStorage:
		return KindStorageName
	default:
		return fmt.Sprintf("[unknown kind: %d]", k)
	}
}

// Committee is a per-runtime (instance) committee.
type Committee struct {
	// Kind is the functionality a committee exists to provide.
	Kind CommitteeKind `json:"kind"`

	// Members is the committee members.
	Members []*CommitteeNode `json:"members"`

	// RuntimeID is the runtime ID that this committee is for.
	RuntimeID common.Namespace `json:"runtime_id"`

	// ValidFor is the epoch for which the committee is valid.
	ValidFor beacon.EpochTime `json:"valid_for"`
}

// Workers returns committee nodes with Worker role.
// removed func

// String returns a string representation of a Committee.
func (c Committee) String() string {
	members := make([]string, len(c.Members))
	for i, m := range c.Members {
		members[i] = fmt.Sprintf("%+v", m)
	}
	return fmt.Sprintf("&{Kind:%v Members:[%v] RuntimeID:%v ValidFor:%v}", c.Kind, strings.Join(members, " "), c.RuntimeID, c.ValidFor)
}

// EncodedMembersHash returns the encoded cryptographic hash of the committee members.
// removed func

// BaseUnitsPerVotingPower is the ratio of base units staked to validator power.
var BaseUnitsPerVotingPower quantity.Quantity

// VotingPowerFromStake computes that by dividing by BaseUnitsPerVotingPower.
//
// NOTE: It's not that we're implementation-hiding the conversion though.
// It's just that otherwise if we accidentally skip the `IsInt64`, it would
// still appear to work, and that would be a bad thing to have in a routine
// that's written multiple times.
// removed func

// Validator is a consensus validator.
type Validator struct {
	// ID is the validator Oasis node identifier.
	ID signature.PublicKey `json:"id"`

	// VotingPower is the validator's consensus voting power.
	VotingPower int64 `json:"voting_power"`
}

// Backend is a scheduler implementation.
// removed interface

// GetCommitteesRequest is a GetCommittees request.
type GetCommitteesRequest struct {
	Height    int64            `json:"height"`
	RuntimeID common.Namespace `json:"runtime_id"`
}

// Genesis is the committee scheduler genesis state.
type Genesis struct {
	// Parameters are the scheduler consensus parameters.
	Parameters ConsensusParameters `json:"params"`
}

// ConsensusParameters are the scheduler consensus parameters.
type ConsensusParameters struct {
	// MinValidators is the minimum number of validators that MUST be
	// present in elected validator sets.
	MinValidators int `json:"min_validators"`

	// MaxValidators is the maximum number of validators that MAY be
	// present in elected validator sets.
	MaxValidators int `json:"max_validators"`

	// MaxValidatorsPerEntity is the maximum number of validators that
	// may be elected per entity in a single validator set.
	MaxValidatorsPerEntity int `json:"max_validators_per_entity"`

	// DebugBypassStake is true iff the scheduler should bypass all of
	// the staking related checks and operations.
	DebugBypassStake bool `json:"debug_bypass_stake,omitempty"`

	// DebugStaticValidators is true iff the scheduler should use
	// a static validator set instead of electing anything.
	DebugStaticValidators bool `json:"debug_static_validators,omitempty"`

	// RewardFactorEpochElectionAny is the factor for a reward
	// distributed per epoch to entities that have any node considered
	// in any election.
	RewardFactorEpochElectionAny quantity.Quantity `json:"reward_factor_epoch_election_any"`
}

// SanityCheck does basic sanity checking on the genesis state.
// removed func

// removed func
