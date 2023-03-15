package nodeapi

import (
	"context"

	beacon "github.com/oasisprotocol/oasis-core/go/beacon/api"
	coreCommon "github.com/oasisprotocol/oasis-core/go/common"
	hash "github.com/oasisprotocol/oasis-core/go/common/crypto/hash"
	"github.com/oasisprotocol/oasis-core/go/common/crypto/signature"
	consensus "github.com/oasisprotocol/oasis-core/go/consensus/api"
	consensusTransaction "github.com/oasisprotocol/oasis-core/go/consensus/api/transaction"
	consensusResults "github.com/oasisprotocol/oasis-core/go/consensus/api/transaction/results"
	genesis "github.com/oasisprotocol/oasis-core/go/genesis/api"
	governance "github.com/oasisprotocol/oasis-core/go/governance/api"
	registry "github.com/oasisprotocol/oasis-core/go/registry/api"
	roothash "github.com/oasisprotocol/oasis-core/go/roothash/api"
	scheduler "github.com/oasisprotocol/oasis-core/go/scheduler/api"
	staking "github.com/oasisprotocol/oasis-core/go/staking/api"
	apiTypes "github.com/oasisprotocol/oasis-indexer/api/v1/types"
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
	GetTransactionsWithResults(ctx context.Context, height int64) ([]*TransactionWithResults, error)
	GetEpoch(ctx context.Context, height int64) (beacon.EpochTime, error)
	RegistryEvents(ctx context.Context, height int64) ([]*registry.Event, error)
	StakingEvents(ctx context.Context, height int64) ([]*staking.Event, error)
	GovernanceEvents(ctx context.Context, height int64) ([]*governance.Event, error)
	RoothashEvents(ctx context.Context, height int64) ([]*roothash.Event, error)
	GetValidators(ctx context.Context, height int64) ([]*scheduler.Validator, error)
	GetCommittees(ctx context.Context, height int64, runtimeID coreCommon.Namespace) ([]*scheduler.Committee, error)
	GetProposal(ctx context.Context, height int64, proposalID uint64) (*governance.Proposal, error)
}

// A lightweight subset of `consensus.TransactionsWithResults`.
type TransactionWithResults struct {
	Transaction consensusTransaction.SignedTransaction
	Result      TxResult
}

type TxResult struct {
	Error  TxError
	Events []Event
}

type TxError consensusResults.Error

// A lightweight version of "consensus/api/transactions/results".Event.
//
// NOTE: Not all types of events are expressed as sub-structs, only those that
// the indexer actively inspects (as opposed to just logging them) to perform
// dead reckoning or similar state updates.
type Event struct {
	Height int64
	TxHash hash.Hash

	// The fine-grained Event struct directly from oasis-core; corresponds to
	// at most one of the fields below (StakingTransfer, StakingBurn, etc.).
	Raw interface{}

	// Called "Kind" in oasis-core but "Type" in indexer APIs and DBs.
	Type apiTypes.ConsensusEventType

	StakingTransfer             *TransferEvent
	StakingBurn                 *BurnEvent
	StakingAddEscrow            *AddEscrowEvent
	StakingTakeEscrow           *TakeEscrowEvent
	StakingEscrowDebondingStart *DebondingStartEscrowEvent
	StakingReclaimEscrow        *ReclaimEscrowEvent
	StakingDebondingStart       *DebondingStartEscrowEvent // Available starting in Damask.
	StakingAllowanceChange      *AllowanceChangeEvent

	RegistryRuntime      *RuntimeEvent
	RegistryEntity       *EntityEvent
	RegistryNode         *NodeEvent
	RegistryNodeUnfrozen *NodeUnfrozenEvent

	RoothashExecutorCommitted *ExecutorCommittedEvent

	GovernanceProposalSubmitted *ProposalSubmittedEvent
	GovernanceVote              *VoteEvent
}

type TransferEvent staking.TransferEvent // NOTE: NewShares field is not always populated.
type BurnEvent staking.BurnEvent
type AddEscrowEvent staking.AddEscrowEvent // NOTE: NewShares field is not always populated.
type TakeEscrowEvent staking.TakeEscrowEvent
type DebondingStartEscrowEvent staking.DebondingStartEscrowEvent
type ReclaimEscrowEvent staking.ReclaimEscrowEvent
type AllowanceChangeEvent staking.AllowanceChangeEvent

// RuntimeEvent signifies new runtime registration.
type RuntimeEvent struct {
	ID coreCommon.Namespace

	// EntityID is the public key identifying the Entity controlling
	// the runtime.
	EntityID signature.PublicKey
}
type EntityEvent registry.EntityEvent
type NodeEvent struct {
	NodeID         signature.PublicKey
	EntityID       signature.PublicKey
	RuntimeIDs     []coreCommon.Namespace
	IsRegistration bool
}
type NodeUnfrozenEvent registry.NodeUnfrozenEvent

type ExecutorCommittedEvent struct {
	NodeID *signature.PublicKey // Available starting in Damask.
}

type ProposalSubmittedEvent struct {
	Submitter staking.Address
}
type VoteEvent struct {
	Submitter staking.Address
}
