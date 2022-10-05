// Package postgres implements the target storage interface
// backed by PostgreSQL.
package postgres

import (
	"context"

	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"

	"github.com/oasisprotocol/oasis-indexer/log"
)

const (
	moduleName = "postgres"
)

// Client is a client for connecting to PostgreSQL.
type Client struct {
	pool   *pgxpool.Pool
	logger *log.Logger
}

// NewClient creates a new PostgreSQL client.
func NewClient(connString string, l *log.Logger) (*Client, error) {
	config, err := pgxpool.ParseConfig(connString)
	if err != nil {
		return nil, err
	}

	pool, err := pgxpool.ConnectConfig(context.Background(), config)
	if err != nil {
		return nil, err
	}
	return &Client{
		pool:   pool,
		logger: l.WithModule(moduleName),
	}, nil
}

// SendBatch submits a new batch of queries as an atomic transaction to PostgreSQL.
//
// For now, updated row counts are discarded as this is not intended to be used
// by any indexer. We only care about atomic success or failure of the batch of queries
// corresponding to a new block.
func (c *Client) SendBatch(ctx context.Context, batch *pgx.Batch) error {
	if err := c.pool.BeginTxFunc(ctx, pgx.TxOptions{}, func(tx pgx.Tx) error {
		batchResults := tx.SendBatch(ctx, batch)
		defer batchResults.Close()
		for i := 0; i < batch.Len(); i++ {
			if _, err := batchResults.Exec(); err != nil {
				return err
			}
		}

		return nil
	}); err != nil {
		c.logger.Error("failed to execute db batch",
			"error", err,
			"batch_size", batch.Len(),
		)
		return err
	}

	return nil
}

// Query submits a new read query to PostgreSQL.
func (c *Client) Query(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error) {
	rows, err := c.pool.Query(ctx, sql, args...)
	if err != nil {
		c.logger.Error("failed to query db",
			"error", err,
		)
		return nil, err
	}
	return rows, nil
}

// QueryRow submits a new read query for a single row to PostgreSQL.
func (c *Client) QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row {
	return c.pool.QueryRow(ctx, sql, args...)
}

// Shutdown implements the storage.TargetStorage interface for Client.
func (c *Client) Shutdown() {
	c.pool.Close()
}

// Name implements the storage.TargetStorage interface for Client.
func (c *Client) Name() string {
	return moduleName
}
