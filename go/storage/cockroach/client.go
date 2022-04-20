// Package cockroach implements the target storage interface
// backed by CockroachDB.
package cockroach

import (
	"context"

	"github.com/cockroachdb/cockroach-go/v2/crdb/crdbpgx"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
)

const (
	moduleName = "cockroach"
)

type CockroachClient struct {
	pool *pgxpool.Pool
}

// NewCockroachClient creates a new CockroachDB client.
func NewCockroachClient(connString string) (*CockroachClient, error) {
	pool, err := pgxpool.Connect(context.Background(), connString)
	if err != nil {
		return nil, err
	}
	return &CockroachClient{pool}, nil
}

// SendBatch submits a new transaction batch to CockroachDB.
func (c *CockroachClient) SendBatch(ctx context.Context, batch *pgx.Batch) error {
	conn, err := c.pool.Acquire(ctx)
	if err != nil {
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
		return err
	}

	return nil
}

// Query submits a new query to CockroachDB.
func (c *CockroachClient) Query(ctx context.Context, sql string, args ...interface{}) (*pgx.Rows, error) {
	conn, err := c.pool.Acquire(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Release()

	rows, err := conn.Query(ctx, sql, args)
	if err != nil {
		return nil, err
	}
	return &rows, nil
}

// Name returns the name of the CockroachDB client.
func (c *CockroachClient) Name() string {
	return moduleName
}
