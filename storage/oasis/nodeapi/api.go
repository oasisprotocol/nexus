package nodeapi

import (
	"context"
	"encoding/json"
	"time"

	beacon "github.com/oasisprotocol/oasis-core/go/beacon/api"
	coreCommon "github.com/oasisprotocol/oasis-core/go/common"
	hash "github.com/oasisprotocol/oasis-core/go/common/crypto/hash"
	"github.com/oasisprotocol/oasis-core/go/common/crypto/signature"
	"github.com/oasisprotocol/oasis-core/go/common/errors"
	consensus "github.com/oasisprotocol/oasis-core/go/consensus/api"
	consensusTransaction "github.com/oasisprotocol/oasis-core/go/consensus/api/transaction"
	consensusResults "github.com/oasisprotocol/oasis-core/go/consensus/api/transaction/results"
	genesis "github.com/oasisprotocol/oasis-core/go/genesis/api"
	governance "github.com/oasisprotocol/oasis-core/go/governance/api"
	registry "github.com/oasisprotocol/oasis-core/go/registry/api"
	scheduler "github.com/oasisprotocol/oasis-core/go/scheduler/api"
	staking "github.com/oasisprotocol/oasis-core/go/staking/api"

	apiTypes "github.com/oasisprotocol/oasis-indexer/api/v1/types"
	sdkClient "github.com/oasisprotocol/oasis-sdk/client-sdk/go/client"
	sdkTypes "github.com/oasisprotocol/oasis-sdk/client-sdk/go/types"
)

// ....................................................
// ....................  Consensus  ...................
// ....................................................

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

// A lightweight subset of `consensus.TransactionsWithResults`.
type TransactionWithResults struct {
	Transaction consensusTransaction.SignedTransaction `json:"transaction"`
	Result      TxResult                               `json:"result"`
}

type TxResult struct {
	Error  TxError `json:"error"`
	Events []Event `json:"events"`
}

// IsSuccess returns true if transaction execution was successful.
func (r *TxResult) IsSuccess() bool {
	return r.Error.Code == errors.CodeNoError
}

type TxError consensusResults.Error

