// Package consensus implements an analyzer for the consensus layer.
package consensus

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/iancoleman/strcase"
	"github.com/jackc/pgx/v4"
	registry "github.com/oasisprotocol/metadata-registry-tools"
	"github.com/oasisprotocol/oasis-core/go/common/cbor"
	"github.com/oasisprotocol/oasis-core/go/consensus/api/transaction"
	"github.com/oasisprotocol/oasis-core/go/consensus/api/transaction/results"
	staking "github.com/oasisprotocol/oasis-core/go/staking/api"
	oasisConfig "github.com/oasisprotocol/oasis-sdk/client-sdk/go/config"
	"golang.org/x/sync/errgroup"

	"github.com/oasislabs/oasis-indexer/analyzer"
	"github.com/oasislabs/oasis-indexer/analyzer/util"
	"github.com/oasislabs/oasis-indexer/config"
	"github.com/oasislabs/oasis-indexer/log"
	"github.com/oasislabs/oasis-indexer/metrics"
	"github.com/oasislabs/oasis-indexer/storage"
	source "github.com/oasislabs/oasis-indexer/storage/oasis"
)

const (
	consensusMainDamaskName = "consensus_main_damask"
	registryUpdateFrequency = 100 // once per n block
)

var (
	// ErrOutOfRange is returned if the current block does not fall within tge
	// analyzer's analysis range.
	ErrOutOfRange = errors.New("range not found. no data source available")

	// ErrLatestBlockNotFound is returned if the analyzer has not indexed any
	// blocks yet. This indicates to begin from the start of its range.
	ErrLatestBlockNotFound = errors.New("latest block not found")
)

// Main is the main Analyzer for the consensus layer.
type Main struct {
	cfg     analyzer.Config
	target  storage.TargetStorage
	logger  *log.Logger
	metrics metrics.DatabaseMetrics
}

// NewMain returns a new main analyzer for the consensus layer.
func NewMain(cfg *config.AnalyzerConfig, target storage.TargetStorage, logger *log.Logger) (*Main, error) {
	ctx := context.Background()

	var ac analyzer.Config
	if cfg.Interval == "" {
		// Initialize source storage.
		networkCfg := oasisConfig.Network{
			ChainContext: cfg.ChainContext,
			RPC:          cfg.RPC,
		}
		source, err := source.NewClient(ctx, &networkCfg)
		if err != nil {
			return nil, err
		}

		// Configure analyzer.
		blockRange := analyzer.Range{
			From: cfg.From,
			To:   cfg.To,
		}
		ac = analyzer.Config{
			ChainID:    cfg.ChainID,
			BlockRange: blockRange,
			Source:     source,
		}
	} else {
		interval, err := time.ParseDuration(cfg.Interval)
		if err != nil {
			return nil, err
		}

		// Configure analyzer.
		ac = analyzer.Config{
			ChainID:  cfg.ChainID,
			Interval: interval,
		}
	}
	cfg.ChainID = strcase.ToSnake(cfg.ChainID)
	return &Main{
		cfg:     ac,
		target:  target,
		logger:  logger.With("analyzer", consensusMainDamaskName),
		metrics: metrics.NewDefaultDatabaseMetrics(consensusMainDamaskName),
	}, nil
}

// Start starts the main consensus analyzer.
func (m *Main) Start() {
	ctx := context.Background()

	// Get block to be indexed.
	var height int64

	latest, err := m.latestBlock(ctx)
	if err != nil {
		if err != pgx.ErrNoRows {
			m.logger.Error("last block height not found",
				"err", err.Error(),
			)
			return
		}
		m.logger.Debug("setting height using range config")
		height = m.cfg.BlockRange.From
	} else {
		m.logger.Debug("setting height using latest block")
		height = latest + 1
	}

	backoff, err := util.NewBackoff(
		100*time.Millisecond,
		6*time.Second,
		// ^cap the timeout at the expected
		// consensus block time
	)
	if err != nil {
		m.logger.Error("error configuring indexer backoff policy",
			"err", err.Error(),
		)
		return
	}
	for m.cfg.BlockRange.To == 0 || height <= m.cfg.BlockRange.To {
		if err := m.processBlock(ctx, height); err != nil {
			if err == ErrOutOfRange {
				m.logger.Info("no data source available at this height",
					"height", height,
				)
				return
			}

			m.logger.Error("error processing block",
				"err", err.Error(),
			)
			backoff.Wait()
			continue
		}

		backoff.Reset()
		height++
	}
}

