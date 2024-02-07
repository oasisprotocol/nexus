// Package item implements the generic item based analyzer.
//
// Item based analyzer uses a ItemProcessor to process work items
// and handles the common logic for fetching work items and processing
// them in parallel.
package item

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/oasisprotocol/nexus/analyzer"
	"github.com/oasisprotocol/nexus/analyzer/util"
	"github.com/oasisprotocol/nexus/config"
	"github.com/oasisprotocol/nexus/log"
	"github.com/oasisprotocol/nexus/metrics"
	"github.com/oasisprotocol/nexus/storage"
)

const (
	// Timeout to process a single batch.
	processBatchTimeout = 61 * time.Second
	// Default number of items processed in a batch.
	defaultBatchSize = 20
)

// ErrEmptyBatch is returned by the item based analyzer when there
// are no work items in the batch.
var ErrEmptyBatch = errors.New("no items in batch")

type itemBasedAnalyzer[Item any] struct {
	maxBatchSize        uint64
	stopIfQueueEmptyFor time.Duration
	fixedInterval       time.Duration
	interItemDelay      time.Duration
	analyzerName        string

	processor ItemProcessor[Item]

	target  storage.TargetStorage
	logger  *log.Logger
	metrics metrics.AnalysisMetrics
}

var _ analyzer.Analyzer = (*itemBasedAnalyzer[any])(nil)

type ItemProcessor[Item any] interface {
	// GetItems fetches the next batch of work items.
	GetItems(ctx context.Context, limit uint64) ([]Item, error)
	// ProcessItem processes a single item, retrieving all required information
	// from source storage or out of band and committing the resulting batch
	// of queries to target storage.
	ProcessItem(ctx context.Context, batch *storage.QueryBatch, item Item) error
	// QueueLength returns the number of total items in the work queue. This
	// is currently used for observability metrics.
	QueueLength(ctx context.Context) (int, error)
}

// NewAnalyzer returns a new item based analyzer using the provided item processor.
//
// If stopIfQueueEmptyFor is a non-zero duration, the analyzer will process batches of items until its
// work queue is empty for `stopIfQueueEmptyFor`, at which point it will terminate and return. Likely to
// be used in the regression tests.
//
// If fixedInterval is provided, the analyzer will process one batch every fixedInterval.
// By default, the analyzer will use a backoff mechanism that will attempt to run as
// fast as possible until encountering an error.
func NewAnalyzer[Item any](
	name string,
	cfg config.ItemBasedAnalyzerConfig,
	processor ItemProcessor[Item],
	target storage.TargetStorage,
	logger *log.Logger,
) (analyzer.Analyzer, error) {
	if cfg.BatchSize == 0 {
		cfg.BatchSize = defaultBatchSize
	}
	a := &itemBasedAnalyzer[Item]{
		maxBatchSize:        cfg.BatchSize,
		stopIfQueueEmptyFor: cfg.StopIfQueueEmptyFor,
		fixedInterval:       cfg.Interval,
		interItemDelay:      cfg.InterItemDelay,
		analyzerName:        name,
		processor:           processor,
		target:              target,
		logger:              logger,
		metrics:             metrics.NewDefaultAnalysisMetrics(name),
	}

	return a, nil
}

// sendQueueLength reports the current number of items in the work queue to Prometheus.
func (a *itemBasedAnalyzer[Item]) sendQueueLengthMetric(ctx context.Context) (int, error) {
	queueLength, err := a.processor.QueueLength(ctx)
	if err != nil {
		a.logger.Warn("error fetching queue length", "err", err)
		return 0, err
	}
	a.metrics.QueueLength(a.analyzerName).Set(float64(queueLength))
	return queueLength, nil
}

