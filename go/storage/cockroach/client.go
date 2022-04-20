// Package cockroach implements the target storage interface
// backed by CockroachDB.
package cockroach

import (
	"context"

	"github.com/cockroachdb/cockroach-go/v2/crdb/crdbpgx"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"

	"github.com/oasislabs/oasis-block-indexer/go/log"
	"github.com/oasislabs/oasis-block-indexer/go/oasis-indexer/cmd/common"
)

const (
	moduleName = "cockroach"
)

type CockroachClient struct {
	pool   *pgxpool.Pool
	logger *log.Logger
}

// NewCockroachClient creates a new CockroachDB client.
func NewCockroachClient(connString string) (*CockroachClient, error) {
	logger := common.Logger().WithModule(moduleName)

	pool, err := pgxpool.Connect(context.Background(), connString)
	if err != nil {
		return nil, err
	}
	return &CockroachClient{pool, logger}, nil
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

// Name returns the name of the CockroachDB client.
func (c *CockroachClient) Name() string {
	return moduleName
}
