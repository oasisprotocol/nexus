// Package cockroach implements the target storage interface
// backed by CockroachDB.
package cockroach

import (
	"context"

	"github.com/cockroachdb/cockroach-go/v2/crdb/crdbpgx"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"

	"github.com/oasislabs/oasis-indexer/log"
)

const (
	moduleName = "cockroach"
)

var defaultMaxConns = int32(32)

// Client is a CockroachDB client.
type Client struct {
	pool   *pgxpool.Pool
	logger *log.Logger
}

// NewClient creates a new CockroachDB client.
func NewClient(connString string, l *log.Logger) (*Client, error) {
	config, err := pgxpool.ParseConfig(connString)
	if err != nil {
		return nil, err
	}
	config.MaxConns = defaultMaxConns

	pool, err := pgxpool.ConnectConfig(context.Background(), config)
	if err != nil {
		return nil, err
	}
	return &Client{
		pool:   pool,
		logger: l.WithModule(moduleName),
	}, nil
}

// SendBatch submits a new transaction batch to CockroachDB.
func (c *Client) SendBatch(ctx context.Context, batch *pgx.Batch) error {
	if err := crdbpgx.ExecuteTx(ctx, c.pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		batchResults := tx.SendBatch(ctx, batch)
		defer batchResults.Close()
		for i := 0; i < batch.Len(); i++ {
			if _, err := batchResults.Exec(); err != nil {
				return err
			}
		}

		return nil
	}); err != nil {
		c.logger.Error("failed to execute tx batch",
			"error", err,
		)
		return err
	}

	return nil
}

// Query submits a new query to CockroachDB.
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

// QueryRow submits a new query for a single row to CockroachDB.
func (c *Client) QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row {
	return c.pool.QueryRow(ctx, sql, args...)
}

// Shutdown shuts down the target storage client.
func (c *Client) Shutdown() {
	c.pool.Close()
}

// Name returns the name of the CockroachDB client.
func (c *Client) Name() string {
	return moduleName
}