// processBatch fetches the next batch of work items, processes them in parallel, and
// commits the resulting state changes to the database. Note that if the analyzer fails
// to process one or more items in a batch, the entire batch is discarded and no database
// updates are applied.
func (a *itemBasedAnalyzer[Item]) processBatch(ctx context.Context) (int, error) {
	// Fetch the batch.
	items, err := a.processor.GetItems(ctx, a.maxBatchSize)
	if err != nil {
		return 0, fmt.Errorf("error fetching work items: %w", err)
	}
	a.logger.Info("processing", "num_items", len(items))
	if len(items) == 0 {
		return 0, nil
	}

	// Process the items in parallel.
	batchCtx, batchCancel := context.WithCancel(ctx)
	defer batchCancel()
	var wg sync.WaitGroup
	batches := make([]*storage.QueryBatch, len(items))
	errors := make([]error, len(items))

	for i, it := range items {
		wg.Add(1)
		go func(idx int, item Item) {
			defer wg.Done()
			batches[idx] = &storage.QueryBatch{} // initialize here to avoid nil entries.
			batch := storage.QueryBatch{}
			if err := a.processor.ProcessItem(batchCtx, &batch, item); err != nil {
				a.logger.Error("failed to process item", "item", item, "err", err)
				errors[idx] = err
				return
			}
			batches[idx] = &batch
		}(i, it)
		time.Sleep(a.interItemDelay)
	}

	batchDone := util.ClosingChannel(&wg)
	select {
	case <-time.After(processBatchTimeout):
		a.logger.Warn("timed out waiting for batch items to process")
		// note: we do not return here because we do not want to block successfully processed items from being added.
		batchCancel()
	case <-batchDone:
	}

	// Commit the changes from all successfully processed items to the database.
	batch := &storage.QueryBatch{}
	for _, b := range batches {
		batch.Extend(b)
	}
	if err := a.target.SendBatch(ctx, batch); err != nil {
		return 0, fmt.Errorf("sending batch: %w", err)
	}

	// Process errors.
	numErrs, firstErr := processErrors(errors)

	return len(items) - numErrs, firstErr
}

// Helper function that counts the number of errors and returns the first one if any.
func processErrors(errs []error) (int, error) {
	count := 0
	var firstErr error
	for _, e := range errs {
		if e != nil {
			count++
			if firstErr == nil {
				firstErr = e
			}
		}
	}

	return count, firstErr
}

// Start starts the item based analyzer.
func (a *itemBasedAnalyzer[Item]) Start(ctx context.Context) {
	backoff, err := util.NewBackoff(
		100*time.Millisecond,
		// Cap the timeout at the expected consensus block time
		6*time.Second,
	)
	if err != nil {
		a.logger.Error("error configuring backoff policy",
			"err", err.Error(),
		)
		return
	}
	mostRecentTask := time.Now()

	for firstIter := true; ; firstIter = false {
		delay := backoff.Timeout()
		if a.fixedInterval != 0 {
			delay = a.fixedInterval
		}
		if firstIter {
			delay = 0 // Don't sleep before first iteration.
		}
		select {
		case <-time.After(delay):
			// Process another batch of items.
		case <-ctx.Done():
			a.logger.Warn("shutting down item analyzer", "reason", ctx.Err())
			return
		}
		// Update queueLength
		queueLength, err := a.sendQueueLengthMetric(ctx)
		// Stop if queue has been empty for a while, and configured to do so.
		if err == nil && queueLength == 0 && a.stopIfQueueEmptyFor != 0 && time.Since(mostRecentTask) > a.stopIfQueueEmptyFor {
			a.logger.Warn("item analyzer work queue has been empty for a while; shutting down",
				"queue_empty_since", mostRecentTask,
				"queue_empty_for", time.Since(mostRecentTask),
				"stop_if_queue_empty_for", a.stopIfQueueEmptyFor)
			return
		}
		a.logger.Info("work queue length", "num_items", queueLength)

		numProcessed, err := a.processBatch(ctx)
		if err != nil {
			a.logger.Error("error processing batch", "err", err)
			backoff.Failure()
			continue
		}
		if numProcessed == 0 {
			// Count this as a failure to reduce the polling when we are
			// running faster than the block analyzer can find new tokens.
			backoff.Failure()
			continue
		}
		mostRecentTask = time.Now()

		backoff.Success()
	}
}

func (a *itemBasedAnalyzer[Item]) Name() string {
	return a.analyzerName
}