// A lightweight version of "consensus/api/transactions/results".Event.
//
// NOTE: Not all types of events are expressed as sub-structs, only those that
// the indexer actively inspects (as opposed to just logging them) to perform
// dead reckoning or similar state updates.
type Event struct {
	Height int64     `json:"height"`
	TxHash hash.Hash `json:"tx_hash"`

	// The body of the Event struct as it was received from oasis-core. For most
	// event types, a summary of the event (containing only indexer-relevant fields)
	// will be present in one of the fields below (StakingTransfer, StakingBurn, etc.).
	// For event types that the indexer doesn't process beyond logging, only this
	// field will be populated.
	// We convert to JSON and effectively erase the type here in order to decouple
	// oasis-core types (which vary between versions) from the indexer.
	RawBodyJSON json.RawMessage `json:"raw_body"`

	// Called "Kind" in oasis-core but "Type" in indexer APIs and DBs.
	Type apiTypes.ConsensusEventType `json:"type"`

	StakingTransfer             *TransferEvent             `json:"staking_transfer,omitempty"`
	StakingBurn                 *BurnEvent                 `json:"staking_burn,omitempty"`
	StakingAddEscrow            *AddEscrowEvent            `json:"staking_add_escrow,omitempty"`
	StakingTakeEscrow           *TakeEscrowEvent           `json:"staking_take_escrow,omitempty"`
	StakingEscrowDebondingStart *DebondingStartEscrowEvent `json:"staking_escrow_debonding_start,omitempty"`
	StakingReclaimEscrow        *ReclaimEscrowEvent        `json:"staking_reclaim_escrow,omitempty"`
	StakingDebondingStart       *DebondingStartEscrowEvent `json:"staking_debonding_start,omitempty"` // Available starting in Damask.
	StakingAllowanceChange      *AllowanceChangeEvent      `json:"staking_allowance_change,omitempty"`

	RegistryRuntimeRegistered *RuntimeRegisteredEvent `json:"registry_runtime_registered,omitempty"`
	RegistryEntity            *EntityEvent            `json:"registry_entity,omitempty"`
	RegistryNode              *NodeEvent              `json:"registry_node,omitempty"`
	RegistryNodeUnfrozen      *NodeUnfrozenEvent      `json:"registry_node_unfrozen,omitempty"`

	RoothashExecutorCommitted *ExecutorCommittedEvent `json:"roothash_executor_committed,omitempty"`

	GovernanceProposalSubmitted *ProposalSubmittedEvent `json:"governance_proposal_submitted,omitempty"`
	GovernanceProposalExecuted  *ProposalExecutedEvent  `json:"governance_proposal_executed,omitempty"`
	GovernanceProposalFinalized *ProposalFinalizedEvent `json:"governance_proposal_finalized,omitempty"`
	GovernanceVote              *VoteEvent              `json:"governance_vote,omitempty"`
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
// This is a stripped-down version of an unfortunately named `registry.RuntimeEvent` (in Cobalt and Damask).
// Post-Damask, this is replaced by registry.RuntimeStartedEvent.type RuntimeRegisteredEvent struct {
type RuntimeRegisteredEvent struct {
	ID          coreCommon.Namespace  `json:"id"`
	EntityID    signature.PublicKey   `json:"entity_id"`             // The Entity controlling the runtime.
	Kind        string                `json:"kind"`                  // enum: "compute", "keymanager"
	KeyManager  *coreCommon.Namespace `json:"key_manager,omitempty"` // Key manager runtime ID.
	TEEHardware string                `json:"tee_hardware"`          // enum: "invalid" (= no TEE), "intel-sgx"
}
type (
	EntityEvent registry.EntityEvent
	NodeEvent   struct {
		NodeID             signature.PublicKey    `json:"node_id"`
		EntityID           signature.PublicKey    `json:"entity_id"`
		Expiration         uint64                 `json:"expiration"` // Epoch in which the node expires.
		RuntimeIDs         []coreCommon.Namespace `json:"runtime_ids"`
		VRFPubKey          *signature.PublicKey   `json:"vrf_pub_key,omitempty"`
		TLSAddresses       []string               `json:"tls_addresses"` // TCP addresses of the node's TLS-enabled gRPC endpoint.
		TLSPubKey          signature.PublicKey    `json:"tls_pub_key"`
		TLSNextPubKey      signature.PublicKey    `json:"tls_next_pub_key,omitempty"`
		P2PID              signature.PublicKey    `json:"p2p_id"`
		P2PAddresses       []string               `json:"p2p_addresses"` // TCP addresses of the node's P2P endpoint.
		ConsensusID        signature.PublicKey    `json:"consensus_id"`
		ConsensusAddresses []string               `json:"consensus_addresses"` // TCP addresses of the node's tendermint endpoint.
		Roles              []string               `json:"roles"`               // enum: "compute", "key-manager", "validator", "consensus-rpc", "storage-rpc"
		SoftwareVersion    string                 `json:"software_version"`
		IsRegistration     bool                   `json:"is_registration"`
	}
)
type NodeUnfrozenEvent registry.NodeUnfrozenEvent

// .................... RootHash  ....................

type ExecutorCommittedEvent struct {
	NodeID *signature.PublicKey `json:"node_id,omitempty"` // Available starting in Damask.
}

// .................... Governance ....................

type (
	ProposalSubmittedEvent governance.ProposalSubmittedEvent
	ProposalExecutedEvent  governance.ProposalExecutedEvent
	ProposalFinalizedEvent governance.ProposalFinalizedEvent
	VoteEvent              struct {
		ID        uint64          `json:"id"`
		Submitter staking.Address `json:"submitter"`
		Vote      string          `json:"vote"` // enum: "yes", "no", "abstain"
	}
)

// .................... Scheduler ....................

type (
	Validator scheduler.Validator
	Committee scheduler.Committee
)

// .................... Governance ....................

type Proposal governance.Proposal

// ....................................................
// ....................  Runtimes  ....................
// ....................................................

// Like ConsensusApiLite, but for the runtime API.
type RuntimeApiLite interface {
	GetEventsRaw(ctx context.Context, round uint64) ([]*RuntimeEvent, error)
	EVMSimulateCall(ctx context.Context, round uint64, gasPrice []byte, gasLimit uint64, caller []byte, address []byte, value []byte, data []byte) ([]byte, error)
	GetBlockHeader(ctx context.Context, round uint64) (*RuntimeBlockHeader, error)
	GetTransactionsWithResults(ctx context.Context, round uint64) ([]*RuntimeTransactionWithResults, error)
}

type (
	RuntimeEvent                  sdkTypes.Event
	RuntimeTransactionWithResults sdkClient.TransactionWithResults
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
