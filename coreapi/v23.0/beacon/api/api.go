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

	// EpochMax is the end of time.
	EpochMax EpochTime = 0xfffffffffffffffe

	// BackendInsecure is the name of the insecure backend.
	BackendInsecure = "insecure"

	// BackendVRF is the name of the VRF backend.
	BackendVRF = "vrf"
)

// removed var block

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

	// InsecureParameters are the beacon parameters for the insecure backend.
	InsecureParameters *InsecureParameters `json:"insecure_parameters,omitempty"`

	// VRFParameters are the beacon parameters for the VRF backend.
	VRFParameters *VRFParameters `json:"vrf_parameters,omitempty"`
}

// Interval returns the epoch interval (in blocks).
// removed func

// InsecureParameters are the beacon parameters for the insecure backend.
type InsecureParameters struct {
	// Interval is the epoch interval (in blocks).
	Interval int64 `json:"interval,omitempty"`
}

// EpochEvent is the epoch event.
type EpochEvent struct {
	// Epoch is the new epoch.
	Epoch EpochTime `json:"epoch,omitempty"`
}

// EventKind returns a string representation of this event's kind.
// removed func

// BeaconEvent is the beacon event.
type BeaconEvent struct {
	// Beacon is the new beacon value.
	Beacon []byte `json:"beacon,omitempty"`
}

// EventKind returns a string representation of this event's kind.
// removed func