// Name returns the name of the Main.
func (m *Main) Name() string {
	return consensusMainDamaskName
}

// source returns the source storage for the provided block height.
func (m *Main) source(height int64) (storage.SourceStorage, error) {
	r := m.cfg
	if height >= r.BlockRange.From && (r.BlockRange.To == 0 || height <= r.BlockRange.To) {
		return r.Source, nil
	}

	return nil, ErrOutOfRange
}

// latestBlock returns the latest block processed by the consensus analyzer.
func (m *Main) latestBlock(ctx context.Context) (int64, error) {
	var latest int64
	if err := m.target.QueryRow(
		ctx,
		makeLatestBlockQuery(m.cfg.ChainID),
		// ^analyzers should only analyze for a single chain ID, and we anchor this
		// at the starting block.
		consensusMainDamaskName,
	).Scan(&latest); err != nil {
		return 0, err
	}
	return latest, nil
}

// processBlock processes the block at the provided block height.
func (m *Main) processBlock(ctx context.Context, height int64) error {
	m.logger.Info("processing block",
		"height", height,
	)
	chainID := m.cfg.ChainID

	group, groupCtx := errgroup.WithContext(ctx)

	// Prepare and perform updates.
	batch := &storage.QueryBatch{}

	type prepareFunc = func(context.Context, int64, *storage.QueryBatch) error
	for _, f := range []prepareFunc{
		m.prepareBlockData,
		m.prepareRegistryData,
		m.prepareStakingData,
		m.prepareSchedulerData,
		m.prepareGovernanceData,
	} {
		func(f prepareFunc) {
			group.Go(func() error {
				if err := f(groupCtx, height, batch); err != nil {
					return err
				}
				return nil
			})
		}(f)
	}

	// Update indexing progress.
	group.Go(func() error {
		batch.Queue(
			makeIndexingProgressQuery(chainID),
			height,
			consensusMainDamaskName,
		)
		return nil
	})

	if err := group.Wait(); err != nil {
		return err
	}

	opName := "process_block"
	timer := m.metrics.DatabaseTimer(m.target.Name(), opName)
	defer timer.ObserveDuration()

	if err := m.target.SendBatch(ctx, batch); err != nil {
		m.metrics.DatabaseCounter(m.target.Name(), opName, "failure").Inc()
		return err
	}
	m.metrics.DatabaseCounter(m.target.Name(), opName, "success").Inc()
	return nil
}

// prepareBlockData adds block data queries to the batch.
func (m *Main) prepareBlockData(ctx context.Context, height int64, batch *storage.QueryBatch) error {
	source, err := m.source(height)
	if err != nil {
		return err
	}

	data, err := source.BlockData(ctx, height)
	if err != nil {
		return err
	}

	for _, f := range []func(*storage.QueryBatch, *storage.BlockData) error{
		m.queueBlockInserts,
		m.queueEpochInserts,
		m.queueTransactionInserts,
		m.queueEventInserts,
	} {
		if err := f(batch, data); err != nil {
			return err
		}
	}

	return nil
}

func (m *Main) queueBlockInserts(batch *storage.QueryBatch, data *storage.BlockData) error {
	chainID := m.cfg.ChainID

	batch.Queue(
		makeBlockInsertQuery(chainID),
		data.BlockHeader.Height,
		data.BlockHeader.Hash.Hex(),
		data.BlockHeader.Time.UTC(),
		data.BlockHeader.StateRoot.Namespace.String(),
		int64(data.BlockHeader.StateRoot.Version),
		data.BlockHeader.StateRoot.Type.String(),
		data.BlockHeader.StateRoot.Hash.Hex(),
	)

	return nil
}

