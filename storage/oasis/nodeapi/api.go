package nodeapi

import (
	"context"

	beacon "github.com/oasisprotocol/oasis-core/go/beacon/api"
	"github.com/oasisprotocol/oasis-core/go/common"
	consensus "github.com/oasisprotocol/oasis-core/go/consensus/api"
	genesis "github.com/oasisprotocol/oasis-core/go/genesis/api"
	governance "github.com/oasisprotocol/oasis-core/go/governance/api"
	registry "github.com/oasisprotocol/oasis-core/go/registry/api"
	roothash "github.com/oasisprotocol/oasis-core/go/roothash/api"
	scheduler "github.com/oasisprotocol/oasis-core/go/scheduler/api"
	staking "github.com/oasisprotocol/oasis-core/go/staking/api"
)

// ConsensusApiLite provides low-level access to the consensus API of one or
// more (versions of) Oasis nodes.
//
// Each method is intended to be a thin wrapper around a corresponding gRPC
// method of the node. In this sense, this is a reimplentation of convenience
// methods in oasis-core.
//
// The difference is that this interface only supports methods needed by the
// indexer, and that implementations allow handling different versions of Oasis
// nodes.
type ConsensusApiLite interface {
	GetGenesisDocument(ctx context.Context) (*genesis.Document, error)
	StateToGenesis(ctx context.Context, height int64) (*genesis.Document, error)
	GetBlock(ctx context.Context, height int64) (*consensus.Block, error)
	GetTransactionsWithResults(ctx context.Context, height int64) (*consensus.TransactionsWithResults, error)
	GetEpoch(ctx context.Context, height int64) (beacon.EpochTime, error)
	RegistryEvents(ctx context.Context, height int64) ([]*registry.Event, error)
	StakingEvents(ctx context.Context, height int64) ([]*staking.Event, error)
	GovernanceEvents(ctx context.Context, height int64) ([]*governance.Event, error)
	RoothashEvents(ctx context.Context, height int64) ([]*roothash.Event, error)
	GetValidators(ctx context.Context, height int64) ([]*scheduler.Validator, error)
	GetCommittees(ctx context.Context, height int64, runtimeID common.Namespace) ([]*scheduler.Committee, error)
	GetProposal(ctx context.Context, height int64, proposalID uint64) (*governance.Proposal, error)
}
