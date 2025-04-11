package consensus_accounts_list

import (
	"context"
	"time"

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
	accountTxCountsRefreshQuery = `REFRESH MATERIALIZED VIEW CONCURRENTLY views.accounts_tx_counts`

	accountTxLastRefreshQuery = `
		SELECT
 	 	  COALESCE((SELECT MAX(height) FROM chain.blocks), 0) AS latest_block,
   		  COALESCE((SELECT computed_height FROM views.accounts_tx_counts LIMIT 1), 0) AS computed_height`
)

type processor struct {
	source nodeapi.ConsensusApiLite
	target storage.TargetStorage
	logger *log.Logger

	transactionCountBlocksInterval uint64
}

var _ item.ItemProcessor[struct{}] = (*processor)(nil)

func NewAnalyzer(
	cfg config.ConsensusAccountsListAnalyzerConfig,
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

	if cfg.TransactionCountBlocksInterval != nil {
		p.transactionCountBlocksInterval = *cfg.TransactionCountBlocksInterval
	} else {
		// 1000 blocks ~= 100 minutes (assuming 6s blocks).
		p.transactionCountBlocksInterval = 1000
	}

	return item.NewAnalyzer(
		analyzerName,
		cfg.ItemBasedAnalyzerConfig,
		p,
		target,
		logger,
	)
}

func (p *processor) GetItems(ctx context.Context, limit uint64) ([]struct{}, error) {
	return []struct{}{{}}, nil
}

func (p *processor) ProcessItem(ctx context.Context, batch *storage.QueryBatch, item struct{}) error {
	// Always refresh accounts list view.
	batch.Queue(accountListViewRefreshQuery)

	// Accounts tx view is more expensive, refresh it after a configurable number of blocks.
	var latestBlock, computedHeight uint64
	err := p.target.QueryRow(ctx, accountTxLastRefreshQuery).Scan(&latestBlock, &computedHeight)
	if err != nil {
		return err
	}
	if (latestBlock > 0 && computedHeight == 0) || latestBlock-computedHeight > p.transactionCountBlocksInterval {
		batch.Queue(accountTxCountsRefreshQuery)
	}

	return nil
}

func (p *processor) QueueLength(ctx context.Context) (int, error) {
	// The concept of a work queue does not apply to this analyzer
	return 0, nil
}