func (m *Main) queueEpochInserts(batch *storage.QueryBatch, data *storage.BlockData) error {
	chainID := m.cfg.ChainID

	batch.Queue(
		makeEpochInsertQuery(chainID),
		data.Epoch,
		data.BlockHeader.Height,
	)
	batch.Queue(
		makeEpochUpdateQuery(chainID),
		data.Epoch-1,
		data.BlockHeader.Height,
	)

	return nil
}

func (m *Main) queueTransactionInserts(batch *storage.QueryBatch, data *storage.BlockData) error {
	chainID := m.cfg.ChainID
	transactionInsertQuery := makeTransactionInsertQuery(chainID)
	accountNonceUpdateQuery := makeAccountNonceUpdateQuery(chainID)
	commissionsUpsertQuery := makeCommissionsUpsertQuery(chainID)

	for i := range data.Transactions {
		signedTx := data.Transactions[i]
		result := data.Results[i]

		var tx transaction.Transaction
		if err := signedTx.Open(&tx); err != nil {
			continue
		}

		sender := staking.NewAddress(
			signedTx.Signature.PublicKey,
		).String()

		batch.Queue(transactionInsertQuery,
			data.BlockHeader.Height,
			signedTx.Hash().Hex(),
			i,
			tx.Nonce,
			tx.Fee.Amount.ToBigInt().Uint64(),
			tx.Fee.Gas,
			tx.Method,
			sender,
			tx.Body,
			result.Error.Module,
			result.Error.Code,
			result.Error.Message,
		)
		batch.Queue(accountNonceUpdateQuery,
			sender,
			tx.Nonce+1,
		)

		// TODO: Use event when available
		// https://github.com/oasisprotocol/oasis-core/issues/4818
		if tx.Method == "staking.AmendCommissionSchedule" {
			var rawSchedule staking.AmendCommissionSchedule
			if err := cbor.Unmarshal(tx.Body, &rawSchedule); err != nil {
				return err
			}

			schedule, err := json.Marshal(rawSchedule)
			if err != nil {
				return err
			}

			batch.Queue(commissionsUpsertQuery,
				staking.NewAddress(signedTx.Signature.PublicKey),
				string(schedule),
			)
		}
	}

	return nil
}

func (m *Main) queueEventInserts(batch *storage.QueryBatch, data *storage.BlockData) error {
	chainID := m.cfg.ChainID
	eventInsertQuery := makeEventInsertQuery(chainID)

	for i := 0; i < len(data.Results); i++ {
		for j := 0; j < len(data.Results[i].Events); j++ {
			backend, ty, body, err := extractEventData(data.Results[i].Events[j])
			if err != nil {
				return err
			}

			batch.Queue(eventInsertQuery,
				backend.String(),
				ty.String(),
				string(body),
				data.BlockHeader.Height,
				data.Transactions[i].Hash().Hex(),
				i,
			)
		}
	}

	return nil
}

// prepareRegistryData adds registry data queries to the batch.
func (m *Main) prepareRegistryData(ctx context.Context, height int64, batch *storage.QueryBatch) error {
	source, err := m.source(height)
	if err != nil {
		return err
	}

	data, err := source.RegistryData(ctx, height)
	if err != nil {
		return err
	}

	for _, f := range []func(*storage.QueryBatch, *storage.RegistryData) error{
		m.queueRuntimeRegistrations,
		m.queueRuntimeStatusUpdates,
		m.queueEntityEvents,
		m.queueNodeEvents,
	} {
		if err := f(batch, data); err != nil {
			return err
		}
	}

	if height%registryUpdateFrequency == 0 {
		if err := m.queueMetadataRegistry(ctx, batch); err != nil {
			return err
		}
	}

	return nil
}

