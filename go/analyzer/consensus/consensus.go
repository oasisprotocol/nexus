// Package consensus implements an analyzer for the consensus layer.
package consensus

import (
	"context"
	"fmt"
	"time"

	"github.com/oasisprotocol/oasis-core/go/consensus/api/transaction"
	"golang.org/x/sync/errgroup"

	"github.com/oasislabs/oasis-block-indexer/go/log"
	"github.com/oasislabs/oasis-block-indexer/go/storage"
)

const (
	chainID = "oasis_3"

	analyzerName = "consensus-main"
)

// ConsensusMain is the main Analyzer for the consensus layer.
type ConsensusMain struct {
	source storage.SourceStorage
	target storage.TargetStorage
	logger *log.Logger
}

// NewConsensusMain returns a new analyzer for the consensus layer.
func NewConsensusMain(source storage.SourceStorage, target storage.TargetStorage, logger *log.Logger) *ConsensusMain {
	return &ConsensusMain{
		source: source,
		target: target,
		logger: logger.With("analyzer", analyzerName),
	}
}

// Start starts the ConsensusMain.
func (c *ConsensusMain) Start() {
	ctx := context.Background()

	for {
		if err := c.processNextBlock(ctx); err != nil {
			c.logger.Error("error processing block",
				"err", err.Error(),
			)
			time.Sleep(1 * time.Second)
			continue
		}
	}
}

// Name returns the name of the ConsensusMain.
func (c *ConsensusMain) Name() string {
	return analyzerName
}

// latestBlock returns the latest block processed by the consensus analyzer.
func (c *ConsensusMain) latestBlock(ctx context.Context) (int64, error) {
	row, err := c.target.QueryRow(
		ctx,
		fmt.Sprintf(`
			SELECT height FROM %s.processed_blocks
				ORDER BY height DESC
				LIMIT 1
		`, chainID),
		// analyzerName,
	)
	if err != nil {
		return 0, err
	}

	var latest int64
	if err := row.Scan(&latest); err != nil {
		return 0, err
	}
	return latest, nil
}

// processNextBlock processes the block at the provided block height.
func (c *ConsensusMain) processNextBlock(ctx context.Context) error {
	// Get block to be indexed.
	latest, err := c.latestBlock(ctx)
	if err != nil {
		return err
	}
	height := latest + 1

	c.logger.Info("processing block",
		"height", height,
	)

	group, groupCtx := errgroup.WithContext(ctx)

	// Prepare and perform updates.
	batch := &storage.QueryBatch{}

	type prepareFunc = func(context.Context, int64, *storage.QueryBatch) error
	for _, f := range []prepareFunc{
		c.prepareBlockData,
		// TODO: Other network data.
	} {
		func(f prepareFunc) {
			group.Go(func() error {
				if err := f(groupCtx, height, batch); err != nil {
					return err
				}
				return nil
			})
		}(f)
	}

	// Update indexing progress.
	group.Go(func() error {
		batch.Queue(fmt.Sprintf(`
			INSERT INTO %s.processed_blocks (height, analyzer, processed_time)
			VALUES
				($1, $2, CURRENT_TIMESTAMP);
		`, chainID),
			height,
			analyzerName,
		)
		return nil
	})

	if err := group.Wait(); err != nil {
		return err
	}

	return c.target.SendBatch(ctx, batch)
}

// prepareBlockData adds block data queries to the batch.
func (c *ConsensusMain) prepareBlockData(ctx context.Context, height int64, batch *storage.QueryBatch) error {
	data, err := c.source.BlockData(ctx, height)
	if err != nil {
		return err
	}

	for _, f := range []func(*storage.QueryBatch, *storage.BlockData) error{
		c.queueBlockInserts,
		c.queueTransactionInserts,
		c.queueEventInserts,
	} {
		if err := f(batch, data); err != nil {
			return err
		}
	}

	return nil
}

func (c *ConsensusMain) queueBlockInserts(batch *storage.QueryBatch, data *storage.BlockData) error {
	batch.Queue(fmt.Sprintf(`
		INSERT INTO %s.blocks (height, block_hash, time, namespace, version, type, root_hash)
			VALUES ($1, $2, $3, $4, $5, $6, $7);
	`, chainID),
		data.BlockHeader.Height,
		data.BlockHeader.Hash.Hex(),
		data.BlockHeader.Time,
		data.BlockHeader.StateRoot.Namespace.String(),
		int64(data.BlockHeader.StateRoot.Version),
		data.BlockHeader.StateRoot.Type.String(),
		data.BlockHeader.StateRoot.Hash.Hex(),
	)

	return nil
}

func (c *ConsensusMain) queueTransactionInserts(batch *storage.QueryBatch, data *storage.BlockData) error {
	for i := range data.Transactions {
		signedTx := data.Transactions[i]
		result := data.Results[i]

		var tx transaction.Transaction
		if err := signedTx.Open(&tx); err != nil {
			continue
		}

		batch.Queue(fmt.Sprintf(`
			INSERT INTO %s.transactions (block, txn_hash, txn_index, nonce, fee_amount, max_gas, method, body, module, code, message)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11);
		`, chainID),
			data.BlockHeader.Height,
			signedTx.Hash().Hex(),
			i,
			tx.Nonce,
			tx.Fee.Amount.ToBigInt().Uint64(),
			tx.Fee.Gas,
			tx.Method,
			tx.Body,
			result.Error.Module,
			result.Error.Code,
			result.Error.Message,
		)
	}

	return nil
}

func (c *ConsensusMain) queueEventInserts(batch *storage.QueryBatch, data *storage.BlockData) error {
	for i := 0; i < len(data.Results); i++ {
		for j := 0; j < len(data.Results[i].Events); j++ {
			backend, ty, body, err := extractEventData(data.Results[i].Events[j])
			if err != nil {
				return err
			}

			batch.Queue(fmt.Sprintf(`
				INSERT INTO %s.events (backend, type, body, txn_block, txn_hash, txn_index)
					VALUES ($1, $2, $3, $4, $5, $6);
			`, chainID),
				backend,
				ty,
				string(body),
				data.BlockHeader.Height,
				data.Transactions[i].Hash().Hex(),
				i,
			)
		}
	}

	return nil
}
