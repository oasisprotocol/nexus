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
	// The concept of a work queue does not apply to this analyzer
	return 0, nil
}
