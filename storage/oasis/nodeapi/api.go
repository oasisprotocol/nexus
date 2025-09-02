// Package nodeapi defines the types used by Nexus to represent the data
// returned by the node API.
//
// Due to the (sometimes breaking) changes in the types used by different
// evolutions of oasis-core over the Cobalt, Damask, and Eden upgrades,
// Nexus defines its own set of internal types here.
//
// The types that it exposes are mostly simplified versions of the types exposed by
// oasis-core Damask: The top-level type structs are defined in this file, and
// the types of their fields are almost universally directly the types exposed
// by oasis-core Damask. The reason is that as oasis-core evolves, Damask types
// are mostly able to represent all the information from Cobalt, plus some. However,
// there are a few exceptions where we use oasis-core Eden types instead, e.g. in the
// `governance` module. Over time, types should be migrated to newer versions, e.g. Eden
// or later, as needed.
package nodeapi

import (
	"context"
	"encoding/json"
	"time"

	coreCommon "github.com/oasisprotocol/oasis-core/go/common"

	hash "github.com/oasisprotocol/oasis-core/go/common/crypto/hash"
	"github.com/oasisprotocol/oasis-core/go/common/crypto/signature"
	"github.com/oasisprotocol/oasis-core/go/common/errors"
	"github.com/oasisprotocol/oasis-core/go/common/quantity"

	beacon "github.com/oasisprotocol/nexus/coreapi/v22.2.11/beacon/api"
	node "github.com/oasisprotocol/nexus/coreapi/v22.2.11/common/node"
	consensusTransaction "github.com/oasisprotocol/nexus/coreapi/v22.2.11/consensus/api/transaction"
	consensusResults "github.com/oasisprotocol/nexus/coreapi/v22.2.11/consensus/api/transaction/results"
	registry "github.com/oasisprotocol/nexus/coreapi/v22.2.11/registry/api"
	roothash "github.com/oasisprotocol/nexus/coreapi/v22.2.11/roothash/api"
	"github.com/oasisprotocol/nexus/coreapi/v22.2.11/roothash/api/message"
	scheduler "github.com/oasisprotocol/nexus/coreapi/v22.2.11/scheduler/api"
	staking "github.com/oasisprotocol/nexus/coreapi/v22.2.11/staking/api"
	consensus "github.com/oasisprotocol/nexus/coreapi/v24.0/consensus/api"
	governance "github.com/oasisprotocol/nexus/coreapi/v24.0/governance/api"
	"github.com/oasisprotocol/nexus/storage/oasis/connections"

	sdkClient "github.com/oasisprotocol/oasis-sdk/client-sdk/go/client"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/modules/rofl"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/modules/roflmarket"
	sdkTypes "github.com/oasisprotocol/oasis-sdk/client-sdk/go/types"

	apiTypes "github.com/oasisprotocol/nexus/api/v1/types"
	common "github.com/oasisprotocol/nexus/common"
)

// ....................................................
// ....................  Consensus  ...................
// ....................................................

// ConsensusApiLite provides low-level access to the consensus API of one or
// more (versions of) Oasis nodes.
//
// Each method of this corresponds to a gRPC method of the node. In this sense,
// this is a reimplementation of convenience gRPC wrapers in oasis-core.
// However, this interface ONLY supports methods needed by Nexus, and the
// return values (events, genesis doc, ...) are converted to simplified internal
// types that mirror oasis-core types but contain only the fields relevant to
// Nexus.
//
// Since the types are simpler and fewer, their structure is, in
// places, flattened compared to their counterparts in oasis-core.
type ConsensusApiLite interface {
	GetGenesisDocument(ctx context.Context, chainContext string) (*GenesisDocument, error)
	StateToGenesis(ctx context.Context, height int64) (*GenesisDocument, error)
	GetConsensusParameters(ctx context.Context, height int64) (*ConsensusParameters, error)
	GetBlock(ctx context.Context, height int64) (*consensus.Block, error)
	GetTransactionsWithResults(ctx context.Context, height int64) ([]TransactionWithResults, error)
	GetEpoch(ctx context.Context, height int64) (beacon.EpochTime, error)
	RegistryEvents(ctx context.Context, height int64) ([]Event, error)
	StakingEvents(ctx context.Context, height int64) ([]Event, error)
	GovernanceEvents(ctx context.Context, height int64) ([]Event, error)
	RoothashEvents(ctx context.Context, height int64) ([]Event, error)
	RoothashLastRoundResults(ctx context.Context, height int64, runtimeID coreCommon.Namespace) (*roothash.RoundResults, error)
	GetValidators(ctx context.Context, height int64) ([]Validator, error)
	GetNodes(ctx context.Context, height int64) ([]Node, error)
	GetCommittees(ctx context.Context, height int64, runtimeID coreCommon.Namespace) ([]Committee, error)
	GetProposal(ctx context.Context, height int64, proposalID uint64) (*Proposal, error)
	GetAccount(ctx context.Context, height int64, address Address) (*Account, error)
	DelegationsTo(ctx context.Context, height int64, address Address) (map[Address]*Delegation, error)
	StakingTotalSupply(ctx context.Context, height int64) (*quantity.Quantity, error)
	Close() error
	// Exposes the underlying gRPC connection, if applicable. Implementations may return nil.
	// NOTE: Intended only for debugging purposes, e.g. one-off testing of gRPC methods that
	//       are not exposed via one of the above wrappers.
	GrpcConn() connections.GrpcConn
}