func (m *Main) queueRuntimeRegistrations(batch *storage.QueryBatch, data *storage.RegistryData) error {
	chainID := m.cfg.ChainID
	runtimeUpsertQuery := makeRuntimeUpsertQuery(chainID)

	for _, runtimeEvent := range data.RuntimeEvents {
		keyManager := "none"

		if runtimeEvent.Runtime.KeyManager != nil {
			keyManager = runtimeEvent.Runtime.KeyManager.String()
		}

		batch.Queue(runtimeUpsertQuery,
			runtimeEvent.Runtime.ID.String(),
			false,
			runtimeEvent.Runtime.Kind.String(),
			runtimeEvent.Runtime.TEEHardware.String(),
			keyManager,
		)
	}

	return nil
}

func (m *Main) queueRuntimeStatusUpdates(batch *storage.QueryBatch, data *storage.RegistryData) error {
	chainID := m.cfg.ChainID
	runtimeSuspensionQuery := makeRuntimeSuspensionQuery(chainID)
	runtimeUnsuspensionQuery := makeRuntimeUnsuspensionQuery(chainID)

	for _, runtime := range data.RuntimeSuspensions {
		batch.Queue(runtimeSuspensionQuery, runtime)
	}
	for _, runtime := range data.RuntimeUnsuspensions {
		batch.Queue(runtimeUnsuspensionQuery, runtime)
	}

	return nil
}

func (m *Main) queueEntityEvents(batch *storage.QueryBatch, data *storage.RegistryData) error {
	chainID := m.cfg.ChainID
	claimedNodeInsertQuery := makeClaimedNodeInsertQuery(chainID)
	entityUpsertQuery := makeEntityUpsertQuery(chainID)

	for _, entityEvent := range data.EntityEvents {
		entityID := entityEvent.Entity.ID.String()

		for _, node := range entityEvent.Entity.Nodes {
			batch.Queue(claimedNodeInsertQuery,
				entityID,
				node,
			)
		}
		batch.Queue(entityUpsertQuery,
			entityID,
			staking.NewAddress(entityEvent.Entity.ID).String(),
		)
	}

	return nil
}

func (m *Main) queueNodeEvents(batch *storage.QueryBatch, data *storage.RegistryData) error {
	chainID := m.cfg.ChainID
	nodeUpsertQuery := makeNodeUpsertQuery(chainID)
	nodeDeleteQuery := makeNodeDeleteQuery(chainID)

	for _, nodeEvent := range data.NodeEvents {
		vrfPubkey := ""

		if nodeEvent.Node.VRF != nil {
			vrfPubkey = nodeEvent.Node.VRF.ID.String()
		}
		var tlsAddresses []string
		for _, address := range nodeEvent.Node.TLS.Addresses {
			tlsAddresses = append(tlsAddresses, address.String())
		}

		var p2pAddresses []string
		for _, address := range nodeEvent.Node.P2P.Addresses {
			p2pAddresses = append(p2pAddresses, address.String())
		}

		var consensusAddresses []string
		for _, address := range nodeEvent.Node.Consensus.Addresses {
			consensusAddresses = append(consensusAddresses, address.String())
		}

		if nodeEvent.IsRegistration {
			// A new node is registered.
			batch.Queue(nodeUpsertQuery,
				nodeEvent.Node.ID.String(),
				nodeEvent.Node.EntityID.String(),
				nodeEvent.Node.Expiration,
				nodeEvent.Node.TLS.PubKey.String(),
				nodeEvent.Node.TLS.NextPubKey.String(),
				fmt.Sprintf(`{'%s'}`, strings.Join(tlsAddresses, `','`)),
				nodeEvent.Node.P2P.ID.String(),
				fmt.Sprintf(`{'%s'}`, strings.Join(p2pAddresses, `','`)),
				nodeEvent.Node.Consensus.ID.String(),
				strings.Join(consensusAddresses, ","),
				vrfPubkey,
				nodeEvent.Node.Roles,
				nodeEvent.Node.SoftwareVersion,
				0,
			)
		} else {
			// An existing node is expired.
			batch.Queue(nodeDeleteQuery,
				nodeEvent.Node.ID.String(),
			)
		}
	}

	return nil
}

