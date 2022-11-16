// Package consensus implements an analyzer for the consensus layer.
package consensus

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
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

	"github.com/oasisprotocol/oasis-indexer/analyzer"
	"github.com/oasisprotocol/oasis-indexer/analyzer/util"
	"github.com/oasisprotocol/oasis-indexer/config"
	"github.com/oasisprotocol/oasis-indexer/log"
	"github.com/oasisprotocol/oasis-indexer/metrics"
	"github.com/oasisprotocol/oasis-indexer/storage"
	source "github.com/oasisprotocol/oasis-indexer/storage/oasis"
)

const (
	consensusDamaskAnalyzerName = "consensus_damask"
	registryUpdateFrequency     = 100 // once per n block
)

// Main is the main Analyzer for the consensus layer.
type Main struct {
	cfg     analyzer.ConsensusConfig
	qf      analyzer.QueryFactory
	target  storage.TargetStorage
	logger  *log.Logger
	metrics metrics.DatabaseMetrics
}

// NewMain returns a new main analyzer for the consensus layer.
func NewMain(cfg *config.AnalyzerConfig, target storage.TargetStorage, logger *log.Logger) (*Main, error) {
	ctx := context.Background()

	var ac analyzer.ConsensusConfig
	if cfg.Interval == "" {
		// Initialize source storage.
		networkCfg := oasisConfig.Network{
			ChainContext: cfg.ChainContext,
			RPC:          cfg.RPC,
		}
		factory, err := source.NewClientFactory(ctx, &networkCfg)
		if err != nil {
			logger.Error("error creating client factory",
				"err", err.Error(),
			)
			return nil, err
		}
		client, err := factory.Consensus()
		if err != nil {
			logger.Error("error creating consensus client",
				"err", err.Error(),
			)
			return nil, err
		}

		// Configure analyzer.
		blockRange := analyzer.BlockRange{
			From: cfg.From,
			To:   cfg.To,
		}
		ac = analyzer.ConsensusConfig{
			ChainID: cfg.ChainID,
			Range:   blockRange,
			Source:  client,
		}
	} else {
		interval, err := time.ParseDuration(cfg.Interval)
		if err != nil {
			logger.Error("error parsing analysis interval",
				"err", err.Error(),
			)
			return nil, err
		}

		// Configure analyzer.
		ac = analyzer.ConsensusConfig{
			ChainID:  cfg.ChainID,
			Interval: interval,
		}
	}

	logger.Info("Starting consensus analyzer", "config", ac)
	return &Main{
		cfg:     ac,
		qf:      analyzer.NewQueryFactory(strcase.ToSnake(cfg.ChainID), "" /* no runtime identifier for the consensus layer */),
		target:  target,
		logger:  logger.With("analyzer", consensusDamaskAnalyzerName),
		metrics: metrics.NewDefaultDatabaseMetrics(consensusDamaskAnalyzerName),
	}, nil
}

// Start starts the main consensus analyzer.
func (m *Main) Start() {
	ctx := context.Background()

	// Start aggregate worker.
	go m.aggregateWorker(ctx)

	// Get block to be indexed.
	var height int64

	isGenesisProcessed, err := m.isGenesisProcessed(ctx)
	if err != nil {
		m.logger.Error("failed to check if genesis is processed",
			"err", err.Error(),
		)
		return
	}
	if !isGenesisProcessed {
		if err = m.processGenesis(ctx); err != nil {
			m.logger.Error("failed to process genesis",
				"err", err.Error(),
			)
			return
		}
	}

	latest, err := m.latestBlock(ctx)
	if err != nil {
		if err != pgx.ErrNoRows {
			m.logger.Error("last block height not found",
				"err", err.Error(),
			)
			return
		}
		m.logger.Debug("setting height using range config")
		height = m.cfg.Range.From
	} else {
		m.logger.Debug("setting height using latest block")
		height = latest + 1
	}

	backoff, err := util.NewBackoff(
		100*time.Millisecond,
		6*time.Second, // cap the timeout at the expected consensus block time
	)
	if err != nil {
		m.logger.Error("error configuring indexer backoff policy",
			"err", err.Error(),
		)
		return
	}
	for m.cfg.Range.To == 0 || height <= m.cfg.Range.To {
		backoff.Wait()
		m.logger.Info("attempting block", "height", height)

		if err := m.processBlock(ctx, height); err != nil {
			if err == analyzer.ErrOutOfRange {
				m.logger.Info("no data available; will retry",
					"height", height,
					"retry_interval_ms", backoff.Timeout().Milliseconds(),
				)
			} else {
				m.logger.Error("error processing block",
					"height", height,
					"err", err.Error(),
				)
			}
			backoff.Failure()
			continue
		}

		m.logger.Info("processed block", "height", height)
		backoff.Success()
		height++
	}
}