// A lightweight subset of `consensus.Parameters`.
type ConsensusParameters struct {
	MaxBlockGas  uint64 `json:"max_block_gas"`
	MaxBlockSize uint64 `json:"max_block_size"`
}

// A lightweight subset of `consensus.TransactionsWithResults`.
type TransactionWithResults struct {
	Transaction consensusTransaction.SignedTransaction
	Result      TxResult
}

type TxResult struct {
	Error   TxError
	Events  []Event
	GasUsed uint64 `json:"gas_used,omitempty"` // Added in 24.3.
}

// IsSuccess returns true if transaction execution was successful.
func (r *TxResult) IsSuccess() bool {
	return r.Error.Code == errors.CodeNoError
}

type TxError consensusResults.Error

type Address = common.Address

// A lightweight version of "consensus/api/transactions/results".Event.
//
// NOTE: Not all types of events are expressed as sub-structs, only those that
// Nexus actively inspects (as opposed to just logging them) to perform
// dead reckoning or similar state updates.
type Event struct {
	Height int64
	TxHash hash.Hash

	// Called "Kind" in oasis-core but "Type" in Nexus APIs and DBs.
	Type     apiTypes.ConsensusEventType
	EventIdx int // Index of the event within the same block and event type.

	// The body of the Event struct as it was received from oasis-core. For most
	// event types, a summary of the event (containing only nexus-relevant fields)
	// will be present in one of the fields below (StakingTransfer, StakingBurn, etc.).
	// For event types that Nexus doesn't process beyond logging, only this
	// field will be populated.
	// We convert to JSON and effectively erase the type here in order to decouple
	// oasis-core types (which vary between versions) from Nexus.
	RawBody json.RawMessage

	StakingTransfer             *TransferEvent
	StakingBurn                 *BurnEvent
	StakingAddEscrow            *AddEscrowEvent
	StakingTakeEscrow           *TakeEscrowEvent
	StakingEscrowDebondingStart *DebondingStartEscrowEvent
	StakingReclaimEscrow        *ReclaimEscrowEvent
	StakingDebondingStart       *DebondingStartEscrowEvent // Available starting in Damask.
	StakingAllowanceChange      *AllowanceChangeEvent

	RegistryRuntimeStarted   *RuntimeStartedEvent
	RegistryRuntimeSuspended *RuntimeSuspendedEvent // Available starting with Eden.
	RegistryEntity           *EntityEvent
	RegistryNode             *NodeEvent
	RegistryNodeUnfrozen     *NodeUnfrozenEvent

	RoothashMisc              *RoothashEvent
	RoothashExecutorCommitted *ExecutorCommittedEvent
	RoothashMessage           *MessageEvent // Available only in Cobalt.

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
	Account                   staking.Account
	Delegation                staking.Delegation
)

// .................... Registry ....................

