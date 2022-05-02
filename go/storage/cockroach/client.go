// Package cockroach implements the target storage interface
// backed by CockroachDB.
package cockroach

import (
	"context"

	"github.com/cockroachdb/cockroach-go/v2/crdb/crdbpgx"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"

	"github.com/oasislabs/oasis-block-indexer/go/log"
)

const (
	moduleName = "cockroach"
)

type CockroachClient struct {
	pool   *pgxpool.Pool
	logger *log.Logger
}

// NewCockroachClient creates a new CockroachDB client.
func NewCockroachClient(connString string, l *log.Logger) (*CockroachClient, error) {
	config, err := pgxpool.ParseConfig(connString)
	if err != nil {
		return nil, err
	}

	pool, err := pgxpool.ConnectConfig(context.Background(), config)
	if err != nil {
		return nil, err
	}
	return &CockroachClient{
		pool:   pool,
		logger: l.WithModule(moduleName),
	}, nil
}

// SendBatch submits a new transaction batch to CockroachDB.
func (c *CockroachClient) SendBatch(ctx context.Context, batch *pgx.Batch) error {
	conn, err := c.pool.Acquire(ctx)
	if err != nil {
		c.logger.Error("failed to acquire db connection from pool",
			"error", err,
		)
		return err
	}
	defer conn.Release()

	if err := crdbpgx.ExecuteTx(ctx, conn, pgx.TxOptions{}, func(tx pgx.Tx) error {
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
func (c *CockroachClient) Query(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error) {
	conn, err := c.pool.Acquire(ctx)
	if err != nil {
		c.logger.Error("failed to acquire db connection from pool",
			"error", err,
		)
		return nil, err
	}
	defer conn.Release()

	rows, err := conn.Query(ctx, sql, args...)
	if err != nil {
		c.logger.Error("failed to query db",
			"error", err,
		)
		return nil, err
	}
	return rows, nil
}

// QueryRow submits a new query for a single row to CockroachDB.
func (c *CockroachClient) QueryRow(ctx context.Context, sql string, args ...interface{}) (pgx.Row, error) {
	conn, err := c.pool.Acquire(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Release()

	return conn.QueryRow(ctx, sql, args...), nil
}

// Name returns the name of the CockroachDB client.
func (c *CockroachClient) Name() string {
	return moduleName
}
