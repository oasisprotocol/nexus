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
	roothash "github.com/oasisprotocol/oasis-core/go/roothash/api/block"
	scheduler "github.com/oasisprotocol/oasis-core/go/scheduler/api"
	staking "github.com/oasisprotocol/oasis-core/go/staking/api"
	apiTypes "github.com/oasisprotocol/oasis-indexer/api/v1/types"
	sdkClient "github.com/oasisprotocol/oasis-sdk/client-sdk/go/client"
	sdkTypes "github.com/oasisprotocol/oasis-sdk/client-sdk/go/types"
)

// ConsensusApiLite provides low-level access to the consensus API of one or
// more (versions of) Oasis nodes.
//
// Each method of this corresponds to a gRPC method of the node. In this sense,
// this is a reimplementation of convenience gRPC wrapers in oasis-core.
// However, this interface ONLY supports methods needed by the indexer, and the
// return values (events, genesis doc, ...) are converted to simplified internal
// types that mirror oasis-core types but contain only the fields relevant to
// the indexer.
//
// Since the types are simpler and fewer, their structure is, in
// places, flattened compared to their counterparts in oasis-core.
type ConsensusApiLite interface {
	// TODO: Introduce internal, stripped-down version of `genesis.Document`.
	GetGenesisDocument(ctx context.Context) (*genesis.Document, error)
	StateToGenesis(ctx context.Context, height int64) (*genesis.Document, error)
	GetBlock(ctx context.Context, height int64) (*consensus.Block, error)
	GetTransactionsWithResults(ctx context.Context, height int64) ([]TransactionWithResults, error)
	GetEpoch(ctx context.Context, height int64) (beacon.EpochTime, error)
	RegistryEvents(ctx context.Context, height int64) ([]Event, error)
	StakingEvents(ctx context.Context, height int64) ([]Event, error)
	GovernanceEvents(ctx context.Context, height int64) ([]Event, error)
	RoothashEvents(ctx context.Context, height int64) ([]Event, error)
	GetValidators(ctx context.Context, height int64) ([]Validator, error)
	GetCommittees(ctx context.Context, height int64, runtimeID coreCommon.Namespace) ([]Committee, error)
	GetProposal(ctx context.Context, height int64, proposalID uint64) (*Proposal, error)
}

// Like ConsensusApiLite, but for the runtime API.
type RuntimeApiLite interface {
	GetEventsRaw(ctx context.Context, round uint64) ([]*RuntimeEvent, error)
	EVMSimulateCall(ctx context.Context, round uint64, gasPrice []byte, gasLimit uint64, caller []byte, address []byte, value []byte, data []byte) ([]byte, error)
	GetBlockHeader(ctx context.Context, round uint64) (*RuntimeBlockHeader, error)
	GetTransactionsWithResults(ctx context.Context, round uint64) ([]*RuntimeTransactionWithResults, error)
}

type RuntimeEvent sdkTypes.Event
type RuntimeBlockHeader roothash.Header
type RuntimeTransactionWithResults sdkClient.TransactionWithResults

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
	Body interface{}

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

	RegistryRuntimeRegistered *RuntimeRegisteredEvent
	RegistryEntity            *EntityEvent
	RegistryNode              *NodeEvent
	RegistryNodeUnfrozen      *NodeUnfrozenEvent

	RoothashExecutorCommitted *ExecutorCommittedEvent

	GovernanceProposalSubmitted *ProposalSubmittedEvent
	GovernanceProposalExecuted  *ProposalExecutedEvent
	GovernanceProposalFinalized *ProposalFinalizedEvent
	GovernanceVote              *VoteEvent
}

// .................... Staking  ....................
type (
	TransferEvent             staking.TransferEvent // NOTE: NewShares field is available starting with Damask.
	BurnEvent                 staking.BurnEvent
	AddEscrowEvent            staking.AddEscrowEvent // NOTE: NewShares field is available starting with Damask.
	TakeEscrowEvent           staking.TakeEscrowEvent
	DebondingStartEscrowEvent staking.DebondingStartEscrowEvent
	ReclaimEscrowEvent        staking.ReclaimEscrowEvent
	AllowanceChangeEvent      staking.AllowanceChangeEvent
)

// .................... Registry ....................

// RuntimeRegisteredEvent signifies new runtime registration.
type RuntimeRegisteredEvent struct {
	ID          coreCommon.Namespace
	EntityID    signature.PublicKey   // The Entity controlling the runtime.
	Kind        string                // enum: "compute", "keymanager"
	KeyManager  *coreCommon.Namespace // Key manager runtime ID.
	TEEHardware string                // enum: "invalid" (= no TEE), "intel-sgx"
}
type (
	EntityEvent registry.EntityEvent
	NodeEvent   struct {
		NodeID             signature.PublicKey
		EntityID           signature.PublicKey
		Expiration         uint64 // Epoch in which the node expires.
		RuntimeIDs         []coreCommon.Namespace
		VRFPubKey          *signature.PublicKey
		TLSAddresses       []string // TCP addresses of the node's TLS-enabled gRPC endpoint.
		TLSPubKey          signature.PublicKey
		TLSNextPubKey      signature.PublicKey
		P2PID              signature.PublicKey
		P2PAddresses       []string // TCP addresses of the node's P2P endpoint.
		ConsensusID        signature.PublicKey
		ConsensusAddresses []string // TCP addresses of the node's tendermint endpoint.
		Roles              []string // enum: "compute", "key-manager", "validator", "consensus-rpc", "storage-rpc"
		SoftwareVersion    string
		IsRegistration     bool
	}
)
type NodeUnfrozenEvent registry.NodeUnfrozenEvent

// .................... RootHash  ....................

type ExecutorCommittedEvent struct {
	NodeID *signature.PublicKey // Available starting in Damask.
}

// .................... Governance ....................

type (
	ProposalSubmittedEvent governance.ProposalSubmittedEvent
	ProposalExecutedEvent  governance.ProposalExecutedEvent
	ProposalFinalizedEvent governance.ProposalFinalizedEvent
	VoteEvent              struct {
		ID        uint64
		Submitter staking.Address
		Vote      string // enum: "yes", "no", "abstain"
	}
)

// .................... Scheduler ....................

type (
	Validator scheduler.Validator
	Committee scheduler.Committee
)

// .................... Governance ....................

type Proposal governance.Proposal