// Name returns the name of the Main.
func (m *Main) Name() string {
	return consensusDamaskAnalyzerName
}

// source returns the source storage for the provided block height.
func (m *Main) source(height int64) (storage.ConsensusSourceStorage, error) {
	r := m.cfg
	if height >= r.Range.From && (r.Range.To == 0 || height <= r.Range.To) {
		return r.Source, nil
	}

	return nil, analyzer.ErrOutOfRange
}

// latestBlock returns the latest block processed by the consensus analyzer.
func (m *Main) latestBlock(ctx context.Context) (int64, error) {
	var latest int64
	if err := m.target.QueryRow(
		ctx,
		m.qf.LatestBlockQuery(),
		// ^analyzers should only analyze for a single chain ID, and we anchor this
		// at the starting block.
		consensusDamaskAnalyzerName,
	).Scan(&latest); err != nil {
		return 0, err
	}
	return latest, nil
}

func (m *Main) isGenesisProcessed(ctx context.Context) (bool, error) {
	var processed bool
	if err := m.target.QueryRow(
		ctx,
		m.qf.IsGenesisProcessedQuery(),
		m.cfg.ChainID,
		consensusDamaskAnalyzerName,
	).Scan(&processed); err != nil {
		return false, err
	}
	return processed, nil
}

func (m *Main) processGenesis(ctx context.Context) error {
	m.logger.Info("fetching genesis document")
	genesisDoc, err := m.cfg.Source.GenesisDocument(ctx)
	if err != nil {
		return err
	}

	m.logger.Info("processing genesis document")
	gen := NewGenesisProcessor(m.logger.With("height", "genesis"))
	queries, err := gen.Process(genesisDoc)
	if err != nil {
		return err
	}

	// Debug: log the SQL into a file if requested.
	debugPath := os.Getenv("CONSENSUS_DAMASK_GENESIS_DUMP")
	if debugPath != "" {
		sql := strings.Join(queries, "\n")
		if err := os.WriteFile(debugPath, []byte(sql), 0o600 /* Permissions: rw------- */); err != nil {
			gen.logger.Error("failed to write genesis sql to file", "err", err)
		} else {
			gen.logger.Info("wrote genesis sql to file", "path", debugPath)
		}
	}

	batch := &storage.QueryBatch{}
	for _, query := range queries {
		batch.Queue(query)
	}
	batch.Queue(
		m.qf.GenesisIndexingProgressQuery(),
		m.cfg.ChainID,
		consensusDamaskAnalyzerName,
	)
	if err := m.target.SendBatch(ctx, batch); err != nil {
		return err
	}
	m.logger.Info("genesis document processed")

	return nil
}