func (m *Main) queueMetadataRegistry(ctx context.Context, batch *storage.QueryBatch) error {
	gp, err := registry.NewGitProvider(registry.NewGitConfig())
	if err != nil {
		m.logger.Error(fmt.Sprintf("Failed to create Git registry provider: %s\n", err))
		return err
	}

	// Get a list of all entities in the registry.
	entities, err := gp.GetEntities(ctx)
	if err != nil {
		m.logger.Error(fmt.Sprintf("Failed to get a list of entities in registry: %s\n", err))
		return err
	}

	entityMetaUpsertQuery := makeEntityMetaUpsertQuery(m.cfg.ChainID)
	for id, meta := range entities {
		batch.Queue(entityMetaUpsertQuery,
			id,
			meta,
		)
	}

	return nil
}

func (m *Main) prepareStakingData(ctx context.Context, height int64, batch *storage.QueryBatch) error {
	source, err := m.source(height)
	if err != nil {
		return err
	}

	data, err := source.StakingData(ctx, height)
	if err != nil {
		return err
	}

	for _, f := range []func(*storage.QueryBatch, *storage.StakingData) error{
		m.queueTransfers,
		m.queueBurns,
		m.queueEscrows,
		m.queueAllowanceChanges,
	} {
		if err := f(batch, data); err != nil {
			return err
		}
	}

	return nil
}

func (m *Main) queueTransfers(batch *storage.QueryBatch, data *storage.StakingData) error {
	chainID := m.cfg.ChainID
	senderUpdateQuery := makeSenderUpdateQuery(chainID)
	receiverUpsertQuery := makeReceiverUpdateQuery(chainID)

	for _, transfer := range data.Transfers {
		from := transfer.From.String()
		to := transfer.To.String()
		amount := transfer.Amount.ToBigInt().Uint64()
		batch.Queue(senderUpdateQuery,
			from,
			amount,
		)
		batch.Queue(receiverUpsertQuery,
			to,
			amount,
		)
	}

	return nil
}

func (m *Main) queueBurns(batch *storage.QueryBatch, data *storage.StakingData) error {
	chainID := m.cfg.ChainID
	burnUpdateQuery := makeBurnUpdateQuery(chainID)

	for _, burn := range data.Burns {
		batch.Queue(burnUpdateQuery,
			burn.Owner.String(),
			burn.Amount.ToBigInt().Uint64(),
		)
	}

	return nil
}

