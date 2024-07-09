package accounts_list

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/oasisprotocol/nexus/analyzer"
	"github.com/oasisprotocol/nexus/analyzer/item"
	"github.com/oasisprotocol/nexus/config"
	"github.com/oasisprotocol/nexus/log"
	"github.com/oasisprotocol/nexus/storage"
	"github.com/oasisprotocol/nexus/storage/oasis/nodeapi"
)

const (
	analyzerName = "consensus_account_list"

	defaultInterval = 2 * time.Minute

	accountListViewRefreshQuery = `REFRESH MATERIALIZED VIEW CONCURRENTLY views.accounts_list`

	accountListStatusQuery = `
		SELECT
			COALESCE((SELECT computed_height FROM views.accounts_list LIMIT 1), 0) AS computed_height,
			b.height AS latest_block_height
		FROM
			chain.blocks b
		ORDER BY
			b.height DESC
		LIMIT 1`
)

type processor struct {
	source nodeapi.ConsensusApiLite
	target storage.TargetStorage
	logger *log.Logger
}

var _ item.ItemProcessor[struct{}] = (*processor)(nil)

func NewAnalyzer(
	cfg config.ItemBasedAnalyzerConfig,
	client nodeapi.ConsensusApiLite,
	target storage.TargetStorage,
	logger *log.Logger,
) (analyzer.Analyzer, error) {
	if cfg.Interval == 0 {
		cfg.Interval = defaultInterval
	}
	logger = logger.With("analyzer", analyzerName)
	p := &processor{
		source: client,
		target: target,
		logger: logger,
	}

	return item.NewAnalyzer(
		analyzerName,
		cfg,
		p,
		target,
		logger,
	)
}

func (p *processor) GetItems(ctx context.Context, limit uint64) ([]struct{}, error) {
	return []struct{}{{}}, nil
}

func (p *processor) ProcessItem(ctx context.Context, batch *storage.QueryBatch, item struct{}) error {
	batch.Queue(accountListViewRefreshQuery)
	return nil
}

func (p *processor) QueueLength(ctx context.Context) (int, error) {
	var latestViewHeight, latestHeight uint64
	err := p.target.QueryRow(ctx, accountListStatusQuery).Scan(&latestViewHeight, &latestHeight)
	switch {
	case err == nil:
		// Continues below.
	case errors.Is(err, pgx.ErrNoRows):
		// If there are no blocks, return non-empty queue so that analyzer won't get shut-down
		// due to inactivity even before the first block is processed.
		return 1, nil
	default:
		return 0, fmt.Errorf("failed to get latest account view status: %w", err)
	}

	// If there are new blocks, we have "1" item in queue.
	if latestViewHeight < latestHeight {
		return 1, nil
	}
	return 0, nil
}
