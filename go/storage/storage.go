// Package storage defines storage interfaces.
package storage

// SourceStorage defines an interface for retrieving raw block data.
type SourceStorage interface {

	// GetBlockData gets block data at the specified height. This includes all
	// block header information, as well as transactions and events included
	// within that block.
	GetBlockData(int64)

	// GetBeaconData gets beacon data at the specified height. This includes
	// the epoch number at that height, as well as the beacon state.
	GetBeaconData(int64)

	// GetRegistryData gets registry data at the specified height. This includes
	// all registered entities and their controlled nodes and statuses.
	GetRegistryData(int64)

	// GetStakingData gets staking data at the specified height. This includes
	// staking backend events to be applied to indexed state.
	GetStakingData(int64)

	// GetSchedulingData gets scheduling data at the specified height. This
	// includes all validators and runtime committees.
	GetSchedulingData(int64)

	// GetGovernanceData gets governance data at the specified height. This
	// includes all proposals, their respective statuses and voting responses.
	GetGovernanceData(int64)

	// TODO: Extend this interface to include a GetRoothashData to pull
	// runtime blocks. This is only relevant when we begin to build runtime
	// analyzers.

	// Name returns the name of this source storage.
	Name() string
}

// TargetStorage defines an interface for reading and writing
// processed block data.
type TargetStorage interface {
	// TODO: Define the rest of this interface.

	// Name returns the name of this target storage.
	Name() string
}
