// Package block implements the generic block based analyzer.
//
// Block based analyzer uses a BlockProcessor to process blocks and handles the
// common logic for queueing blocks and support for parallel processing.
package block

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/oasisprotocol/nexus/analyzer"
	"github.com/oasisprotocol/nexus/analyzer/queries"
	"github.com/oasisprotocol/nexus/analyzer/util"
	"github.com/oasisprotocol/nexus/config"
	"github.com/oasisprotocol/nexus/log"
	"github.com/oasisprotocol/nexus/metrics"
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
	// backoffMaxTimeout is the maximum timeout for the backoff policy in case of errors.
	backoffMaxTimeout = 30 * time.Second
	// headPollingInterval is the interval at which the analyzer will poll for new blocks
	// when at the head of the chain.
	headPollingInterval = 1 * time.Second

	// If last successfully processed block was more than 12 seconds ago, the error is likely
	// NOT due to being at the head of the chain.
	headPollingThreshold = 12 * time.Second
)

// BlockProcessor is the interface that block-based processors should implement to use them with the
// block based analyzer.
type BlockProcessor interface {
	// PreWork performs tasks that need to be done before the main processing loop starts.
	// In parallel mode, this method is called by a single instance.
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

	target  storage.TargetStorage
	logger  *log.Logger
	metrics metrics.AnalysisMetrics

	lastProcessedBlock time.Time

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
	if b.slowSync {
		// In slow-sync mode, locks are ignored, as this should be the only analyzer running.
		return
	}
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
	b.logger.Info("fast-sync: ensuring that any gaps in the already-processed block range can be picked up later", "from", b.blockRange.From, "to", b.blockRange.To)
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
	b.logger.Info("any potential gaps resolved", "from", b.blockRange.From, "to", b.blockRange.To)
	return nil
}

// Validates assumptions/prerequisites for starting a slow sync analyzer:
//   - No blocks in the configured [from, to] range have been processed, except possibly a contiguous
//     subrange [from, X] for some X.
//   - If the most recently processed block was not processed by slow-sync (i.e. by fast sync, or not
//     at all), triggers a finalization of the fast-sync process.
func (b *blockBasedAnalyzer) ensureSlowSyncPrerequisites(ctx context.Context) (ok bool) {
	b.logger.Info("slow sync: checking prerequisites: checking that blocks processed so far form a contiguous range")
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
		b.logger.Info("finalizing the work of previous fast-sync analyzer(s)", "last_fast_sync_height", maxProcessedHeight)
		if err := b.processor.FinalizeFastSync(ctx, maxProcessedHeight); err != nil {
			b.logger.Error("failed to finalize the fast-sync phase (i.e. download genesis or similar)", "err", err)
			return false
		}
		b.logger.Info("fast-sync finalization complete; proceeding with regular slow-sync analysis")
	}

	return true
}

// sendQueueLengthMetric updates the relevant Prometheus metric with an approximation
// of the number of known blocks that require processing, calculated as the difference
// between the analyzer's highest processed block and the current block height
// on-chain (fetched by the node-stats analyzer).
// Note that the true count may be higher during fast-sync.
func (b *blockBasedAnalyzer) sendQueueLengthMetric(queueLength uint64) {
	// For block-based analyzers, the analyzer name is identical to the layer name.
	b.metrics.QueueLength(b.analyzerName).Set(float64(queueLength))
}

// Returns the chain height of the layer this analyzer is processing.
// The heights are fetched and added to the database by the node-stats analyzer.
func (b *blockBasedAnalyzer) nodeHeight(ctx context.Context) (int, error) {
	var nodeHeight int
	err := b.target.QueryRow(ctx, queries.NodeHeight, b.analyzerName).Scan(&nodeHeight)
	switch err {
	case nil:
		return nodeHeight, nil
	case pgx.ErrNoRows:
		return -1, nil
	default:
		return -1, fmt.Errorf("error fetching chain height for consensus: %w", err)
	}
}

// Returns true if `ctx` is about to expire within `margin`, or has already expired.
func expiresWithin(ctx context.Context, margin time.Duration) bool {
	if ctx.Err() != nil {
		// The context has expired.
		return true
	}
	deadline, hasDeadline := ctx.Deadline()
	if !hasDeadline {
		return false
	}
	return time.Until(deadline) < margin
}

func (b *blockBasedAnalyzer) PreWork(ctx context.Context) error {
	// Run processor-specific pre-work first.
	err := b.processor.PreWork(ctx)
	if err != nil {
		return fmt.Errorf("processor pre-work failed: %w", err)
	}

	// Run general block-based analyzer pre-work.
	if b.slowSync && !b.ensureSlowSyncPrerequisites(ctx) {
		// We cannot continue or recover automatically. Logging happens inside the validate function.
		return fmt.Errorf("failed to validate prerequisites for slow-sync mode")
	}

	if !b.slowSync && b.softEnqueueGapsInProcessedBlocks(ctx) != nil {
		// We cannot continue or recover automatically. Logging happens inside the validate function.
		return fmt.Errorf("failed to soft-enqueue gaps in already-processed blocks")
	}

	return nil
}

