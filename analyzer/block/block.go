// Package block implements the generic block based analyzer.
//
// Block based analyzer uses a BlockProcessor to process blocks and handles the
// common logic for queueing blocks and support for parallel processing.
package block

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/oasisprotocol/nexus/analyzer"
	"github.com/oasisprotocol/nexus/analyzer/queries"
	"github.com/oasisprotocol/nexus/analyzer/util"
	"github.com/oasisprotocol/nexus/config"
	"github.com/oasisprotocol/nexus/log"
	"github.com/oasisprotocol/nexus/storage"
)

const (
	// Timeout to process a block.
	processBlockTimeout = 61 * time.Second
	// Default number of blocks to be processed in a batch.
	defaultBatchSize = 1_000
	// Lock expire timeout for blocks (in minutes). Locked blocks not processed within
	// this time can be picked again. Keep strictly > 1; the analyzer stops processing
	// blocks before the lock expires, by a safety margin of 1 minute.
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
	// The implementation must commit processed blocks (update the analysis.processed_blocks record with processed_time timestamp).
	ProcessBlock(ctx context.Context, height uint64) error
	// FinalizeFastSync updates any data that was neglected during the indexing of blocks
	// up to and including `lastFastSyncHeight`, e.g. dead-reckoned balances.
	// It is intended to be used when an analyzer wishes to transition from fast-sync mode
	// into slow-sync mode. If the analyzer never used fast-sync and is starting from
	// height 0 in slow-sync mode, `lastFastSyncHeight` will be -1.
	// The method should be a no-op if all the blocks up to `height` have been analyzed
	// in slow-sync mode, assuming no bugs in dead reckoning code.
	FinalizeFastSync(ctx context.Context, lastFastSyncHeight int64) error
}

var _ analyzer.Analyzer = (*blockBasedAnalyzer)(nil)

