package api

import (
	"github.com/oasisprotocol/nexus/coreapi/v22.2.11/consensus/api/transaction"
	"github.com/oasisprotocol/oasis-core/go/common/crypto/signature"
)

// GasOpVRFProve is the gas operation identifier for VRF proof submission.
const GasOpVRFProve transaction.Op = "vrf_prove"

// removed var block

// VRFParameters are the beacon parameters for the VRF backend.
type VRFParameters struct {
	// AlphaHighQualityThreshold is the minimum number of proofs (Pi)
	// that must be received for the next input (Alpha) to be considered
	// high quality.  If the VRF input is not high quality, runtimes will
	// be disabled for the next epoch.
	AlphaHighQualityThreshold uint64 `json:"alpha_hq_threshold,omitempty"`

	// Interval is the epoch interval (in blocks).
	Interval int64 `json:"interval,omitempty"`

	// ProofSubmissionDelay is the wait peroid in blocks after an epoch
	// transition that nodes MUST wait before attempting to submit a
	// VRF proof for the next epoch's elections.
	ProofSubmissionDelay int64 `json:"proof_delay,omitempty"`

	// GasCosts are the VRF proof gas costs.
	GasCosts transaction.Costs `json:"gas_costs,omitempty"`
}

// VRFState is the VRF backend state.
type VRFState struct {
	// Epoch is the epoch for which this alpha is valid.
	Epoch EpochTime `json:"epoch,omitempty"`

	// Alpha is the active VRF alpha_string input.
	Alpha []byte `json:"alpha,omitempty"`

	// Pi is the accumulated pi_string (VRF proof) outputs.
	Pi map[signature.PublicKey]*signature.Proof `json:"pi,omitempty"`

	// AlphaIsHighQuality is true iff the alpha was generated from
	// high quality input such that elections will be possible.
	AlphaIsHighQuality bool `json:"alpha_hq,omitempty"`

	// SubmitAfter is the block height after which nodes may submit
	// VRF proofs for the current epoch.
	SubmitAfter int64 `json:"submit_after,omitempty"`

	// PrevState is the VRF state from the previous epoch, for the
	// current epoch's elections.
	PrevState *PrevVRFState `json:"prev_state,omitempty"`
}

// PrevVRFState is the previous epoch's VRF state that is to be used for
// elections.
type PrevVRFState struct {
	// Pi is the accumulated pi_string (VRF proof) outputs for the
	// previous epoch.
	Pi map[signature.PublicKey]*signature.Proof `json:"pi.omitempty"`

	// CanElectCommittees is true iff the previous alpha was generated
	// from high quality input such that committee elections are possible.
	CanElectCommittees bool `json:"can_elect,omitempty"`
}

// VRFProve is a VRF proof transaction payload.
type VRFProve struct {
	Epoch EpochTime `json:"epoch"`

	Pi []byte `json:"pi"`
}

// VRFEvent is a VRF backend event.
type VRFEvent struct {
	// Epoch is the epoch that Alpha is valid for.
	Epoch EpochTime `json:"epoch,omitempty"`

	// Alpha is the active VRF alpha_string input.
	Alpha []byte `json:"alpha,omitempty"`

	// SubmitAfter is the block height after which nodes may submit
	// VRF proofs for the current epoch.
	SubmitAfter int64 `json:"submit_after"`
}

// removed func

// EventKind returns a string representation of this event's kind.
// removed func

// VRFBackend is a Backend that is backed by VRFs.
// removed interface
