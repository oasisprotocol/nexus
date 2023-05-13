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
	sdkTypes "github.com/oasisprotocol/oasis-sdk/client-sdk/go/types"

	"github.com/oasisprotocol/oasis-indexer/storage/oasis/nodeapi"
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

// Tx represents a database transaction.
type Tx = pgx.Tx

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

	// LatestBlockHeight returns the latest height for which a block is available.
	LatestBlockHeight(ctx context.Context) (int64, error)

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

	// Close instructs the source storage to clean up resources. Calling other
	// methods after this one results in undefined behavior.
	Close() error
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
	TransactionsWithResults []nodeapi.TransactionWithResults
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

	Events                  []nodeapi.Event
	RuntimeRegisteredEvents []nodeapi.RuntimeRegisteredEvent
	EntityEvents            []nodeapi.EntityEvent
	NodeEvents              []nodeapi.NodeEvent
	NodeUnfrozenEvents      []nodeapi.NodeUnfrozenEvent
}

// StakingData represents data for accounts at a given height.
//
// Note: The staking backend supports getting events directly. We support
// retrieving events as updates to apply when getting data at specific height.
type StakingData struct {
	Height int64
	Epoch  beacon.EpochTime

	Events                []nodeapi.Event
	Transfers             []nodeapi.TransferEvent
	Burns                 []nodeapi.BurnEvent
	AddEscrows            []nodeapi.AddEscrowEvent
	TakeEscrows           []nodeapi.TakeEscrowEvent
	ReclaimEscrows        []nodeapi.ReclaimEscrowEvent
	DebondingStartEscrows []nodeapi.DebondingStartEscrowEvent
	AllowanceChanges      []nodeapi.AllowanceChangeEvent
}

// RootHashData represents data for runtime processing at a given height.
type RootHashData struct {
	Height int64

	Events []nodeapi.Event
}

// SchedulerData represents data for elected committees and validators at a given height.
type SchedulerData struct {
	Height int64

	Validators []nodeapi.Validator
	Committees map[common.Namespace][]nodeapi.Committee
}

// GovernanceData represents governance data for proposals at a given height.
//
// Note: The governance backend supports getting events directly. We support
// retrieving events as updates to apply when getting data at a specific height.
type GovernanceData struct {
	Height int64

	Events                []nodeapi.Event
	ProposalSubmissions   []nodeapi.Proposal
	ProposalExecutions    []nodeapi.ProposalExecutedEvent
	ProposalFinalizations []nodeapi.Proposal
	Votes                 []nodeapi.VoteEvent
}

// RuntimeSourceStorage defines an interface for retrieving raw block data
// from the runtime layer.
type RuntimeSourceStorage interface {
	// AllData returns all data tied to a specific round.
	AllData(ctx context.Context, round uint64) (*RuntimeAllData, error)

	// LatestBlockHeight returns the latest height for which a block is available.
	LatestBlockHeight(ctx context.Context) (uint64, error)

	// EVMSimulateCall gets the result of the given EVM simulate call query.
	EVMSimulateCall(ctx context.Context, round uint64, gasPrice []byte, gasLimit uint64, caller []byte, address []byte, value []byte, data []byte) ([]byte, error)

	// Close instructs the source storage to clean up resources. Calling other
	// methods after this one results in undefined behavior.
	Close() error
}

type RuntimeAllData struct {
	Round                   uint64
	BlockHeader             nodeapi.RuntimeBlockHeader
	RawEvents               []nodeapi.RuntimeEvent
	TransactionsWithResults []nodeapi.RuntimeTransactionWithResults
}

// TransactionWithResults contains a verified transaction, and the results of
// executing that transactions.
type TransactionWithResults struct {
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

	// Begin starts a new transaction.
	// XXX: Not the nicest that this exposes the underlying pgx.Tx interface. Could instead
	// return a `TargetStorage`-like interface wrapper, that only exposes Query/QueryRow/SendBatch/SendBatchWithOptions
	// and Commit/Rollback.
	Begin(ctx context.Context) (Tx, error)

	// Close shuts down the target storage client.
	Close()

	// Name returns the name of the target storage.
	Name() string

	// Wipe removes all contents of the target storage.
	Wipe(ctx context.Context) error

	// DisableTriggersAndFKConstraints disables all triggers and foreign key constraints
	// in indexer tables. This is useful when inserting blockchain data out of order,
	// so that later blocks can refer to (yet unindexed) earlier blocks without violating constraints.
	DisableTriggersAndFKConstraints(ctx context.Context) error

	// EnableTriggersAndFKConstraints enables all triggers and foreign key constraints
	// in the given schema.
	// WARNING: This might enable triggers not explicitly disabled by DisableTriggersAndFKConstraints.
	// WARNING: This does not enforce/check contraints on rows that were inserted while triggers were disabled.
	EnableTriggersAndFKConstraints(ctx context.Context) error
}
