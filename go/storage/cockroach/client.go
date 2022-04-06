// Package cockroach implements the target storage interface
// backed by CockroachDB.
package cockroach

import (
	"context"

	"github.com/jackc/pgx/v4/pgxpool"

	"github.com/oasislabs/oasis-block-indexer/go/storage"
)

const (
	clientName = "cockroach"
)

type CockroachClient struct {
	pool *pgx.ConnPool
}

// NewCockroachClient creates a new CockroachDB client.
func NewCockroachClient(connstring string) (*CockroachClient, error) {
	pool, err := pgxpool.Connect(context.Background(), connstring)
	if err != nil {
		return nil, err
	}
	return &CockroachClient{pool}, nil
}

// SetBlockData applies the block data as an update at the provided block height.
func (c *CockroachClient) SetBlockData(data *storage.BlockData) error {
	conn, err = c.pool.Acquire(context.Background())
	if err != nil {
		return err
	}
	defer c.pool.Release(conn)

	return nil
}

// SetBeaconData applies the beacon data as an update at the provided block height.
func (c *CockroachClient) SetBeaconData(data *storage.BeaconData) error {
	conn, err = c.pool.Acquire(context.Background())
	if err != nil {
		return err
	}
	defer c.pool.Release(conn)

	return nil
}

// SetRegistryData applies the registry data as an update at the provided block height.
func (c *CockroachClient) SetRegistryData(data *storage.RegistryData) error {
	conn, err = c.pool.Acquire(context.Background())
	if err != nil {
		return err
	}
	defer c.pool.Release(conn)

	return nil
}

// SetStakingData applies the staking data as an update at the provided block height.
func (c *CockroachClient) SetStakingData(data *storage.StakingData) error {
	conn, err = c.pool.Acquire(context.Background())
	if err != nil {
		return err
	}
	defer c.pool.Release(conn)

	return nil
}

// SetSchedulerData applies the scheduler data as an update at the provided block height.
func (c *CockroachClient) SetSchedulerData(data *storage.SchedulerData) error {
	conn, err = c.pool.Acquire(context.Background())
	if err != nil {
		return err
	}
	defer c.pool.Release(conn)

	return nil
}

// SetGovernanceData applies the governance data as an update at the provided block height.
func (c *CockroachClient) SetGovernanceData(data *storage.BeaconData) error {
	conn, err = c.pool.Acquire(context.Background())
	if err != nil {
		return err
	}
	defer c.pool.Release(conn)

	return nil
}

// Name returns the name of the CockroachDB client.
func (c *CockroachClient) Name() string {
	return clientName
}

// TODO: Cleanup method to gracefully shut down client.