func (m *Main) queueEscrows(batch *storage.QueryBatch, data *storage.StakingData) error {
	chainID := m.cfg.ChainID
	addGeneralBalanceUpdateQuery := makeAddGeneralBalanceUpdateQuery(chainID)
	addEscrowBalanceUpsertQuery := makeAddEscrowBalanceUpsertQuery(chainID)
	addDelegationsUpsertQuery := makeAddDelegationsUpsertQuery(chainID)
	takeEscrowUpdateQuery := makeTakeEscrowUpdateQuery(chainID)
	debondingStartEscrowBalanceUpdateQuery := makeDebondingStartEscrowBalanceUpdateQuery(chainID)
	debondingStartDelegationsUpdateQuery := makeDebondingStartDelegationsUpdateQuery(chainID)
	debondingStartDebondingDelegationsInsertQuery := makeDebondingStartDebondingDelegationsInsertQuery(chainID)
	reclaimGeneralBalanceUpdateQuery := makeReclaimGeneralBalanceUpdateQuery(chainID)
	reclaimEscrowBalanceUpdateQuery := makeReclaimEscrowBalanceUpdateQuery(chainID)

	for _, escrow := range data.Escrows {
		switch e := escrow; {
		case e.Add != nil:
			owner := e.Add.Owner.String()
			escrower := e.Add.Escrow.String()
			amount := e.Add.Amount.ToBigInt().Uint64()
			newShares := e.Add.NewShares.ToBigInt().Uint64()
			batch.Queue(addGeneralBalanceUpdateQuery,
				owner,
				amount,
			)
			batch.Queue(addEscrowBalanceUpsertQuery,
				escrower,
				amount,
				newShares,
			)
			batch.Queue(addDelegationsUpsertQuery,
				escrower,
				owner,
				newShares,
			)
		case e.Take != nil:
			batch.Queue(takeEscrowUpdateQuery,
				e.Take.Owner.String(),
				e.Take.Amount.ToBigInt().Uint64(),
			)
		case e.DebondingStart != nil:
			batch.Queue(debondingStartEscrowBalanceUpdateQuery,
				e.DebondingStart.Escrow.String(),
				e.DebondingStart.DebondingShares.ToBigInt().Uint64(),
			)
			batch.Queue(debondingStartDelegationsUpdateQuery,
				e.DebondingStart.Escrow.String(),
				e.DebondingStart.Owner.String(),
				e.DebondingStart.ActiveShares.ToBigInt().Uint64(),
			)
			batch.Queue(debondingStartDebondingDelegationsInsertQuery,
				e.DebondingStart.Escrow.String(),
				e.DebondingStart.Owner.String(),
				e.DebondingStart.DebondingShares.ToBigInt().Uint64(),
				e.DebondingStart.DebondEndTime,
			)
		case e.Reclaim != nil:
			batch.Queue(reclaimGeneralBalanceUpdateQuery,
				e.Reclaim.Owner.String(),
				e.Reclaim.Amount.ToBigInt().Uint64(),
			)
			batch.Queue(reclaimEscrowBalanceUpdateQuery,
				e.Reclaim.Escrow.String(),
				e.Reclaim.Amount.ToBigInt().Uint64(),
				e.Reclaim.Shares.ToBigInt().Uint64(),
			)

			// TODO: Delete row from `debonding_delegations` that corresponds with
			// the reclaimed escrow. The reclaim occurs on epoch transition, so
			// check which epoch just transitioned.
		}
	}

	return nil
}

func (m *Main) queueAllowanceChanges(batch *storage.QueryBatch, data *storage.StakingData) error {
	chainID := m.cfg.ChainID
	allowanceChangeDeleteQuery := makeAllowanceChangeDeleteQuery(chainID)
	allowanceChangeUpdateQuery := makeAllowanceChangeUpdateQuery(chainID)

	for _, allowanceChange := range data.AllowanceChanges {
		allowance := allowanceChange.Allowance.ToBigInt().Uint64()
		if allowance == 0 {
			batch.Queue(allowanceChangeDeleteQuery,
				allowanceChange.Owner.String(),
				allowanceChange.Beneficiary.String(),
			)
		} else {
			batch.Queue(allowanceChangeUpdateQuery,
				allowanceChange.Owner.String(),
				allowanceChange.Beneficiary.String(),
				allowance,
			)
		}
	}

	return nil
}

// prepareSchedulerData adds scheduler data queries to the batch.
func (m *Main) prepareSchedulerData(ctx context.Context, height int64, batch *storage.QueryBatch) error {
	source, err := m.source(height)
	if err != nil {
		return err
	}

	data, err := source.SchedulerData(ctx, height)
	if err != nil {
		return err
	}

	for _, f := range []func(*storage.QueryBatch, *storage.SchedulerData) error{
		m.queueValidatorUpdates,
		m.queueCommitteeUpdates,
	} {
		if err := f(batch, data); err != nil {
			return err
		}
	}

	return nil
}

func (m *Main) queueValidatorUpdates(batch *storage.QueryBatch, data *storage.SchedulerData) error {
	chainID := m.cfg.ChainID
	validatorNodeUpdateQuery := makeValidatorNodeUpdateQuery(chainID)
	for _, validator := range data.Validators {
		batch.Queue(validatorNodeUpdateQuery,
			validator.ID,
			validator.VotingPower,
		)
	}

	return nil
}

