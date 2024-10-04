package api

import (
	"fmt"

	"github.com/oasisprotocol/oasis-core/go/common/crypto/signature"

	pvss "github.com/oasisprotocol/nexus/coreapi/v21.1.1/common/crypto/pvss"
)

// removed var block

// PVSSParameters are the beacon parameters for the PVSS backend.
type PVSSParameters struct {
	Participants uint32 `json:"participants"`
	Threshold    uint32 `json:"threshold"`

	CommitInterval  int64 `json:"commit_interval"`
	RevealInterval  int64 `json:"reveal_interval"`
	TransitionDelay int64 `json:"transition_delay"`

	DebugForcedParticipants []signature.PublicKey `json:"debug_forced_participants,omitempty"`
}

// PVSSCommit is a PVSS commitment transaction payload.
type PVSSCommit struct {
	Epoch EpochTime `json:"epoch"`
	Round uint64    `json:"round"`

	Commit pvss.Commit `json:"commit,omitempty"`
}

// Implements transaction.MethodMetadataProvider.
// removed func

// PVSSReveal is a PVSS reveal transaction payload.
type PVSSReveal struct {
	Epoch EpochTime `json:"epoch"`
	Round uint64    `json:"round"`

	Reveal pvss.Reveal `json:"reveal,omitempty"`
}

// Implements transaction.MethodMetadataProvider.
// removed func

// RoundState is a PVSS round state.
type RoundState uint8

const (
	StateInvalid  RoundState = 0
	StateCommit   RoundState = 1
	StateReveal   RoundState = 2
	StateComplete RoundState = 3
)

func (s RoundState) String() string {
	switch s {
	case StateInvalid:
		return "invalid"
	case StateCommit:
		return "commit"
	case StateReveal:
		return "reveal"
	case StateComplete:
		return "complete"
	default:
		return fmt.Sprintf("[invalid state: %d]", s)
	}
}

// PVSSState is the PVSS backend state.
type PVSSState struct {
	Height int64 `json:"height,omitempty"`

	Epoch EpochTime  `json:"epoch,omitempty"`
	Round uint64     `json:"round,omitempty"`
	State RoundState `json:"state,omitempty"`

	Instance     interface{}           `json:"instance,omitempty"`
	Participants []signature.PublicKey `json:"participants,omitempty"`
	Entropy      []byte                `json:"entropy,omitempty"`

	BadParticipants map[signature.PublicKey]bool `json:"bad_participants,omitempty"`

	CommitDeadline   int64 `json:"commit_deadline,omitempty"`
	RevealDeadline   int64 `json:"reveal_deadline,omitempty"`
	TransitionHeight int64 `json:"transition_height,omitempty"`

	RuntimeDisableHeight int64 `json:"runtime_disable_height,omitempty"`
}

// PVSSEvent is a PVSS backend event.
type PVSSEvent struct {
	Height int64 `json:"height,omitempty"`

	Epoch EpochTime  `json:"epoch,omitempty"`
	Round uint64     `json:"round,omitempty"`
	State RoundState `json:"state,omitempty"`

	Participants []signature.PublicKey `json:"participants,omitempty"`
}

// removed func

// PVSSBackend is a Backend that is backed by PVSS.
// removed interface
