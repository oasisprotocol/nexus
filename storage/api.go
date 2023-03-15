// Package storage defines storage interfaces.
package storage

import (
	"context"
	"sync"

	"github.com/jackc/pgx/v5"

	beacon "github.com/oasisprotocol/oasis-core/go/beacon/api"
	"github.com/oasisprotocol/oasis-core/go/common"
	consensus "github.com/oasisprotocol/oasis-core/go/consensus/api"
	genesisAPI "github.com/oasisprotocol/oasis-core/go/genesis/api"
	governance "github.com/oasisprotocol/oasis-core/go/governance/api"
	registry "github.com/oasisprotocol/oasis-core/go/registry/api"
	roothash "github.com/oasisprotocol/oasis-core/go/roothash/api"
	"github.com/oasisprotocol/oasis-core/go/roothash/api/block"
	scheduler "github.com/oasisprotocol/oasis-core/go/scheduler/api"
	staking "github.com/oasisprotocol/oasis-core/go/staking/api"
	"github.com/oasisprotocol/oasis-indexer/storage/oasis/nodeapi"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/client"
	sdkTypes "github.com/oasisprotocol/oasis-sdk/client-sdk/go/types"
)

type BatchItem struct {
	Cmd  string
	Args []interface{}
}

// QueryBatch represents a batch of queries to be executed atomically.
// We use a custom type that mirrors `pgx.Batch`, but is thread-safe to use and
// allows introspection for debugging.
type QueryBatch struct {
	items []*BatchItem
	mu    sync.Mutex
}

// QueryResults represents the results from a read query.
type QueryResults = pgx.Rows

// QueryResult represents the result from a read query.
type QueryResult = pgx.Row

// TxOptions encodes the way DB transactions are executed.
type TxOptions = pgx.TxOptions

// Queue adds query to a batch.
func (b *QueryBatch) Queue(cmd string, args ...interface{}) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.items = append(b.items, &BatchItem{
		Cmd:  cmd,
		Args: args,
	})
}

// Extend merges another batch into the current batch.
func (b *QueryBatch) Extend(qb *QueryBatch) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if qb != b {
		qb.mu.Lock()
		defer qb.mu.Unlock()
	}

	b.items = append(b.items, qb.items...)
}

// Len returns the number of queries in the batch.
func (b *QueryBatch) Len() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.items)
}

// AsPgxBatch converts a QueryBatch to a pgx.Batch.
func (b *QueryBatch) AsPgxBatch() pgx.Batch {
	b.mu.Lock()
	defer b.mu.Unlock()
	pgxBatch := pgx.Batch{}
	for _, item := range b.items {
		pgxBatch.Queue(item.Cmd, item.Args...)
	}
	return pgxBatch
}

// Queries returns the queries in the batch. Each item of the returned slice
// is composed of the SQL command and its arguments.
func (b *QueryBatch) Queries() []*BatchItem {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.items
}

// ConsensusSourceStorage defines an interface for retrieving raw block data
// from the consensus layer.
type ConsensusSourceStorage interface {
	// GenesisDocument returns the genesis document for the chain.
	GenesisDocument(ctx context.Context) (*genesisAPI.Document, error)

	// AllData returns all data tied to a specific height.
	AllData(ctx context.Context, height int64) (*ConsensusAllData, error)

	// BlockData gets block data at the specified height. This includes all
	// block header information, as well as transactions and events included
	// within that block.
	BlockData(ctx context.Context, height int64) (*ConsensusBlockData, error)

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

	// RootHashData gets root hash data at the specified height. This includes
	// root hash events.
	RootHashData(ctx context.Context, height int64) (*RootHashData, error)

	// Name returns the name of the source storage.
	Name() string
}

type ConsensusAllData struct {
	BlockData      *ConsensusBlockData
	BeaconData     *BeaconData
	RegistryData   *RegistryData
	RootHashData   *RootHashData
	StakingData    *StakingData
	SchedulerData  *SchedulerData
	GovernanceData *GovernanceData
}