func (m *Main) queueCommitteeUpdates(batch *storage.QueryBatch, data *storage.SchedulerData) error {
	chainID := m.cfg.ChainID
	committeeMemberInsertQuery := makeCommitteeMemberInsertQuery(chainID)

	batch.Queue(makeCommitteeMembersTruncateQuery(chainID))
	for namespace, committees := range data.Committees {
		runtime := namespace.String()
		for _, committee := range committees {
			kind := committee.String()
			validFor := int64(committee.ValidFor)
			for _, member := range committee.Members {
				batch.Queue(committeeMemberInsertQuery,
					member.PublicKey,
					validFor,
					runtime,
					kind,
					member.Role.String(),
				)
			}
		}
	}

	return nil
}

// prepareGovernanceData adds governance data queries to the batch.
func (m *Main) prepareGovernanceData(ctx context.Context, height int64, batch *storage.QueryBatch) error {
	source, err := m.source(height)
	if err != nil {
		return err
	}

	data, err := source.GovernanceData(ctx, height)
	if err != nil {
		return err
	}

	for _, f := range []func(*storage.QueryBatch, *storage.GovernanceData) error{
		m.queueSubmissions,
		m.queueExecutions,
		m.queueFinalizations,
		m.queueVotes,
	} {
		if err := f(batch, data); err != nil {
			return err
		}
	}

	return nil
}

func (m *Main) queueSubmissions(batch *storage.QueryBatch, data *storage.GovernanceData) error {
	chainID := m.cfg.ChainID
	proposalSubmissionInsertQuery := makeProposalSubmissionInsertQuery(chainID)
	proposalSubmissionCancelInsertQuery := makeProposalSubmissionCancelInsertQuery(chainID)

	for _, submission := range data.ProposalSubmissions {
		if submission.Content.Upgrade != nil {
			batch.Queue(proposalSubmissionInsertQuery,
				submission.ID,
				submission.Submitter.String(),
				submission.State.String(),
				submission.Deposit.ToBigInt().Uint64(),
				submission.Content.Upgrade.Handler,
				submission.Content.Upgrade.Target.ConsensusProtocol.String(),
				submission.Content.Upgrade.Target.RuntimeHostProtocol.String(),
				submission.Content.Upgrade.Target.RuntimeCommitteeProtocol.String(),
				submission.Content.Upgrade.Epoch,
				submission.CreatedAt,
				submission.ClosesAt,
			)
		} else if submission.Content.CancelUpgrade != nil {
			batch.Queue(proposalSubmissionCancelInsertQuery,
				submission.ID,
				submission.Submitter.String(),
				submission.State.String(),
				submission.Deposit.ToBigInt().Uint64(),
				submission.Content.CancelUpgrade.ProposalID,
				submission.CreatedAt,
				submission.ClosesAt,
			)
		}
	}

	return nil
}

func (m *Main) queueExecutions(batch *storage.QueryBatch, data *storage.GovernanceData) error {
	chainID := m.cfg.ChainID
	proposalExecutionsUpdateQuery := makeProposalExecutionsUpdateQuery(chainID)

	for _, execution := range data.ProposalExecutions {
		batch.Queue(proposalExecutionsUpdateQuery,
			execution.ID,
		)
	}

	return nil
}

func (m *Main) queueFinalizations(batch *storage.QueryBatch, data *storage.GovernanceData) error {
	chainID := m.cfg.ChainID
	proposalUpdateQuery := makeProposalUpdateQuery(chainID)
	proposalInvalidVotesUpdateQuery := makeProposalInvalidVotesUpdateQuery(chainID)

	for _, finalization := range data.ProposalFinalizations {
		batch.Queue(proposalUpdateQuery,
			finalization.ID,
			finalization.State.String(),
		)
		batch.Queue(proposalInvalidVotesUpdateQuery,
			finalization.ID,
			finalization.InvalidVotes,
		)
	}

	return nil
}

func (m *Main) queueVotes(batch *storage.QueryBatch, data *storage.GovernanceData) error {
	chainID := m.cfg.ChainID
	voteInsertQuery := makeVoteInsertQuery(chainID)

	for _, vote := range data.Votes {
		batch.Queue(voteInsertQuery,
			vote.ID,
			vote.Submitter.String(),
			vote.Vote.String(),
		)
	}

	return nil
}

