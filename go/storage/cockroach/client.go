// Package cockroach implements the target storage interface
// backed by CockroachDB.
package cockroach

import (
	"context"
	"fmt"

	"github.com/cockroachdb/cockroach-go/v2/crdb/crdbpgx"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
)

const (
	clientName = "cockroach"
)

// CockroachClient is a client for connecting to a CockroachDB cluster.
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
	fmt.Println("sending batch")
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

// Name returns the name of the CockroachDB client.
func (c *CockroachClient) Name() string {
	return clientName
}
