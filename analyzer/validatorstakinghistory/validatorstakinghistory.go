package validatorstakinghistory

import (
	"context"
	"fmt"
	"math"

	"github.com/jackc/pgx/v5"
	"github.com/oasisprotocol/oasis-core/go/common/crypto/signature"

	"github.com/oasisprotocol/nexus/analyzer"
	"github.com/oasisprotocol/nexus/analyzer/item"
	"github.com/oasisprotocol/nexus/analyzer/queries"
	"github.com/oasisprotocol/nexus/config"
	staking "github.com/oasisprotocol/nexus/coreapi/v22.2.11/staking/api"
	"github.com/oasisprotocol/nexus/log"
	"github.com/oasisprotocol/nexus/storage"
	"github.com/oasisprotocol/nexus/storage/oasis/nodeapi"
)

// The validator staking history analyzer (1) gets the next epoch / validators to download validator
// history for, (2) downloads that info, and (3) saves the info in the database.
//
// We process epochs sequentially in chronological order in order to track validator
// history for all past and current validators. More concretly, for a given epoch, we find all current
// validators and take the union with the set of all tracked validators from the previous epoch.
// Notably, we do not track validator history for an entity before it first becomes an active validator.
//
// WARNING: This analyzer SHOULD NOT run while block analyzers are in fast sync. This analyzer expects that
// when a row appears in `chain.epochs`, its first block is already processed. In fast sync, blocks are processed
// out of order. (As of 2024-06, fast-sync actually does not update chain.epochs directly so it might not cause bugs
// when running in parallel with this analyzer, but that's by chance rather than by design.)

const (
	validatorHistoryAnalyzerName = "validator_history"
)

type processor struct {
	source     nodeapi.ConsensusApiLite
	target     storage.TargetStorage
	startEpoch uint64
	logger     *log.Logger
}

var _ item.ItemProcessor[*Epoch] = (*processor)(nil)

func NewAnalyzer(
	initCtx context.Context,
	cfg config.ItemBasedAnalyzerConfig,
	startHeight uint64,
	sourceClient nodeapi.ConsensusApiLite,
	target storage.TargetStorage,
	logger *log.Logger,
) (analyzer.Analyzer, error) {
	logger = logger.With("analyzer", validatorHistoryAnalyzerName)

	// Find the epoch corresponding to startHeight.
	if startHeight > math.MaxInt64 {
		return nil, fmt.Errorf("startHeight %d is too large", startHeight)
	}
	epoch, err := sourceClient.GetEpoch(initCtx, int64(startHeight))
	if err != nil {
		return nil, fmt.Errorf("failed to fetch epoch for startHeight %d", startHeight)
	}
	p := &processor{
		source:     sourceClient,
		target:     target,
		startEpoch: uint64(epoch),
		logger:     logger,
	}

	return item.NewAnalyzer[*Epoch](
		validatorHistoryAnalyzerName,
		cfg,
		p,
		target,
		logger,
	)
}

type Epoch struct {
	epoch          uint64
	startHeight    int64
	prevValidators []signature.PublicKey // Validator set from the previous epoch.
}

// Note: limit is ignored here because epochs must be processed sequentially in chronological order.
func (p *processor) GetItems(ctx context.Context, limit uint64) ([]*Epoch, error) {
	var epoch Epoch
	vAddrs := []string{}
	err := p.target.QueryRow(
		ctx,
		queries.ValidatorStakingHistoryUnprocessedEpochs,
		p.startEpoch,
		1, // overwrite limit to 1
	).Scan(
		&epoch.epoch,
		&epoch.startHeight,
		&vAddrs,
	)
	switch err {
	case nil:
		// continue
	case pgx.ErrNoRows:
		return []*Epoch{}, nil
	default:
		return nil, fmt.Errorf("querying epochs for validator history: %w", err)
	}

	// Convert vAddrs to signature.PublicKey
	validators := []signature.PublicKey{}
	for _, v := range vAddrs {
		a := signature.PublicKey{}
		if err := a.UnmarshalText([]byte(v)); err != nil {
			return nil, fmt.Errorf("invalid validator pubkey '%s': %w", v, err)
		}
		validators = append(validators, a)
	}
	epoch.prevValidators = validators

	return []*Epoch{&epoch}, nil
}

func (p *processor) ProcessItem(ctx context.Context, batch *storage.QueryBatch, epoch *Epoch) error {
	p.logger.Info("downloading validator balances", "epoch", epoch.epoch, "height", epoch.startHeight, "prev_validators", epoch.prevValidators)
	currValidators, err := p.source.GetValidators(ctx, epoch.startHeight)
	if err != nil {
		return fmt.Errorf("downloading validators for height %d", epoch.startHeight)
	}
	nodes, err := p.source.GetNodes(ctx, epoch.startHeight)
	if err != nil {
		return fmt.Errorf("downloading nodes for height %d", epoch.startHeight)
	}
	nodeToEntity := make(map[signature.PublicKey]signature.PublicKey)
	for _, n := range nodes {
		nodeToEntity[n.ID] = n.EntityID
	}
	// Find the union of past + current validator sets.
	validators := make(map[signature.PublicKey]struct{})
	for _, v := range epoch.prevValidators {
		validators[v] = struct{}{}
	}
	for _, v := range currValidators {
		// The ID returned by validator objects is the node ID, but we track validators by entity ID.
		vID := nodeToEntity[v.ID]
		if _, ok := validators[vID]; !ok {
			p.logger.Info("found new validator", "addr", staking.NewAddress(vID), "id", vID)
			validators[vID] = struct{}{}
		}
	}
	// Download info for each validator.
	validatorIDs := []string{}
	for vID := range validators {
		addr := staking.NewAddress(vID)
		p.logger.Info("starting to process validator", "addr", addr, "height", epoch.startHeight)
		validatorIDs = append(validatorIDs, vID.String())
		acct, err1 := p.source.GetAccount(ctx, epoch.startHeight, addr)
		if err1 != nil {
			return fmt.Errorf("fetching account info for %s at height %d", addr.String(), epoch.startHeight)
		}
		delegations, err1 := p.source.DelegationsTo(ctx, epoch.startHeight, addr)
		if err1 != nil {
			return fmt.Errorf("fetching delegations to account %s at height %d", addr.String(), epoch.startHeight)
		}
		batch.Queue(queries.ValidatorBalanceInsert,
			vID.String(),
			epoch.epoch,
			acct.Escrow.Active.Balance,
			acct.Escrow.Debonding.Balance,
			acct.Escrow.Active.TotalShares,
			acct.Escrow.Debonding.TotalShares,
			len(delegations),
		)
		p.logger.Info("processed validator", "addr", addr, "height", epoch.startHeight)
	}

	// Update chain.epochs with the set of past and current validators at this epoch.
	batch.Queue(queries.EpochValidatorsUpdate,
		epoch.epoch,
		validatorIDs,
	)

	p.logger.Info("finished processing epoch", "epoch", epoch.epoch)

	return nil
}

func (p *processor) QueueLength(ctx context.Context) (int, error) {
	var queueLength int
	if err := p.target.QueryRow(ctx, queries.ValidatorStakingHistoryUnprocessedCount, p.startEpoch).Scan(&queueLength); err != nil {
		return 0, fmt.Errorf("querying number of unprocessed epochs: %w", err)
	}
	return queueLength, nil
}
