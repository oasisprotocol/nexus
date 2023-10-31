// Package storage defines storage interfaces.
package storage

import (
	"context"
	"strings"
	"sync"

	"github.com/jackc/pgx/v5"
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
	// in nexus tables. This is useful when inserting blockchain data out of order,
	// so that later blocks can refer to (yet unindexed) earlier blocks without violating constraints.
	DisableTriggersAndFKConstraints(ctx context.Context) error

	// EnableTriggersAndFKConstraints enables all triggers and foreign key constraints
	// in the given schema.
	// WARNING: This might enable triggers not explicitly disabled by DisableTriggersAndFKConstraints.
	// WARNING: This does not enforce/check contraints on rows that were inserted while triggers were disabled.
	EnableTriggersAndFKConstraints(ctx context.Context) error
}

// Postgres requires valid UTF-8 with no 0x00.
func SanitizeString(msg string) string {
	return strings.ToValidUTF8(strings.ReplaceAll(msg, "\x00", "?"), "?")
}
