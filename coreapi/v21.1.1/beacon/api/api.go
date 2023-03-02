// Package api implements the random beacon and time keeping APIs.
package api

import (
	original "github.com/oasisprotocol/oasis-core/go/beacon/api"
)

const (
	// ModuleName is a unique module name for the beacon module.
	ModuleName = "beacon"

	// BeaconSize is the size of the beacon in bytes.
	BeaconSize = 32

	// EpochInvalid is the placeholder invalid epoch.
	EpochInvalid EpochTime = 0xffffffffffffffff // ~50 quadrillion years away.

	// BackendInsecure is the name of the insecure backend.
	BackendInsecure = "insecure"

	// BackendPVSS is the name of the PVSS backend.
	BackendPVSS = "pvss"
)

// ErrBeaconNotAvailable is the error returned when a beacon is not
// available for the requested height for any reason.
// removed var statement

// EpochTime is the number of intervals (epochs) since a fixed instant
// in time/block height (epoch date/height).
type EpochTime = original.EpochTime

// AbsDiff returns the absolute difference (in epochs) between two epochtimes.
// removed func

// EpochTimeState is the epoch state.
type EpochTimeState struct {
	Epoch  EpochTime `json:"epoch"`
	Height int64     `json:"height"`
}

// Backend is a random beacon/time keeping implementation.
// removed interface

// SetableBackend is a Backend that supports setting the current epoch.
// removed interface

// Genesis is the genesis state.
type Genesis struct {
	// Base is the starting epoch.
	Base EpochTime `json:"base"`

	// Parameters are the beacon consensus parameters.
	Parameters ConsensusParameters `json:"params"`
}

// ConsensusParameters are the beacon consensus parameters.
type ConsensusParameters struct {
	// Backend is the beacon backend.
	Backend string `json:"backend"`

	// DebugMockBackend is flag for enabling the mock epochtime backend.
	DebugMockBackend bool `json:"debug_mock_backend,omitempty"`

	// DebugDeterministic is true iff the output should be deterministic.
	DebugDeterministic bool `json:"debug_deterministic,omitempty"`

	// InsecureParameters are the beacon parameters for the insecure backend.
	InsecureParameters *InsecureParameters `json:"insecure_parameters,omitempty"`

	// PVSSParameters are the beacon parameters for the PVSS backend.
	PVSSParameters *PVSSParameters `json:"pvss_parameters,omitempty"`
}

// InsecureParameters are the beacon parameters for the insecure backend.
type InsecureParameters struct {
	// Interval is the epoch interval (in blocks).
	Interval int64 `json:"interval,omitempty"`
}

// SanityCheck does basic sanity checking on the genesis state.
// removed func