// extractEventData extracts the type of an event.
//
// TODO: Eliminate this if possible.
func extractEventData(event *results.Event) (backend analyzer.Backend, ty analyzer.Event, body []byte, err error) {
	switch e := event; {
	case e.Staking != nil:
		backend = analyzer.BackendStaking
		switch b := event.Staking; {
		case b.Transfer != nil:
			ty = analyzer.EventStakingTransfer
			body, err = json.Marshal(b.Transfer)
			return
		case b.Burn != nil:
			ty = analyzer.EventStakingBurn
			body, err = json.Marshal(b.Burn)
			return
		case b.Escrow != nil:
			switch t := b.Escrow; {
			case t.Add != nil:
				ty = analyzer.EventStakingAddEscrow
				body, err = json.Marshal(b.Escrow.Add)
				return
			case t.Take != nil:
				ty = analyzer.EventStakingTakeEscrow
				body, err = json.Marshal(b.Escrow.Take)
				return
			case t.DebondingStart != nil:
				ty = analyzer.EventStakingDebondingStart
				body, err = json.Marshal(b.Escrow.DebondingStart)
				return
			case t.Reclaim != nil:
				ty = analyzer.EventStakingReclaimEscrow
				body, err = json.Marshal(b.Escrow.Reclaim)
				return
			}
		case b.AllowanceChange != nil:
			ty = analyzer.EventStakingAllowanceChange
			body, err = json.Marshal(b.AllowanceChange)
			return
		}
	case e.Registry != nil:
		backend = analyzer.BackendRegistry
		switch b := event.Registry; {
		case b.RuntimeEvent != nil:
			ty = analyzer.EventRegistryRuntime
			body, err = json.Marshal(b.RuntimeEvent)
			return
		case b.EntityEvent != nil:
			ty = analyzer.EventRegistryEntity
			body, err = json.Marshal(b.EntityEvent)
			return
		case b.NodeEvent != nil:
			ty = analyzer.EventRegistryNode
			body, err = json.Marshal(b.NodeEvent)
			return
		case b.NodeUnfrozenEvent != nil:
			ty = analyzer.EventRegistryNodeUnfrozen
			body, err = json.Marshal(b.NodeUnfrozenEvent)
			return
		}
	case e.RootHash != nil:
		backend = analyzer.BackendRoothash
		switch b := event.RootHash; {
		case b.ExecutorCommitted != nil:
			ty = analyzer.EventRoothashExecutorCommitted
			body, err = json.Marshal(event.RootHash.ExecutorCommitted)
			return
		case b.ExecutionDiscrepancyDetected != nil:
			ty = analyzer.EventRoothashDiscrepancyDetected
			body, err = json.Marshal(event.RootHash.ExecutionDiscrepancyDetected)
			return
		case b.Finalized != nil:
			ty = analyzer.EventRoothashFinalized
			body, err = json.Marshal(event.RootHash.Finalized)
			return
		}
	case e.Governance != nil:
		backend = analyzer.BackendGovernance
		switch b := event.Governance; {
		case b.ProposalSubmitted != nil:
			ty = analyzer.EventGovernanceProposalSubmitted
			body, err = json.Marshal(event.Governance.ProposalSubmitted)
			return
		case b.ProposalExecuted != nil:
			ty = analyzer.EventGovernanceProposalExecuted
			body, err = json.Marshal(event.Governance.ProposalExecuted)
			return
		case b.ProposalFinalized != nil:
			ty = analyzer.EventGovernanceProposalExecuted
			body, err = json.Marshal(event.Governance.ProposalFinalized)
			return
		case b.Vote != nil:
			ty = analyzer.EventGovernanceVote
			body, err = json.Marshal(event.Governance.Vote)
			return
		}
	}

	return analyzer.BackendUnknown, analyzer.EventUnknown, []byte{}, errors.New("unknown event type")
}