// Start starts the block analyzer.
func (b *blockBasedAnalyzer) Start(ctx context.Context) {
	// The default max block height that the analyzer will process. This value is not
	// indicative of the maximum height the Oasis blockchain can reach; rather it
	// is set to golang's maximum int64 value for convenience.
	var to uint64 = math.MaxInt64
	// Clamp the latest block height to the configured range.
	if b.blockRange.To != 0 {
		to = b.blockRange.To
	}

	// Start processing blocks.
	backoff, err := util.NewBackoff(
		100*time.Millisecond,
		backoffMaxTimeout,
	)
	if err != nil {
		b.logger.Error("error configuring backoff policy",
			"err", err.Error(),
		)
		return
	}

	// timeout returns the timeout for the next batch of blocks.
	nextBatchTimeout := func(err error) time.Duration {
		// If we reached out of range and have recently processed a block, poll again soon.
		if b.isRecentOutOfRangeError(err) {
			return headPollingInterval
		}
		// Otherwise, use the backoff policy.
		return backoff.Timeout()
	}

	var batchErr error
	for {
		timeout := nextBatchTimeout(batchErr)
		select {
		case <-time.After(timeout):
			// Process another batch of blocks.
		case <-ctx.Done():
			b.logger.Warn("shutting down block analyzer", "reason", ctx.Err())
			return
		}

		// Pick a batch of blocks to process.
		b.logger.Info("picking a batch of blocks to process", "from", b.blockRange.From, "to", to, "is_fast_sync", !b.slowSync, "timeout", timeout)
		heights, err := b.fetchBatchForProcessing(ctx, b.blockRange.From, to)
		if err != nil {
			b.logger.Error("failed to pick blocks for processing",
				"err", err,
			)
			backoff.Failure()
			continue
		}
		b.logger.Debug("picked blocks for processing", "heights", heights)

		// The context for processing the batch of blocks is shorter than the lock expiry.
		// This is to ensure that the batch is processed before the locks expire.
		batchCtx, batchCtxCancel := context.WithTimeout(ctx, (lockExpiryMinutes-1)*time.Minute)
		var numProcessed int
		numProcessed, batchErr = b.processBatch(batchCtx, heights)
		if batchErr != nil {
			b.unlockBlocks(ctx, heights) // Unlock the blocks so they can be picked up sooner.
		}
		batchCtxCancel()

		// Backoff update.
		// TODO: Think about success if some blocks were processed.
		switch {
		case len(heights) == 0:
			// No blocks processed, increase the backoff timeout a bit.
			b.logger.Info("no blocks to process")
			backoff.Failure()
		case batchErr != nil && !b.isRecentOutOfRangeError(batchErr) && numProcessed == 0:
			// Not likely an error due being at the head of the chain, increase the backoff timeout.
			// If some blocks were processed, do not increase the backoff timer.
			backoff.Failure()
		default:
			backoff.Success()
		}

		// Stop processing if end height is set and was reached.
		if len(heights) == 0 && b.blockRange.To != 0 {
			if height, err := b.firstUnprocessedBlock(ctx); err == nil && height > b.blockRange.To {
				break
			}
		}
	}

	b.logger.Info(
		"finished processing all blocks in the configured range",
		"from", b.blockRange.From, "to", b.blockRange.To,
	)
}

func (b *blockBasedAnalyzer) processBatch(ctx context.Context, heights []uint64) (int, error) {
	processed := 0
	nodeHeight := -1
	for _, height := range heights {
		// If the context is close to expiring, do not attempt more blocks and unlock the blocks.
		if expiresWithin(ctx, processBlockTimeout) {
			b.unlockBlocks(ctx, heights)
			return processed, nil
		}

		if b.slowSync || nodeHeight == -1 {
			// In slow-sync mode, we update the node-height at each block for a more
			// precise measurement.
			var err error
			nodeHeight, err = b.nodeHeight(ctx)
			if err != nil {
				b.logger.Warn("error fetching current node height: %w", err)
			}
		}

		b.logger.Info("processing block", "height", height)
		bCtx, cancel := context.WithTimeout(ctx, processBlockTimeout)
		if err := b.processor.ProcessBlock(bCtx, height); err != nil {
			cancel()

			// Unlock the failed block, so it can be retried sooner.
			b.unlockBlocks(ctx, []uint64{height})

			// Log the error only if it is not an out of range error.
			if errors.Is(err, analyzer.ErrOutOfRange) {
				b.logger.Info("no data available", "err", err, "height", height)
			} else {
				b.logger.Error("failed to process block", "err", err, "height", height)
			}

			// If running in slow-sync, stop processing the batch on first error
			// so that the blocks are always processed in order.
			if b.slowSync {
				return processed, err
			}
			continue
		}
		cancel()

		b.logger.Info("processed block", "height", height)
		processed++
		b.lastProcessedBlock = time.Now()
		// If we successfully fetched the node height earlier, update the estimated queue length.
		if nodeHeight != -1 {
			b.sendQueueLengthMetric(uint64(nodeHeight) - height)
		}
	}

	return processed, nil
}

// isRecentOutOfRangeError returns true if the error is an out of range error and
// the last processed block was recent, therfore we're likely just at the head of the chain.
func (b *blockBasedAnalyzer) isRecentOutOfRangeError(err error) bool {
	return errors.Is(err, analyzer.ErrOutOfRange) && time.Since(b.lastProcessedBlock) < headPollingThreshold
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
	a := &blockBasedAnalyzer{
		blockRange:         blockRange,
		batchSize:          batchSize,
		analyzerName:       name,
		processor:          processor,
		target:             target,
		logger:             logger.With("analyzer", name, "mode", mode),
		metrics:            metrics.NewDefaultAnalysisMetrics(name),
		lastProcessedBlock: time.Now(),
		slowSync:           mode == analyzer.SlowSyncMode,
	}

	return a, nil
}
