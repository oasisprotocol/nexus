// Package storage defines storage interfaces.
package storage

import (
	"context"

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
	BlockData(ctx context.Context, height int64) (*BlockData, error)

	// BeaconData gets beacon data at the specified height. This includes
	// the epoch number at that height, as well as the beacon state.
	BeaconData(ctx context.Context, height int64) (*BeaconData, error)

	// RegistryData gets registry data at the specified height. This includes
	// all registered entities and their controlled nodes and statuses.
	RegistryData(ctx context.Context, height int64) (*RegistryData, error)

	// StakingData gets staking data at the specified height. This includes
	// staking backend events to be applied to indexed state.
	StakingData(ctx context.Context, height int64) (*StakingData, error)

	// SchedulerData gets scheduler data at the specified height. This
	// includes all validators and runtime committees.
	SchedulerData(ctx context.Context, height int64) (*SchedulerData, error)

	// GovernanceData gets governance data at the specified height. This
	// includes all proposals, their respective statuses and voting responses.
	GovernanceData(ctx context.Context, height int64) (*GovernanceData, error)

	// TODO: Extend this interface to include a GetRoothashData to pull
	// runtime blocks. This is only relevant when we begin to build runtime
	// analyzers.

	// Name returns the name of the source storage.
	Name() string
}

// TargetStorage defines an interface for reading and writing
// processed block data.
type TargetStorage interface {

	// The following setters apply data from various consensus
	// backends to target storage.
	SetBlockData(ctx context.Context, data *BlockData) error
	SetBeaconData(ctx context.Context, data *BeaconData) error
	SetRegistryData(ctx context.Context, data *RegistryData) error
	SetStakingData(ctx context.Context, data *StakingData) error
	SetSchedulerData(ctx context.Context, data *SchedulerData) error
	SetGovernanceData(ctx context.Context, data *GovernanceData) error

	// Name returns the name of the target storage.
	Name() string
}

// BlockData represents data for a block at a given height.
type BlockData struct {
	Height int64

	BlockHeader  *consensus.Block
	Transactions []*transaction.SignedTransaction
	Results      []*results.Result
}

// BeaconData represents data for the random beacon at a given height.
type BeaconData struct {
	Height int64

	Epoch  beacon.EpochTime
	Beacon []byte
}

// RegistryData represents data for the node registry at a given height.
//
// Note: The registry backend supports getting events directly. We support
// retrieving events as updates to apply when getting data at specific height.
type RegistryData struct {
	Height int64

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
	Height int64

	Transfers        []*staking.TransferEvent
	Burns            []*staking.BurnEvent
	Escrows          []*staking.EscrowEvent
	AllowanceChanges []*staking.AllowanceChangeEvent
}

// SchedulerData represents data for elected committees and validators at a given height.
type SchedulerData struct {
	Height int64

	Validators []*scheduler.Validator
	Committees map[common.Namespace][]*scheduler.Committee
}

// GovernanceData represents governance data for proposals at a given height.
//
// Note: The governance backend supports getting events directly. We support
// retrieving events as updates to apply when getting data at a specific height.
type GovernanceData struct {
	Height int64

	ProposalSubmissions   []*governance.Proposal
	ProposalExecutions    []*governance.ProposalExecutedEvent
	ProposalFinalizations []*governance.Proposal
	Votes                 []*governance.VoteEvent
}
