// Package block implements the generic block based analyzer.
//
// Block based analyzer uses a BlockProcessor to process blocks and handles the
// common logic for queueing blocks and support for parallel processing.
package block

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/oasisprotocol/oasis-indexer/analyzer"
	"github.com/oasisprotocol/oasis-indexer/analyzer/queries"
	"github.com/oasisprotocol/oasis-indexer/analyzer/util"
	"github.com/oasisprotocol/oasis-indexer/config"
	"github.com/oasisprotocol/oasis-indexer/log"
	"github.com/oasisprotocol/oasis-indexer/storage"
)

const (
	// Timeout to process a block.
	processBlockTimeout = 61 * time.Second
	// Number of blocks to be processed in a batch.
	blocksBatchSize = 100
	// Lock expire timeout for blocks (in minutes). Locked blocks not processed within
	// this time can be picked again.
	lockExpiryMinutes = 5
)

// BlockProcessor is the interface that block-based processors should implement to use them with the
// block based analyzer.
type BlockProcessor interface {
	// PreWork performs tasks that need to be done before the main processing loop starts.
	PreWork(ctx context.Context) error
	// ProcessBlock processes the provided block, retrieving all required information
	// from source storage and committing an atomically-executed batch of queries
	// to target storage.
	//
	// The implementation must commit processed blocks (update the chain.processed_blocks record with processed_time timestamp).
	ProcessBlock(ctx context.Context, height uint64) error
	// SourceLatestBlockHeight returns the latest block height available in the source storage.
	SourceLatestBlockHeight(ctx context.Context) (uint64, error)
}

var _ analyzer.Analyzer = (*blockBasedAnalyzer)(nil)

type blockBasedAnalyzer struct {
	config       *config.BlockBasedAnalyzerConfig
	analyzerName string

	processor BlockProcessor

	target storage.TargetStorage
	logger *log.Logger

	slowSync bool
}

// firstUnprocessedBlock returns the first block before which all blocks have been processed.
// If no blocks have been processed, it returns error pgx.ErrNoRows.
func (b *blockBasedAnalyzer) firstUnprocessedBlock(ctx context.Context) (first uint64, err error) {
	err = b.target.QueryRow(
		ctx,
		queries.FirstUnprocessedBlock,
		b.analyzerName,
	).Scan(&first)
	return
}

// unlockBlock unlocks a block.
func (b *blockBasedAnalyzer) unlockBlock(ctx context.Context, height uint64) {
	rows, err := b.target.Query(
		ctx,
		queries.UnlockBlockForProcessing,
		b.analyzerName,
		height,
	)
	if err == nil {
		rows.Close()
	}
}

// fetchBatchForProcessing fetches (and locks) a batch of blocks for processing.
func (b *blockBasedAnalyzer) fetchBatchForProcessing(ctx context.Context, from uint64, to uint64) ([]uint64, error) {
	// XXX: In future, use a system for picking lock IDs in case other parts of the code start using advisory locks.
	const lockID = 1001
	var (
		tx      storage.Tx
		heights []uint64
		rows    pgx.Rows
		err     error
	)

	// Start a transaction.
	tx, err = b.target.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("starting transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Pick an advisory lock for the fetch batch query.
	if rows, err = tx.Query(
		ctx,
		queries.TakeXactLock,
		lockID,
	); err != nil {
		return nil, fmt.Errorf("taking advisory lock: %w", err)
	}
	rows.Close()

	switch b.slowSync {
	case true:
		// If running in slow-sync mode, ignore locks as this should be the only instance
		// of the analyzer running.
		rows, err = tx.Query(
			ctx,
			queries.PickBlocksForProcessing,
			b.analyzerName,
			from,
			to,
			0,
			blocksBatchSize,
		)
	case false:
		// Fetch and lock blocks for processing.
		rows, err = tx.Query(
			ctx,
			queries.PickBlocksForProcessing,
			b.analyzerName,
			from,
			to,
			lockExpiryMinutes,
			blocksBatchSize,
		)
	}
	if err != nil {
		return nil, fmt.Errorf("querying blocks for processing: %w", err)
	}

	defer rows.Close()
	for rows.Next() {
		var height uint64
		if err = rows.Scan(
			&height,
		); err != nil {
			return nil, fmt.Errorf("scanning returned height: %w", err)
		}
		heights = append(heights, height)
	}

	if err = tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("committing transaction: %w", err)
	}

	return heights, nil
}

