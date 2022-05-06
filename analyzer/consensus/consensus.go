// Package consensus implements an analyzer for the consensus layer.
package consensus

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/oasisprotocol/oasis-core/go/consensus/api/transaction"
	"github.com/oasisprotocol/oasis-core/go/consensus/api/transaction/results"
	"golang.org/x/sync/errgroup"

	"github.com/oasislabs/oasis-block-indexer/go/analyzer"
	"github.com/oasislabs/oasis-block-indexer/go/log"
	"github.com/oasislabs/oasis-block-indexer/go/storage"
)

const (
	chainID = "oasis_3"

	analyzerName = "consensus_main_damask_v1"
)

var (
	// ErrRangeOverlapError is returned if this analyzer is given a new processing
	// range that is not disjoint from one of its existing processing ranges.
	ErrRangeOverlapError = errors.New("invalid range provided. ranges must be disjoint")

	// ErrRangeNotFound is returned if the current block does not fall within any
	// of the analyzer's known ranges.
	ErrRangeNotFound = errors.New("range not found. no data source available.")
)

// ConsensusMain is the main Analyzer for the consensus layer.
type ConsensusMain struct {
	rangeCfg analyzer.RangeConfig
	target   storage.TargetStorage
	logger   *log.Logger
}

// NewConsensusMain returns a new analyzer for the consensus layer.
func NewConsensusMain(target storage.TargetStorage, logger *log.Logger) *ConsensusMain {
	return &ConsensusMain{
		target: target,
		logger: logger.With("analyzer", analyzerName),
	}
}

// SetRange adds configuration for the range of blocks to process to
// this analyzer. It is intended to be called before Start.
func (c *ConsensusMain) SetRange(cfg analyzer.RangeConfig) error {
	c.rangeCfg = cfg

	return nil
}

// Start starts the main consensus analyzer.
func (c *ConsensusMain) Start() {
	ctx := context.Background()

	for {
		if err := c.processNextBlock(ctx); err != nil {
			if err == ErrRangeNotFound {
				c.logger.Info("processing complete for known ranges")
				return
			}

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

// source returns the source storage for the provided block height.
func (c *ConsensusMain) source(height int64) (storage.SourceStorage, error) {
	r := c.rangeCfg
	if height >= r.From && (height <= r.To || r.To == 0) {
		return r.Source, nil
	}

	return nil, ErrRangeNotFound
}

// latestBlock returns the latest block processed by the consensus analyzer.
func (c *ConsensusMain) latestBlock(ctx context.Context) (int64, error) {
	row, err := c.target.QueryRow(
		ctx,
		fmt.Sprintf(`
			SELECT height FROM %s.processed_blocks
				WHERE analyzer = $1
				ORDER BY height DESC
				LIMIT 1
		`, chainID),
		analyzerName,
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
	source, err := c.source(height)
	if err != nil {
		return err
	}

	data, err := source.BlockData(ctx, height)
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

// extractEventData extracts the type of an event.
//
// TODO: Eliminate this if possible.
func extractEventData(event *results.Event) (backend string, ty string, body []byte, err error) {
	if event.Staking != nil {
		backend = "staking"
		if event.Staking.Transfer != nil {
			ty = "transfer"
			body, err = json.Marshal(event.Staking.Transfer)
			return
		} else if event.Staking.Burn != nil {
			ty = "burn"
			body, err = json.Marshal(event.Staking.Burn)
			return
		} else if event.Staking.Escrow != nil {
			if event.Staking.Escrow.Add != nil {
				ty = "add"
				body, err = json.Marshal(event.Staking.Escrow.Add)
				return
			} else if event.Staking.Escrow.Take != nil {
				ty = "take"
				body, err = json.Marshal(event.Staking.Escrow.Take)
				return
			} else if event.Staking.Escrow.DebondingStart != nil {
				ty = "debonding_start"
				body, err = json.Marshal(event.Staking.Escrow.DebondingStart)
				return
			} else if event.Staking.Escrow.Reclaim != nil {
				ty = "reclaim"
				body, err = json.Marshal(event.Staking.Escrow.Reclaim)
				return
			}
		} else if event.Staking.AllowanceChange != nil {
			ty = "allowance_change"
			body, err = json.Marshal(event.Staking.AllowanceChange)
			return
		}
	} else if event.Registry != nil {
		backend = "registry"
		if event.Registry.RuntimeEvent != nil {
			ty = "runtime"
			body, err = json.Marshal(event.Registry.RuntimeEvent)
			return
		} else if event.Registry.EntityEvent != nil {
			ty = "entity"
			body, err = json.Marshal(event.Registry.EntityEvent)
			return
		} else if event.Registry.NodeEvent != nil {
			ty = "node"
			body, err = json.Marshal(event.Registry.NodeEvent)
			return
		} else if event.Registry.NodeUnfrozenEvent != nil {
			ty = "node_unfrozen"
			body, err = json.Marshal(event.Registry.NodeUnfrozenEvent)
			return
		}
	} else if event.RootHash != nil {
		backend = "roothash"
		if event.RootHash.ExecutorCommitted != nil {
			ty = "executor_committed"
			body, err = json.Marshal(event.RootHash.ExecutorCommitted)
			return
		} else if event.RootHash.ExecutionDiscrepancyDetected != nil {
			ty = "execution_discrepancy_detected"
			body, err = json.Marshal(event.RootHash.ExecutionDiscrepancyDetected)
			return
		} else if event.RootHash.Finalized != nil {
			ty = "finalized"
			body, err = json.Marshal(event.RootHash.Finalized)
			return
		}
	} else if event.Governance != nil {
		backend = "governance"
		if event.Governance.ProposalSubmitted != nil {
			ty = "proposal_submitted"
			body, err = json.Marshal(event.Governance.ProposalSubmitted)
			return
		} else if event.Governance.ProposalExecuted != nil {
			ty = "proposal_executed"
			body, err = json.Marshal(event.Governance.ProposalExecuted)
			return
		} else if event.Governance.ProposalFinalized != nil {
			ty = "proposal_finalized"
			body, err = json.Marshal(event.Governance.ProposalFinalized)
			return
		} else if event.Governance.Vote != nil {
			ty = "vote"
			body, err = json.Marshal(event.Governance.Vote)
			return
		}
	}

	return "", "", []byte{}, errors.New("unknown event type")
}
