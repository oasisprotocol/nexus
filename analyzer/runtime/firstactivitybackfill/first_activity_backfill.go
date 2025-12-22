// Package firstactivitybackfill implements the first_activity backfill analyzer.
// This analyzer backfills the first_activity field for runtime accounts that
// were created before the first_activity tracking was added.
package firstactivitybackfill

import (
	"context"
	"fmt"
	"time"

	"github.com/oasisprotocol/nexus/analyzer"
	"github.com/oasisprotocol/nexus/analyzer/item"
	"github.com/oasisprotocol/nexus/analyzer/queries"
	"github.com/oasisprotocol/nexus/common"
	"github.com/oasisprotocol/nexus/config"
	"github.com/oasisprotocol/nexus/log"
	"github.com/oasisprotocol/nexus/storage"
)

const (
	analyzerPrefix   = "first_activity_backfill"
	defaultBatchSize = 100
)

type processor struct {
	target storage.TargetStorage
	logger *log.Logger

	// Cursor state for the current batch.
	lastRuntime *common.Runtime
	lastAddress *string
}

var _ item.ItemProcessor[*accountItem] = (*processor)(nil)

type accountItem struct {
	Runtime               common.Runtime
	Address               string
	ComputedFirstActivity *time.Time // The computed first activity timestamp (from batched lookup).
	IsLast                bool       // True if this is the last item in the batch.
}

func NewAnalyzer(
	cfg config.ItemBasedAnalyzerConfig,
	target storage.TargetStorage,
	logger *log.Logger,
) (analyzer.Analyzer, error) {
	if cfg.BatchSize == 0 {
		cfg.BatchSize = defaultBatchSize
	}
	logger = logger.With("analyzer", analyzerPrefix)
	p := &processor{
		target: target,
		logger: logger,
	}
	return item.NewAnalyzer[*accountItem](
		analyzerPrefix,
		cfg,
		p,
		target,
		logger,
	)
}

func (p *processor) GetItems(ctx context.Context, limit uint64) ([]*accountItem, error) {
	// Fetch current cursor position.
	var lastRuntime *common.Runtime
	var lastAddress *string
	err := p.target.QueryRow(ctx, queries.RuntimeAccountFirstActivityBackfillGetCursor).Scan(&lastRuntime, &lastAddress)
	if err != nil {
		return nil, fmt.Errorf("fetching cursor: %w", err)
	}

	// Query accounts after cursor. Use separate queries to ensure index usage.
	var rows storage.QueryResults
	if lastRuntime == nil || lastAddress == nil {
		rows, err = p.target.Query(ctx, queries.RuntimeAccountFirstActivityBackfillAccountsStart, limit)
	} else {
		rows, err = p.target.Query(ctx, queries.RuntimeAccountFirstActivityBackfillAccountsNext, *lastRuntime, *lastAddress, limit)
	}
	if err != nil {
		return nil, fmt.Errorf("querying accounts: %w", err)
	}
	defer rows.Close()

	var items []*accountItem
	for rows.Next() {
		var it accountItem
		var firstActivity *time.Time // Scanned but unused; kept to match query columns.
		if err := rows.Scan(&it.Runtime, &it.Address, &firstActivity, &it.ComputedFirstActivity); err != nil {
			return nil, fmt.Errorf("scanning account: %w", err)
		}
		items = append(items, &it)
	}

	// Mark the last item and store cursor for update.
	if len(items) > 0 {
		items[len(items)-1].IsLast = true
		p.lastRuntime = &items[len(items)-1].Runtime
		p.lastAddress = &items[len(items)-1].Address
	}

	return items, nil
}

func (p *processor) ProcessItem(ctx context.Context, batch *storage.QueryBatch, item *accountItem) error {
	// Update first_activity using the pre-computed timestamp from the batched query.
	// The UPDATE query uses LEAST to ensure we keep the earlier timestamp if one exists.
	if item.ComputedFirstActivity != nil {
		batch.Queue(
			queries.RuntimeAccountFirstActivityBackfillUpdate,
			item.Runtime,
			item.Address,
			item.ComputedFirstActivity,
		)
	}

	// If this is the last item in the batch, update the cursor.
	if item.IsLast && p.lastRuntime != nil && p.lastAddress != nil {
		batch.Queue(
			queries.RuntimeAccountFirstActivityBackfillUpdateCursor,
			*p.lastRuntime,
			*p.lastAddress,
		)
	}

	return nil
}

func (p *processor) QueueLength(ctx context.Context) (int, error) {
	// Fetch current cursor position.
	var lastRuntime *common.Runtime
	var lastAddress *string
	if err := p.target.QueryRow(ctx, queries.RuntimeAccountFirstActivityBackfillGetCursor).Scan(&lastRuntime, &lastAddress); err != nil {
		return 0, fmt.Errorf("fetching cursor: %w", err)
	}

	// Check if there are more accounts after cursor. Use separate queries to ensure index usage.
	var hasMore int
	var err error
	if lastRuntime == nil || lastAddress == nil {
		err = p.target.QueryRow(ctx, queries.RuntimeAccountFirstActivityBackfillHasMoreStart).Scan(&hasMore)
	} else {
		err = p.target.QueryRow(ctx, queries.RuntimeAccountFirstActivityBackfillHasMoreNext, *lastRuntime, *lastAddress).Scan(&hasMore)
	}
	if err != nil {
		return 0, fmt.Errorf("checking remaining work: %w", err)
	}
	return hasMore, nil
}
