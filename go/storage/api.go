// Package storage defines storage interfaces.
package storage

import (
	beacon "github.com/oasisprotocol/oasis-core/go/beacon/api"
	"github.com/oasisprotocol/oasis-core/go/common"
	consensus "github.com/oasisprotocol/oasis-core/go/consensus/api"
	"github.com/oasisprotocol/oasis-core/go/consensus/api/transaction"
	"github.com/oasisprotocol/oasis-core/go/consensus/api/transaction/results"
	governance "github.com/oasisprotocol/oasis-core/go/governance/api"
	registry "github.com/oasisprotocol/oasis-core/go/registry/api"
	scheduler "github.com/oasisprotocol/oasis-core/go/scheduler/api"
	staking "github.com/oasisprotocol/oasis-core/go/staking/api"
)

// SourceStorage defines an interface for retrieving raw block data.
type SourceStorage interface {

	// BlockData gets block data at the specified height. This includes all
	// block header information, as well as transactions and events included
	// within that block.
	BlockData(height int64) (*BlockData, error)

	// BeaconData gets beacon data at the specified height. This includes
	// the epoch number at that height, as well as the beacon state.
	BeaconData(height int64) (*BeaconData, error)

	// RegistryData gets registry data at the specified height. This includes
	// all registered entities and their controlled nodes and statuses.
	RegistryData(height int64) (*RegistryData, error)

	// StakingData gets staking data at the specified height. This includes
	// staking backend events to be applied to indexed state.
	StakingData(height int64) (*StakingData, error)

	// SchedulerData gets scheduler data at the specified height. This
	// includes all validators and runtime committees.
	SchedulerData(height int64) (*SchedulerData, error)

	// GovernanceData gets governance data at the specified height. This
	// includes all proposals, their respective statuses and voting responses.
	GovernanceData(height int64) (*GovernanceData, error)

	// TODO: Extend this interface to include a GetRoothashData to pull
	// runtime blocks. This is only relevant when we begin to build runtime
	// analyzers.

	// Name returns the name of this source storage.
	Name() string
}

// BlockData represents data for a block at a given height.
type BlockData struct {
	BlockHeader  *consensus.Block
	Transactions []*transaction.SignedTransaction
	Results      []*results.Result
}

// BeaconData represents data for the random beacon at a given height.
type BeaconData struct {
	Epoch  beacon.EpochTime
	Beacon []byte
}

// RegistryData represents data for the node registry at a given height.
//
// Note: The registry backend supports getting events directly. We support
// retrieving events as updates to apply when getting data at specific height.
type RegistryData struct {
	RuntimeEvents     []*registry.RuntimeEvent
	EntityEvents      []*registry.EntityEvent
	NodeEvent         []*registry.NodeEvent
	NodeUnfrozenEvent []*registry.NodeUnfrozenEvent
}

// StakingData represents data for accounts at a given height.
//
// Note: The staking backend supports getting events directly. We support
// retrieving events as updates to apply when getting data at specific height.
type StakingData struct {
	Transfers        []*staking.TransferEvent
	Burns            []*staking.BurnEvent
	Escrows          []*staking.EscrowEvent
	AllowanceChanges []*staking.AllowanceChangeEvent
}

// SchedulerData represents data for elected committees and validators at a given height.
type SchedulerData struct {
	Validators []*scheduler.Validator
	Committees map[common.Namespace][]*scheduler.Committee
}

// GovernanceData represents governance data for proposals at a given height.
//
// Note: The governance backend supports getting events directly. We support
// retrieving events as updates to apply when getting data at a specific height.
type GovernanceData struct {
	ProposalSubmissions   []*governance.ProposalSubmittedEvent
	ProposalExecutions    []*governance.ProposalExecutedEvent
	ProposalFinalizations []*governance.ProposalFinalizedEvent
	Votes                 []*governance.VoteEvent
}

// TargetStorage defines an interface for reading and writing
// processed block data.
type TargetStorage interface {
	// TODO: Define the rest of this interface.

	// Name returns the name of this target storage.
	Name() string
}