// processBlock processes the provided block, retrieving all required information
// from source storage and committing an atomically-executed batch of queries
// to target storage.
func (m *Main) processBlock(ctx context.Context, height int64) error {
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
				return f(groupCtx, height, batch)
			})
		}(f)
	}

	// Update indexing progress.
	group.Go(func() error {
		batch.Queue(
			m.qf.IndexingProgressQuery(),
			height,
			consensusDamaskAnalyzerName,
		)
		return nil
	})

	if err := group.Wait(); err != nil {
		if strings.Contains(err.Error(), "must be less than or equal to the current blockchain height") {
			return analyzer.ErrOutOfRange
		}
		return err
	}

	opName := "process_block_consensus"
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

	for _, f := range []func(*storage.QueryBatch, *storage.ConsensusBlockData) error{
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

func (m *Main) queueBlockInserts(batch *storage.QueryBatch, data *storage.ConsensusBlockData) error {
	batch.Queue(
		m.qf.ConsensusBlockInsertQuery(),
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

func (m *Main) queueEpochInserts(batch *storage.QueryBatch, data *storage.ConsensusBlockData) error {
	batch.Queue(
		m.qf.ConsensusEpochInsertQuery(),
		data.Epoch,
		data.BlockHeader.Height,
	)

	// Conclude previous epoch. Epochs start at index 0.
	if data.Epoch > 0 {
		batch.Queue(
			m.qf.ConsensusEpochUpdateQuery(),
			data.Epoch-1,
			data.BlockHeader.Height,
		)
	}

	return nil
}

func (m *Main) queueTransactionInserts(batch *storage.QueryBatch, data *storage.ConsensusBlockData) error {
	transactionInsertQuery := m.qf.ConsensusTransactionInsertQuery()
	accountNonceUpdateQuery := m.qf.ConsensusAccountNonceUpdateQuery()
	commissionsUpsertQuery := m.qf.ConsensusCommissionsUpsertQuery()

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

func (m *Main) queueEventInserts(batch *storage.QueryBatch, data *storage.ConsensusBlockData) error {
	eventInsertQuery := m.qf.ConsensusEventInsertQuery()

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
	runtimeUpsertQuery := m.qf.ConsensusRuntimeUpsertQuery()

	for _, runtimeEvent := range data.RuntimeEvents {
		var keyManager *string

		if runtimeEvent.Runtime.KeyManager != nil {
			km := runtimeEvent.Runtime.KeyManager.String()
			keyManager = &km
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
	runtimeSuspensionQuery := m.qf.ConsensusRuntimeSuspensionQuery()
	runtimeUnsuspensionQuery := m.qf.ConsensusRuntimeUnsuspensionQuery()

	for _, runtime := range data.RuntimeSuspensions {
		batch.Queue(runtimeSuspensionQuery, runtime)
	}
	for _, runtime := range data.RuntimeUnsuspensions {
		batch.Queue(runtimeUnsuspensionQuery, runtime)
	}

	return nil
}

func (m *Main) queueEntityEvents(batch *storage.QueryBatch, data *storage.RegistryData) error {
	claimedNodeInsertQuery := m.qf.ConsensusClaimedNodeInsertQuery()
	entityUpsertQuery := m.qf.ConsensusEntityUpsertQuery()

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
	nodeUpsertQuery := m.qf.ConsensusNodeUpsertQuery()
	nodeDeleteQuery := m.qf.ConsensusNodeDeleteQuery()

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

	entityMetaUpsertQuery := m.qf.ConsensusEntityMetaUpsertQuery()
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
	senderUpdateQuery := m.qf.ConsensusSenderUpdateQuery()
	receiverUpsertQuery := m.qf.ConsensusReceiverUpdateQuery()

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
	burnUpdateQuery := m.qf.ConsensusBurnUpdateQuery()

	for _, burn := range data.Burns {
		batch.Queue(burnUpdateQuery,
			burn.Owner.String(),
			burn.Amount.ToBigInt().Uint64(),
		)
	}

	return nil
}

func (m *Main) queueEscrows(batch *storage.QueryBatch, data *storage.StakingData) error {
	decreaseGeneralBalanceForEscrowUpdateQuery := m.qf.ConsensusDecreaseGeneralBalanceForEscrowUpdateQuery()
	addEscrowBalanceUpsertQuery := m.qf.ConsensusAddEscrowBalanceUpsertQuery()
	addDelegationsUpsertQuery := m.qf.ConsensusAddDelegationsUpsertQuery()
	takeEscrowUpdateQuery := m.qf.ConsensusTakeEscrowUpdateQuery()
	debondingStartEscrowBalanceUpdateQuery := m.qf.ConsensusDebondingStartEscrowBalanceUpdateQuery()
	debondingStartDelegationsUpdateQuery := m.qf.ConsensusDebondingStartDelegationsUpdateQuery()
	debondingStartDebondingDelegationsInsertQuery := m.qf.ConsensusDebondingStartDebondingDelegationsInsertQuery()
	reclaimGeneralBalanceUpdateQuery := m.qf.ConsensusReclaimGeneralBalanceUpdateQuery()
	reclaimEscrowBalanceUpdateQuery := m.qf.ConsensusReclaimEscrowBalanceUpdateQuery()
	deleteDebondingDelegationsQuery := m.qf.ConsensusDeleteDebondingDelegationsQuery()

	for _, escrow := range data.Escrows {
		switch e := escrow; {
		case e.Add != nil:
			owner := e.Add.Owner.String()
			escrower := e.Add.Escrow.String()
			amount := e.Add.Amount.ToBigInt().Uint64()
			newShares := e.Add.NewShares.ToBigInt().Uint64()
			batch.Queue(decreaseGeneralBalanceForEscrowUpdateQuery,
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
				e.DebondingStart.Amount.ToBigInt().Uint64(),
				e.DebondingStart.ActiveShares.ToBigInt().Uint64(),
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
			batch.Queue(deleteDebondingDelegationsQuery,
				e.Reclaim.Owner.String(),
				e.Reclaim.Escrow.String(),
				e.Reclaim.Shares.ToBigInt().Uint64(),
				data.Epoch,
			)
		}
	}

	return nil
}

func (m *Main) queueAllowanceChanges(batch *storage.QueryBatch, data *storage.StakingData) error {
	allowanceChangeDeleteQuery := m.qf.ConsensusAllowanceChangeDeleteQuery()
	allowanceChangeUpdateQuery := m.qf.ConsensusAllowanceChangeUpdateQuery()

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
	validatorNodeUpdateQuery := m.qf.ConsensusValidatorNodeUpdateQuery()
	for _, validator := range data.Validators {
		batch.Queue(validatorNodeUpdateQuery,
			validator.ID,
			validator.VotingPower,
		)
	}

	return nil
}

func (m *Main) queueCommitteeUpdates(batch *storage.QueryBatch, data *storage.SchedulerData) error {
	committeeMemberInsertQuery := m.qf.ConsensusCommitteeMemberInsertQuery()

	batch.Queue(m.qf.ConsensusCommitteeMembersTruncateQuery())
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
	proposalSubmissionInsertQuery := m.qf.ConsensusProposalSubmissionInsertQuery()
	proposalSubmissionCancelInsertQuery := m.qf.ConsensusProposalSubmissionCancelInsertQuery()

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
	proposalExecutionsUpdateQuery := m.qf.ConsensusProposalExecutionsUpdateQuery()

	for _, execution := range data.ProposalExecutions {
		batch.Queue(proposalExecutionsUpdateQuery,
			execution.ID,
		)
	}

	return nil
}

func (m *Main) queueFinalizations(batch *storage.QueryBatch, data *storage.GovernanceData) error {
	proposalUpdateQuery := m.qf.ConsensusProposalUpdateQuery()
	proposalInvalidVotesUpdateQuery := m.qf.ConsensusProposalInvalidVotesUpdateQuery()

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
	voteInsertQuery := m.qf.ConsensusVoteInsertQuery()

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