// Start starts the block analyzer.
func (b *blockBasedAnalyzer) Start(ctx context.Context) {
	// Run prework.
	if err := b.processor.PreWork(ctx); err != nil {
		b.logger.Error("prework failed", "err", err)
		return
	}

	// Start processing blocks.
	backoff, err := util.NewBackoff(
		100*time.Millisecond,
		6*time.Second, // cap the timeout at the expected consensus block time
	)
	if err != nil {
		b.logger.Error("error configuring indexer backoff policy",
			"err", err.Error(),
		)
		return
	}

	var (
		batchCtx       context.Context
		batchCtxCancel context.CancelFunc = func() {}
	)
	for {
		batchCtxCancel()
		select {
		case <-time.After(backoff.Timeout()):
			// Process another batch of blocks.
		case <-ctx.Done():
			b.logger.Warn("shutting down block analyzer", "reason", ctx.Err())
			return
		}
		batchCtx, batchCtxCancel = context.WithTimeout(ctx, lockExpiryMinutes*time.Minute)

		var to uint64
		// Get the latest available block on the source.
		latestBlockHeight, err := b.processor.SourceLatestBlockHeight(ctx)
		if err != nil {
			b.logger.Error("failed to query latest block height on source",
				"err", err,
			)
			backoff.Failure()
			continue
		}
		to = latestBlockHeight

		// Clamp the latest block height to the configured range.
		if b.config.To != 0 && b.config.To < latestBlockHeight {
			to = b.config.To
		}

		// Pick a batch of blocks to process.
		b.logger.Info("picking a batch of blocks to process", "from", b.config.From, "to", to)
		heights, err := b.fetchBatchForProcessing(ctx, b.config.From, to)
		if err != nil {
			b.logger.Error("failed to pick blocks for processing",
				"err", err,
			)
			backoff.Failure()
			continue
		}

		// Process blocks.
		b.logger.Debug("picked blocks for processing", "heights", heights)
		for _, height := range heights {
			b.logger.Info("processing block", "height", height)

			bCtx, cancel := context.WithTimeout(batchCtx, processBlockTimeout)
			if err := b.processor.ProcessBlock(bCtx, height); err != nil {
				cancel()
				backoff.Failure()
				b.logger.Error("error processing block", "height", height, "err", err)

				// If running in slow-sync, stop processing the batch on error so that
				// the blocks are always processed in order.
				if b.slowSync {
					break
				}

				// Unlock a failed block, so it can be retried sooner.
				// TODO: Could add a hook to unlock all remaining blocks in the batch on graceful shutdown.
				b.unlockBlock(ctx, height)
				continue
			}
			cancel()
			backoff.Success()
			b.logger.Info("processed block", "height", height)
		}

		if len(heights) == 0 {
			b.logger.Info("no blocks to process")
			backoff.Failure() // No blocks processed, increase the backoff timeout a bit.
		}

		// Stop processing if end height is set and was reached.
		if len(heights) == 0 && b.config.To != 0 {
			if height, err := b.firstUnprocessedBlock(ctx); err == nil && height > b.config.To {
				break
			}
		}
	}
	batchCtxCancel()

	b.logger.Info(
		"finished processing all blocks in the configured range",
		"from", b.config.From, "to", b.config.To,
	)
}

// Name returns the name of the analyzer.
func (b *blockBasedAnalyzer) Name() string {
	return b.analyzerName
}

// NewAnalyzer returns a new block based analyzer for the provided block processor.
//
// slowSync is a flag that indicates that the analyzer is running in slow-sync mode and it should
// process blocks in order, ignoring locks as it is assumed it is the only analyzer running.
func NewAnalyzer(
	config *config.BlockBasedAnalyzerConfig,
	name string,
	processor BlockProcessor,
	target storage.TargetStorage,
	logger *log.Logger,
	slowSync bool,
) (analyzer.Analyzer, error) {
	return &blockBasedAnalyzer{
		config:       config,
		analyzerName: name,
		processor:    processor,
		target:       target,
		logger:       logger.With("analyzer", name, "slow_sync", slowSync),
		slowSync:     slowSync,
	}, nil
}
