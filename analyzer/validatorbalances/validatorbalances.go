package evmtokens

import (
	"context"
	"fmt"

	"github.com/oasisprotocol/nexus/analyzer"
	"github.com/oasisprotocol/nexus/analyzer/item"
	"github.com/oasisprotocol/nexus/analyzer/queries"
	"github.com/oasisprotocol/nexus/config"
	staking "github.com/oasisprotocol/nexus/coreapi/v22.2.11/staking/api"
	"github.com/oasisprotocol/nexus/log"
	"github.com/oasisprotocol/nexus/storage"
	"github.com/oasisprotocol/nexus/storage/oasis/nodeapi"
)

// The validator balance analyzer (1) gets a list of epochs to download validator
// balance info for, (2) downloads that info, and (3) saves the info in the database.

const (
	//nolint:gosec // thinks this is a hardcoded credential
	validatorBalanceAnalyzerName = "validator_balance_"
)

type processor struct {
	source nodeapi.ConsensusApiLite
	target storage.TargetStorage
	logger *log.Logger
}

var _ item.ItemProcessor[*Epoch] = (*processor)(nil)

func NewAnalyzer(
	cfg config.ItemBasedAnalyzerConfig,
	sourceClient nodeapi.ConsensusApiLite,
	target storage.TargetStorage,
	logger *log.Logger,
) (analyzer.Analyzer, error) {
	logger = logger.With("analyzer", validatorBalanceAnalyzerName)
	p := &processor{
		source: sourceClient,
		target: target,
		logger: logger,
	}

	return item.NewAnalyzer[*Epoch](
		validatorBalanceAnalyzerName,
		cfg,
		p,
		target,
		logger,
	)
}

type Epoch struct {
	Epoch       uint64
	StartHeight int64
}

func (p *processor) GetItems(ctx context.Context, limit uint64) ([]*Epoch, error) {
	var epochs []*Epoch
	rows, err := p.target.Query(ctx, queries.ConsensusValidatorEpochs, limit)
	if err != nil {
		return nil, fmt.Errorf("querying epochs for validator history: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var epoch Epoch
		if err = rows.Scan(
			&epoch.Epoch,
			&epoch.StartHeight,
		); err != nil {
			return nil, fmt.Errorf("scanning epoch: %w", err)
		}
		epochs = append(epochs, &epoch)
	}
	return epochs, nil
}

func (p *processor) ProcessItem(ctx context.Context, batch *storage.QueryBatch, epoch *Epoch) error {
	p.logger.Info("downloading validator balances", "epoch", epoch)
	validators, err := p.source.GetValidators(ctx, epoch.StartHeight)
	if err != nil {
		return fmt.Errorf("downloading validators for height %d", epoch.StartHeight)
	}
	for _, v := range validators {
		addr := staking.NewAddress(v.ID)
		acct, err1 := p.source.GetAccount(ctx, epoch.StartHeight, addr)
		if err1 != nil {
			return fmt.Errorf("fetching account info for %s at height %d", addr.String(), epoch.StartHeight)
		}
		batch.Queue(queries.ConsensusValidatorBalanceInsert,
			v.ID.String(),
			epoch.Epoch,
			acct.Escrow.Active.Balance,
			acct.Escrow.Debonding.Balance,
		)
	}

	return nil
}

func (p *processor) QueueLength(ctx context.Context) (int, error) {
	var queueLength int
	if err := p.target.QueryRow(ctx, queries.ConsensusValidatorEpochsCount).Scan(&queueLength); err != nil {
		return 0, fmt.Errorf("querying number of unprocessed epochs: %w", err)
	}
	return queueLength, nil
}