// RuntimeStartedEvent signifies new runtime registration.
// This is a stripped-down version of an unfortunately named `registry.RuntimeEvent` (in Cobalt and Damask).
// Post-Damask, this is replaced by registry.RuntimeStartedEvent.
type RuntimeStartedEvent struct {
	ID          coreCommon.Namespace
	EntityID    signature.PublicKey   // The Entity controlling the runtime.
	Kind        string                // enum: "compute", "keymanager"
	KeyManager  *coreCommon.Namespace // Key manager runtime ID.
	TEEHardware string                // enum: "invalid" (= no TEE), "intel-sgx"
}
type RuntimeSuspendedEvent struct {
	RuntimeID coreCommon.Namespace
}
type (
	Node              node.Node
	NodeUnfrozenEvent registry.NodeUnfrozenEvent
	EntityEvent       registry.EntityEvent
	NodeEvent         struct {
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

// .................... RootHash  ....................

// RoothashEvent is a subset of various roothash.(...)Event subfields.
// Contains just the attributes we care to expose via the API in a parsed
// form.
// Used to represent events of disparate types; depending on the type of the
// event, not all fields are populated.
type RoothashEvent struct {
	// RuntimeID is RuntimeID from the roothash/api `Event` that would be
	// around this event body. We're flattening it in here to preserve it.
	RuntimeID coreCommon.Namespace
	Round     *uint64
}

type ExecutorCommittedEvent struct {
	// RuntimeID is RuntimeID from the roothash/api `Event` that would be
	// around this event body. We're flattening it in here to preserve it.
	RuntimeID coreCommon.Namespace
	Round     uint64
	NodeID    *signature.PublicKey // Available starting in Damask.
	Messages  []message.Message
}

type MessageEvent struct {
	// RuntimeID is RuntimeID from the roothash/api `Event` that would be
	// around this event body. We're flattening it in here to preserve it.
	RuntimeID coreCommon.Namespace
	Module    string
	Code      uint32
	Index     uint32
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
	Committee struct {
		Kind      CommitteeKind
		Members   []*scheduler.CommitteeNode
		RuntimeID coreCommon.Namespace
		ValidFor  beacon.EpochTime
	}
)

// .................... Governance ....................

type Proposal governance.Proposal

// ....................................................
// ....................  Runtimes  ....................
// ....................................................

// Like ConsensusApiLite, but for the runtime API.
type RuntimeApiLite interface {
	GetEventsRaw(ctx context.Context, round uint64) ([]RuntimeEvent, error)
	EVMSimulateCall(ctx context.Context, round uint64, gasPrice []byte, gasLimit uint64, caller []byte, address []byte, value []byte, data []byte) (*FallibleResponse, error)
	EVMGetCode(ctx context.Context, round uint64, address []byte) ([]byte, error)
	GetBlockHeader(ctx context.Context, round uint64) (*RuntimeBlockHeader, error)
	GetTransactionsWithResults(ctx context.Context, round uint64) ([]RuntimeTransactionWithResults, error)
	GetBalances(ctx context.Context, round uint64, addr Address) (map[sdkTypes.Denomination]common.BigInt, error)
	GetMinGasPrice(ctx context.Context, round uint64) (map[sdkTypes.Denomination]common.BigInt, error)

	RoflApp(ctx context.Context, round uint64, id AppID) (*AppConfig, error)
	RoflApps(ctx context.Context, round uint64) ([]*AppConfig, error)
	RoflAppInstance(ctx context.Context, round uint64, id AppID, rak PublicKey) (*Registration, error)
	RoflAppInstances(ctx context.Context, round uint64, id AppID) ([]*Registration, error)

	RoflMarketProvider(ctx context.Context, round uint64, providerAddress sdkTypes.Address) (*Provider, error)
	RoflMarketProviders(ctx context.Context, round uint64) ([]*Provider, error)
	RoflMarketOffer(ctx context.Context, round uint64, providerAddress sdkTypes.Address, offerID OfferID) (*Offer, error)
	RoflMarketOffers(ctx context.Context, round uint64, providerAddress sdkTypes.Address) ([]*Offer, error)
	RoflMarketInstance(ctx context.Context, round uint64, providerAddress sdkTypes.Address, instanceID InstanceID) (*Instance, error)
	RoflMarketInstances(ctx context.Context, round uint64, providerAddress sdkTypes.Address) ([]*Instance, error)
	RoflMarketInstanceCommands(ctx context.Context, round uint64, providerAddress sdkTypes.Address, instanceID InstanceID) ([]*QueuedCommand, error)

	Close() error
}

type (
	RuntimeEvent                  sdkTypes.Event
	RuntimeTransactionWithResults sdkClient.TransactionWithResults

	AppID        = rofl.AppID
	AppConfig    = rofl.AppConfig
	Registration = rofl.Registration
	PublicKey    = sdkTypes.PublicKey

	QueuedCommand = roflmarket.QueuedCommand
	InstanceID    = roflmarket.InstanceID
	OfferID       = roflmarket.OfferID
	Provider      = roflmarket.Provider
	Offer         = roflmarket.Offer
	Instance      = roflmarket.Instance
)

// Derived from oasis-core: roothash/api/block/header.go
// Expanded to include the precomputed hash of the header;
// we mustn't compute it on the fly because depending on the
// node version, this header struct might not be the exact struct
// returned from the node, so its real hash will differ.
type RuntimeBlockHeader struct { //nolint: maligned
	Version   uint16
	Namespace coreCommon.Namespace
	Round     uint64
	Timestamp time.Time
	// Hash of the raw header struct as received from the node API.
	// The `PreviousHash` of the next round's block should match this.
	Hash           hash.Hash
	PreviousHash   hash.Hash
	IORoot         hash.Hash
	StateRoot      hash.Hash
	MessagesHash   hash.Hash
	InMessagesHash hash.Hash // NOTE: Available starting in Damask.
}

// GenesisDocument is a stripped-down version of `genesis.Document`.
type GenesisDocument struct {
	// Height is the block height at which the document was generated.
	Height int64 `json:"height"`
	// Time is the time the genesis block was constructed.
	Time time.Time `json:"genesis_time"`
	// ChainID is the ID of the chain.
	ChainID string `json:"chain_id"`
	// BaseEpoch is the base epoch for the chain.
	BaseEpoch uint64 `json:"base_epoch"`
	// Registry is the registry genesis state.
	Registry registry.Genesis `json:"registry"`
	// RootHash is the roothash genesis state.
	Staking staking.Genesis `json:"staking"`
	// Governance is the governance genesis state.
	Governance governance.Genesis `json:"governance"`
}
