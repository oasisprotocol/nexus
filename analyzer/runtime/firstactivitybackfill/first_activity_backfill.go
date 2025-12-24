// Package firstactivitybackfill implements the first_activity backfill analyzer.
// This analyzer backfills the first_activity field for runtime accounts that
// were created before the first_activity tracking was added.
package firstactivitybackfill

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

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
	defaultBatchSize = 1000
)

type processor struct {
	target storage.TargetStorage
	logger *log.Logger
}

var _ item.ItemProcessor[*accountItem] = (*processor)(nil)

type accountItem struct {
	Runtime common.Runtime
	Address string
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
	rows, err := p.target.Query(ctx, queries.RuntimeAccountFirstActivityBackfillAccounts, limit)
	if err != nil {
		return nil, fmt.Errorf("querying accounts: %w", err)
	}
	defer rows.Close()

	var items []*accountItem
	for rows.Next() {
		var item accountItem
		if err := rows.Scan(&item.Runtime, &item.Address); err != nil {
			return nil, fmt.Errorf("scanning account: %w", err)
		}
		items = append(items, &item)
	}
	return items, nil
}

func (p *processor) ProcessItem(ctx context.Context, batch *storage.QueryBatch, item *accountItem) error {
	// Query the first activity timestamp for this account.
	var timestamp *time.Time
	err := p.target.QueryRow(
		ctx,
		queries.RuntimeAccountFirstActivityBackfillTimestamp,
		item.Runtime,
		item.Address,
	).Scan(&timestamp)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// The account has related transactions (per GetItems query) but no matching
			// runtime_transactions row. This indicates data skew or partial restore.
			// Log a warning and mark as done to avoid infinite retries.
			p.logger.Warn("account has related transactions but no matching runtime_transaction",
				"runtime", item.Runtime, "address", item.Address)
			batch.Queue(
				queries.RuntimeAccountFirstActivityBackfillMarkDone,
				item.Runtime,
				item.Address,
			)
			return nil
		}
		// Real DB error - propagate it.
		return fmt.Errorf("querying first activity timestamp: %w", err)
	}

	// Update the first_activity for this account.
	batch.Queue(
		queries.RuntimeAccountFirstActivityBackfillUpdate,
		item.Runtime,
		item.Address,
		timestamp,
	)

	return nil
}

func (p *processor) QueueLength(ctx context.Context) (int, error) {
	var count int
	if err := p.target.QueryRow(ctx, queries.RuntimeAccountFirstActivityBackfillQueueLength).Scan(&count); err != nil {
		return 0, fmt.Errorf("querying queue length: %w", err)
	}
	return count, nil
}