// ConsensusBlockData represents data for a consensus block at a given height.
type ConsensusBlockData struct {
	Height int64

	BlockHeader             *consensus.Block
	Epoch                   beacon.EpochTime
	TransactionsWithResults []*nodeapi.TransactionWithResults
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

	Events             []*registry.Event
	RuntimeEvents      []*registry.RuntimeEvent
	EntityEvents       []*registry.EntityEvent
	NodeEvents         []*registry.NodeEvent
	NodeUnfrozenEvents []*registry.NodeUnfrozenEvent
}

// StakingData represents data for accounts at a given height.
//
// Note: The staking backend supports getting events directly. We support
// retrieving events as updates to apply when getting data at specific height.
type StakingData struct {
	Height int64
	Epoch  beacon.EpochTime

	Events           []*staking.Event
	Transfers        []*staking.TransferEvent
	Burns            []*staking.BurnEvent
	Escrows          []*staking.EscrowEvent
	AllowanceChanges []*staking.AllowanceChangeEvent
}

// RootHashData represents data for runtime processing at a given height.
type RootHashData struct {
	Height int64

	Events []*roothash.Event
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

	Events                []*governance.Event
	ProposalSubmissions   []*governance.Proposal
	ProposalExecutions    []*governance.ProposalExecutedEvent
	ProposalFinalizations []*governance.Proposal
	Votes                 []*governance.VoteEvent
}

// RuntimeSourceStorage defines an interface for retrieving raw block data
// from the runtime layer.
type RuntimeSourceStorage interface {
	// AllData returns all data tied to a specific round.
	AllData(ctx context.Context, round uint64) (*RuntimeAllData, error)

	// BlockData gets block data in the specified round. This includes all
	// block header information, as well as transactions and events included
	// within that block.
	BlockData(ctx context.Context, round uint64) (*RuntimeBlockData, error)

	// EventsData gets all events in the specified round, including non-tx events.
	GetEventsRaw(ctx context.Context, round uint64) ([]*sdkTypes.Event, error)

	// EVMSimulateCall gets the result of the given EVM simulate call query.
	EVMSimulateCall(ctx context.Context, round uint64, gasPrice []byte, gasLimit uint64, caller []byte, address []byte, value []byte, data []byte) ([]byte, error)

	// Name returns the name of the source storage.
	Name() string

	// StringifyDenomination returns a string representation of the given denomination.
	// This is simply the denomination's symbol; notably, for the native denomination,
	// this is looked up from network config.
	StringifyDenomination(d sdkTypes.Denomination) string
}

type RuntimeAllData struct {
	Round     uint64
	BlockData *RuntimeBlockData
	RawEvents []*sdkTypes.Event
}

// RuntimeBlockData represents data for a runtime block during a given round.
type RuntimeBlockData struct {
	Round uint64

	BlockHeader             *block.Block
	TransactionsWithResults []*client.TransactionWithResults
}

// TransactionWithResults contains a verified transaction, and the results of
// executing that transactions.
type TransactionWithResults struct {
	Round uint64

	Tx     *sdkTypes.Transaction
	Result sdkTypes.CallResult
	Events []*sdkTypes.Event
}

// TargetStorage defines an interface for reading and writing
// processed block data.
type TargetStorage interface {
	// SendBatch sends a batch of queries to be applied to target storage.
	SendBatch(ctx context.Context, batch *QueryBatch) error

	// SendBatchWithOptions is like SendBatch, with custom DB options (e.g. level of tx isolation).
	SendBatchWithOptions(ctx context.Context, batch *QueryBatch, opts TxOptions) error

	// Query submits a query to fetch data from target storage.
	Query(ctx context.Context, sql string, args ...interface{}) (QueryResults, error)

	// QueryRow submits a query to fetch a single row of data from target storage.
	QueryRow(ctx context.Context, sql string, args ...interface{}) QueryResult

	// Shutdown shuts down the target storage client.
	Shutdown()

	// Name returns the name of the target storage.
	Name() string

	// Wipe removes all contents of the target storage.
	Wipe(ctx context.Context) error
}