type blockBasedAnalyzer struct {
	blockRange   config.BlockRange
	batchSize    uint64
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

// unlockBlocks unlocks the given blocks.
func (b *blockBasedAnalyzer) unlockBlocks(ctx context.Context, heights []uint64) {
	rows, err := b.target.Query(
		ctx,
		queries.UnlockBlocksForProcessing,
		b.analyzerName,
		heights,
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
			b.batchSize,
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
			b.batchSize,
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

// Returns info about already-processed blocks in the analyzer's configured range:
// - Whether the already-processed blocks form a contiguous range starting at "to".
// - The height of the highest already-processed block.
func (b *blockBasedAnalyzer) processedSubrangeInfo(ctx context.Context) (bool, int64, error) {
	var isContiguous bool
	var maxProcessedHeight int64
	if err := b.target.QueryRow(
		ctx,
		queries.ProcessedSubrangeInfo,
		b.analyzerName,
		b.blockRange.From,
		b.blockRange.To,
	).Scan(&isContiguous, &maxProcessedHeight); err != nil {
		return false, 0, err
	}
	return isContiguous, maxProcessedHeight, nil
}

// Returns true if the block at `height` has been processed in slow-sync mode.
func (b *blockBasedAnalyzer) isBlockProcessedBySlowSync(ctx context.Context, height int64) (bool, error) {
	var isProcessed bool
	if err := b.target.QueryRow(
		ctx,
		queries.IsBlockProcessedBySlowSync,
		b.analyzerName,
		height,
	).Scan(&isProcessed); err != nil {
		return false, err
	}
	return isProcessed, nil
}

// Finds any gaps in the range of already-processed blocks, and soft-enqueues them
// (i.e. adds entries for them in `analysis.processed_blocks`) if they overlap with this
// analyzer's configured range.
// This is useful if we're recovering from misconfigured past runs that left gaps in the
// range of processed blocks. The main block-grabbing loop assumes (for efficiency) that
// we only ever need to process blocks that are larger than the largest entry in the db,
// or else explicitly present in the db and marked as not completed. This function ensures
// that that assumption is valid even in the face of misconfigured past runs, e.g. if we
// processed the range [1000, 2000] but now want to process [1, infinity).
func (b *blockBasedAnalyzer) softEnqueueGapsInProcessedBlocks(ctx context.Context) error {
	batch := &storage.QueryBatch{}
	batch.Queue(
		queries.SoftEnqueueGapsInProcessedBlocks,
		b.analyzerName,
		b.blockRange.From,
		b.blockRange.To,
	)
	if err := b.target.SendBatch(ctx, batch); err != nil {
		b.logger.Error("failed to soft-enqueue gaps in already-processed blocks", "err", err, "from", b.blockRange.From, "to", b.blockRange.To)
		return err
	}
	b.logger.Info("ensured that any gaps in the already-processed block range can be picked up later", "from", b.blockRange.From, "to", b.blockRange.To)
	return nil
}

// Validates assumptions/prerequisites for starting a slow sync analyzer:
//   - No blocks in the configured [from, to] range have been processed, except possibly a contiguous
//     subrange [from, X] for some X.
//   - If the most recently processed block was not processed by slow-sync (i.e. by fast sync, or not
//     at all), triggers a finalization of the fast-sync process.
func (b *blockBasedAnalyzer) ensureSlowSyncPrerequisites(ctx context.Context) (ok bool) {
	isContiguous, maxProcessedHeight, err := b.processedSubrangeInfo(ctx)
	if err != nil {
		b.logger.Error("Failed to obtain info about already-processed blocks", "err", err)
		return false
	}
	if !isContiguous {
		b.logger.Error(fmt.Sprintf("cannot run in slow-sync mode because a non-contiguous subset of blocks in range [%d, %d] has already been processed. Use fast-sync to process to at least height %d.", b.blockRange.From, b.blockRange.To, maxProcessedHeight))
		return false
	}

	// If the block before the one we'll attempt has been processed by fast-sync or not at all,
	// we first refetch the full current state (i.e. genesis or similar).
	precededBySlowSync, err := b.isBlockProcessedBySlowSync(ctx, maxProcessedHeight)
	if err != nil {
		b.logger.Error("Failed to obtain info about the last processed block", "err", err, "last_processed_block", maxProcessedHeight)
		return false
	}
	if !precededBySlowSync {
		b.logger.Error("finalizing the work of previous fast-sync analyzer(s)", "last_fast_sync_height", maxProcessedHeight)
		if err := b.processor.FinalizeFastSync(ctx, maxProcessedHeight); err != nil {
			b.logger.Error("failed to finalize the fast-sync phase (i.e. download genesis or similar)", "err", err)
			return false
		}
		b.logger.Error("fast-sync finalization complete; proceeding with regular slow-sync analysis")
	}

	return true
}

// Start starts the block analyzer.
func (b *blockBasedAnalyzer) Start(ctx context.Context) {
	// Run prework.
	if err := b.processor.PreWork(ctx); err != nil {
		b.logger.Error("prework failed", "err", err)
		return
	}

	// The default max block height that the analyzer will process. This value is not
	// indicative of the maximum height the Oasis blockchain can reach; rather it
	// is set to golang's maximum int64 value for convenience.
	var to uint64 = math.MaxInt64
	// Clamp the latest block height to the configured range.
	if b.blockRange.To != 0 {
		to = b.blockRange.To
	}

	if b.slowSync && !b.ensureSlowSyncPrerequisites(ctx) {
		// We cannot continue or recover automatically. Logging happens inside the validate function.
		return
	}

	if !b.slowSync && b.softEnqueueGapsInProcessedBlocks(ctx) != nil {
		// We cannot continue or recover automatically. Logging happens inside the validate function.
		return
	}

	// Start processing blocks.
	backoff, err := util.NewBackoff(
		100*time.Millisecond,
		6*time.Second, // cap the timeout at the expected consensus block time
	)
	if err != nil {
		b.logger.Error("error configuring backoff policy",
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
		// The context for processing the batch of blocks is shorter than the lock expiry.
		// This is to ensure that the batch is processed before the locks expire.
		batchCtx, batchCtxCancel = context.WithTimeout(ctx, (lockExpiryMinutes-1)*time.Minute)

		// Pick a batch of blocks to process.
		b.logger.Info("picking a batch of blocks to process", "from", b.blockRange.From, "to", to, "is_fast_sync", !b.slowSync)
		heights, err := b.fetchBatchForProcessing(ctx, b.blockRange.From, to)
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
			// If running in slow-sync, we are likely at the tip of the chain and are picking up
			// blocks that are not yet available. In this case, wait before processing every block,
			// so that the backoff mechanism can tweak the per-block wait time as needed.
			//
			// Note: If the batch size is greater than 50, the time required to process the blocks
			// in the batch will exceed the current lock expiry of 5min. The analyzer will terminate
			// the batch early and attempt to refresh the locks for a new batch.
			if b.slowSync {
				select {
				case <-time.After(backoff.Timeout()):
					// Process the next block
				case <-batchCtx.Done():
					b.logger.Info("batch locks expiring; refreshing batch")
					b.unlockBlocks(ctx, heights) // Locks are _about_ to expire, but are not expired yet. Unlock explicitly so blocks can be grabbed sooner.
					break
				case <-ctx.Done():
					batchCtxCancel()
					b.logger.Warn("shutting down block analyzer", "reason", ctx.Err())
					b.unlockBlocks(ctx, heights) // Give others a chance to process our blocks even before their locks implicitly expire
					b.logger.Info("unlocked db rows", "heights", heights)
					return
				}
			}
			b.logger.Info("processing block", "height", height)

			bCtx, cancel := context.WithTimeout(batchCtx, processBlockTimeout)
			if err := b.processor.ProcessBlock(bCtx, height); err != nil {
				cancel()
				backoff.Failure()

				if err == analyzer.ErrOutOfRange {
					b.logger.Info("no data available; will retry",
						"height", height,
						"retry_interval_ms", backoff.Timeout().Milliseconds(),
					)
				} else {
					b.logger.Error("error processing block", "height", height, "err", err)
				}

				// If running in slow-sync, stop processing the batch on error so that
				// the blocks are always processed in order.
				if b.slowSync {
					break
				}

				// Unlock a failed block, so it can be retried sooner.
				b.unlockBlocks(ctx, []uint64{height})
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
		if len(heights) == 0 && b.blockRange.To != 0 {
			if height, err := b.firstUnprocessedBlock(ctx); err == nil && height > b.blockRange.To {
				break
			}
		}
	}
	batchCtxCancel()

	b.logger.Info(
		"finished processing all blocks in the configured range",
		"from", b.blockRange.From, "to", b.blockRange.To,
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
	blockRange config.BlockRange,
	batchSize uint64,
	mode analyzer.BlockAnalysisMode,
	name string,
	processor BlockProcessor,
	target storage.TargetStorage,
	logger *log.Logger,
) (analyzer.Analyzer, error) {
	if batchSize == 0 {
		batchSize = defaultBatchSize
	}
	return &blockBasedAnalyzer{
		blockRange:   blockRange,
		batchSize:    batchSize,
		analyzerName: name,
		processor:    processor,
		target:       target,
		logger:       logger.With("analyzer", name, "mode", mode),
		slowSync:     mode == analyzer.SlowSyncMode,
	}, nil
}
