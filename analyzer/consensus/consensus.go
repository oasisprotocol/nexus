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

	"github.com/oasisprotocol/oasis-core/go/common/cbor"
	"github.com/oasisprotocol/oasis-core/go/consensus/api/transaction"
	"github.com/oasisprotocol/oasis-core/go/consensus/api/transaction/results"
	governance "github.com/oasisprotocol/oasis-core/go/governance/api"
	registry "github.com/oasisprotocol/oasis-core/go/registry/api"
	roothash "github.com/oasisprotocol/oasis-core/go/roothash/api"
	staking "github.com/oasisprotocol/oasis-core/go/staking/api"
	oasisConfig "github.com/oasisprotocol/oasis-sdk/client-sdk/go/config"

	"github.com/oasisprotocol/oasis-indexer/analyzer"
	"github.com/oasisprotocol/oasis-indexer/analyzer/util"
	apiTypes "github.com/oasisprotocol/oasis-indexer/api/v1/types"
	"github.com/oasisprotocol/oasis-indexer/config"
	"github.com/oasisprotocol/oasis-indexer/log"
	"github.com/oasisprotocol/oasis-indexer/metrics"
	"github.com/oasisprotocol/oasis-indexer/storage"
	source "github.com/oasisprotocol/oasis-indexer/storage/oasis"
)

const (
	ConsensusAnalyzerName = "consensus"

	ProcessBlockTimeout = 61 * time.Second
)

type EventType = apiTypes.ConsensusEventType // alias for brevity

type parsedEvent struct {
	ty               EventType
	body             interface{}
	relatedAddresses []staking.Address
}

// Main is the main Analyzer for the consensus layer.
type Main struct {
	cfg     analyzer.ConsensusConfig
	qf      analyzer.QueryFactory
	target  storage.TargetStorage
	logger  *log.Logger
	metrics metrics.DatabaseMetrics
}

var _ analyzer.Analyzer = (*Main)(nil)

// NewMain returns a new main analyzer for the consensus layer.
func NewMain(nodeCfg config.NodeConfig, cfg *config.BlockBasedAnalyzerConfig, target storage.TargetStorage, logger *log.Logger) (*Main, error) {
	ctx := context.Background()

	// Initialize source storage.
	networkCfg := oasisConfig.Network{
		ChainContext: nodeCfg.ChainContext,
		RPC:          nodeCfg.RPC,
	}
	factory, err := source.NewClientFactory(ctx, &networkCfg, nodeCfg.FastStartup)
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
	ac := analyzer.ConsensusConfig{
		ChainID: nodeCfg.ChainID,
		Range:   blockRange,
		Source:  client,
	}

	logger.Info("Starting consensus analyzer", "config", ac)
	return &Main{
		cfg:     ac,
		qf:      analyzer.NewQueryFactory(strcase.ToSnake(nodeCfg.ChainID), "" /* no runtime identifier for the consensus layer */),
		target:  target,
		logger:  logger.With("analyzer", ConsensusAnalyzerName),
		metrics: metrics.NewDefaultDatabaseMetrics(ConsensusAnalyzerName),
	}, nil
}

// Start starts the main consensus analyzer.
func (m *Main) Start() {
	ctx := context.Background()

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
	m.logger.Info(
		fmt.Sprintf("finished processing all blocks in the configured range [%d, %d]",
			m.cfg.Range.From, m.cfg.Range.To))
}

// Name returns the name of the Main.
func (m *Main) Name() string {
	return ConsensusAnalyzerName
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
		ConsensusAnalyzerName,
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
		ConsensusAnalyzerName,
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
		ConsensusAnalyzerName,
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
	ctxWithTimeout, cancel := context.WithTimeout(ctx, ProcessBlockTimeout)
	defer cancel()

	// Fetch all data.
	source, err := m.source(height)
	if err != nil {
		return err
	}

	data, err := source.AllData(ctxWithTimeout, height)
	if err != nil {
		return err
	}

	// Process data, prepare updates.
	batch := &storage.QueryBatch{}
	for _, f := range []func(*storage.QueryBatch, *storage.ConsensusBlockData) error{
		m.queueBlockInserts,
		m.queueEpochInserts,
		m.queueTransactionInserts,
		m.queueTxEventInserts,
	} {
		if err := f(batch, data.BlockData); err != nil {
			if strings.Contains(err.Error(), "must be less than or equal to the current blockchain height") {
				return analyzer.ErrOutOfRange
			}
			return err
		}
	}

	for _, f := range []func(*storage.QueryBatch, *storage.RegistryData) error{
		m.queueRuntimeRegistrations,
		m.queueEntityEvents,
		m.queueNodeEvents,
		m.queueRegistryEventInserts,
	} {
		if err := f(batch, data.RegistryData); err != nil {
			return err
		}
	}

	for _, f := range []func(*storage.QueryBatch, *storage.StakingData) error{
		m.queueRegularTransfers,
		m.queueBurns,
		m.queueEscrows,
		m.queueAllowanceChanges,
		m.queueStakingEventInserts,
		m.queueDisbursementTransfers,
	} {
		if err := f(batch, data.StakingData); err != nil {
			return err
		}
	}

	for _, f := range []func(*storage.QueryBatch, *storage.SchedulerData) error{
		m.queueValidatorUpdates,
		m.queueCommitteeUpdates,
	} {
		if err := f(batch, data.SchedulerData); err != nil {
			return err
		}
	}

	for _, f := range []func(*storage.QueryBatch, *storage.GovernanceData) error{
		m.queueSubmissions,
		m.queueExecutions,
		m.queueFinalizations,
		m.queueVotes,
		m.queueGovernanceEventInserts,
	} {
		if err := f(batch, data.GovernanceData); err != nil {
			return err
		}
	}

	if err := m.queueRootHashEventInserts(batch, data.RootHashData); err != nil {
		return err
	}

	// Update indexing progress.
	batch.Queue(
		m.qf.IndexingProgressQuery(),
		height,
		ConsensusAnalyzerName,
	)

	// Apply updates to DB.
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

func (m *Main) queueBlockInserts(batch *storage.QueryBatch, data *storage.ConsensusBlockData) error {
	batch.Queue(
		m.qf.ConsensusBlockInsertQuery(),
		data.BlockHeader.Height,
		data.BlockHeader.Hash.Hex(),
		data.BlockHeader.Time.UTC(),
		len(data.Transactions),
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
			tx.Fee.Amount.String(),
			fmt.Sprintf("%d", tx.Fee.Gas),
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
				staking.NewAddress(signedTx.Signature.PublicKey).String(),
				string(schedule),
			)
		}
	}

	return nil
}

// Enqueue DB statements to store events that were generated as the result of a TX execution.
func (m *Main) queueTxEventInserts(batch *storage.QueryBatch, data *storage.ConsensusBlockData) error {
	eventInsertQuery := m.qf.ConsensusEventInsertQuery()
	accountRelatedTransactionInsertQuery := m.qf.ConsensusAccountRelatedTransactionInsertQuery()

	for i := 0; i < len(data.Results); i++ {
		var txAccounts []staking.Address
		for j := 0; j < len(data.Results[i].Events); j++ {
			eventData, err := m.extractEventData(data.Results[i].Events[j])
			if err != nil {
				return err
			}
			txAccounts = append(txAccounts, eventData.relatedAddresses...)
			accounts := extractUniqueAddresses(eventData.relatedAddresses)
			body, err := json.Marshal(eventData.body)
			if err != nil {
				return err
			}

			batch.Queue(eventInsertQuery,
				string(eventData.ty),
				string(body),
				data.Height,
				data.Transactions[i].Hash().Hex(),
				i,
				accounts,
			)
		}
		uniqueTxAccounts := extractUniqueAddresses(txAccounts)
		for _, addr := range uniqueTxAccounts {
			batch.Queue(accountRelatedTransactionInsertQuery,
				addr,
				data.Height,
				i,
			)
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

func (m *Main) queueEntityEvents(batch *storage.QueryBatch, data *storage.RegistryData) error {
	claimedNodeInsertQuery := m.qf.ConsensusClaimedNodeInsertQuery()
	entityUpsertQuery := m.qf.ConsensusEntityUpsertQuery()

	for _, entityEvent := range data.EntityEvents {
		entityID := entityEvent.Entity.ID.String()

		for _, node := range entityEvent.Entity.Nodes {
			batch.Queue(claimedNodeInsertQuery,
				entityID,
				node.String(),
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

func (m *Main) queueRegistryEventInserts(batch *storage.QueryBatch, data *storage.RegistryData) error {
	for _, event := range data.Events {
		hash := util.SanitizeTxHash(event.TxHash.Hex())
		if hash != nil {
			continue // Events associated with a tx are processed in queueTxEventInserts
		}

		eventData, err := extractRegistryEvent(event)
		if err != nil {
			return err
		}

		if err = m.queueSingleEventInserts(batch, eventData, data.Height); err != nil {
			return err
		}
	}

	return nil
}

func (m *Main) queueRootHashEventInserts(batch *storage.QueryBatch, data *storage.RootHashData) error {
	for _, event := range data.Events {
		hash := util.SanitizeTxHash(event.TxHash.Hex())
		if hash != nil {
			continue // Events associated with a tx are processed in queueTxEventInserts
		}

		eventData, err := extractRootHashEvent(event)
		if err != nil {
			return err
		}

		if err = m.queueSingleEventInserts(batch, eventData, data.Height); err != nil {
			return err
		}
	}

	return nil
}

// Enum of transfer types. We single out transfers that deduct from the special
// "fee accumulator" account. These deductions/disbursements happen at the end
// of each block. However, oasis-core returns each block's events in the
// following order: BeginBlockEvents, EndBlockEvents (which include
// disbursements), TxEvents (which fill the fee accumulator). Thus, processing
// the events in order results in a temporary negative balance for the fee
// accumulator, which violates our DB checks. We therefore artificially split
// transfer events into two: accumulator disbursements, and all others. We
// process the former at the very end.
// We might be able to remove this once https://github.com/oasisprotocol/oasis-core/pull/5117
// is deployed, making oasis-core send the "correct" event order on its own. But
// Cobalt (pre-Damask network) will never be fixed.
type TransferType string

const (
	TransferTypeAccumulatorDisbursement TransferType = "AccumulatorDisbursement"
	TransferTypeOther                   TransferType = "Other"
)

func (m *Main) queueRegularTransfers(batch *storage.QueryBatch, data *storage.StakingData) error {
	return m.queueTransfers(batch, data, TransferTypeOther)
}

func (m *Main) queueDisbursementTransfers(batch *storage.QueryBatch, data *storage.StakingData) error {
	return m.queueTransfers(batch, data, TransferTypeAccumulatorDisbursement)
}

func (m *Main) queueTransfers(batch *storage.QueryBatch, data *storage.StakingData, targetType TransferType) error {
	senderUpdateQuery := m.qf.ConsensusSenderUpdateQuery()
	receiverUpsertQuery := m.qf.ConsensusReceiverUpdateQuery()

	for _, transfer := range data.Transfers {
		// Filter out transfers that are not of the target type.
		typ := TransferTypeOther // type of the current transfer
		if transfer.From == staking.FeeAccumulatorAddress {
			typ = TransferTypeAccumulatorDisbursement
		}
		if typ != targetType {
			continue
		}

		batch.Queue(senderUpdateQuery,
			transfer.From.String(),
			transfer.Amount.String(),
		)
		batch.Queue(receiverUpsertQuery,
			transfer.To.String(),
			transfer.Amount.String(),
		)
	}

	return nil
}

func (m *Main) queueBurns(batch *storage.QueryBatch, data *storage.StakingData) error {
	burnUpdateQuery := m.qf.ConsensusBurnUpdateQuery()

	for _, burn := range data.Burns {
		batch.Queue(burnUpdateQuery,
			burn.Owner.String(),
			burn.Amount.String(),
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
			amount := e.Add.Amount.String()
			newShares := e.Add.NewShares.String()
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
				e.Take.Amount.String(),
			)
		case e.DebondingStart != nil:
			batch.Queue(debondingStartEscrowBalanceUpdateQuery,
				e.DebondingStart.Escrow.String(),
				e.DebondingStart.Amount.String(),
				e.DebondingStart.ActiveShares.String(),
				e.DebondingStart.DebondingShares.String(),
			)
			batch.Queue(debondingStartDelegationsUpdateQuery,
				e.DebondingStart.Escrow.String(),
				e.DebondingStart.Owner.String(),
				e.DebondingStart.ActiveShares.String(),
			)
			batch.Queue(debondingStartDebondingDelegationsInsertQuery,
				e.DebondingStart.Escrow.String(),
				e.DebondingStart.Owner.String(),
				e.DebondingStart.DebondingShares.String(),
				e.DebondingStart.DebondEndTime,
			)
		case e.Reclaim != nil:
			batch.Queue(reclaimGeneralBalanceUpdateQuery,
				e.Reclaim.Owner.String(),
				e.Reclaim.Amount.String(),
			)
			batch.Queue(reclaimEscrowBalanceUpdateQuery,
				e.Reclaim.Escrow.String(),
				e.Reclaim.Amount.String(),
				e.Reclaim.Shares.String(),
			)
			batch.Queue(deleteDebondingDelegationsQuery,
				e.Reclaim.Owner.String(),
				e.Reclaim.Escrow.String(),
				e.Reclaim.Shares.String(),
				data.Epoch,
			)
		}
	}

	return nil
}

func (m *Main) queueAllowanceChanges(batch *storage.QueryBatch, data *storage.StakingData) error {
	allowanceChangeDeleteQuery := m.qf.ConsensusAllowanceChangeDeleteQuery()
	allowanceChangeUpdateQuery := m.qf.ConsensusAllowanceChangeUpdateQuery()
	allowanceOwnerUpsertQuery := m.qf.ConsensusAllowanceOwnerUpsertQuery()

	for _, allowanceChange := range data.AllowanceChanges {
		if allowanceChange.Allowance.IsZero() {
			batch.Queue(allowanceChangeDeleteQuery,
				allowanceChange.Owner.String(),
				allowanceChange.Beneficiary.String(),
			)
		} else {
			// A new account with no funds can still submit allowance change transactions.
			// Ensure account exists and satisfy `allowances->accounts` foreign key.
			batch.Queue(allowanceOwnerUpsertQuery, allowanceChange.Owner.String())
			batch.Queue(allowanceChangeUpdateQuery,
				allowanceChange.Owner.String(),
				allowanceChange.Beneficiary.String(),
				allowanceChange.Allowance.String(),
			)
		}
	}

	return nil
}

func (m *Main) queueStakingEventInserts(batch *storage.QueryBatch, data *storage.StakingData) error {
	for _, event := range data.Events {
		hash := util.SanitizeTxHash(event.TxHash.Hex())
		if hash != nil {
			continue // Events associated with a tx are processed in queueTxEventInserts
		}

		eventData, err := extractStakingEvent(event)
		if err != nil {
			return err
		}

		if err = m.queueSingleEventInserts(batch, eventData, data.Height); err != nil {
			return err
		}
	}

	return nil
}

func (m *Main) queueValidatorUpdates(batch *storage.QueryBatch, data *storage.SchedulerData) error {
	validatorNodeUpdateQuery := m.qf.ConsensusValidatorNodeUpdateQuery()
	for _, validator := range data.Validators {
		batch.Queue(validatorNodeUpdateQuery,
			validator.ID.String(),
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

func (m *Main) queueSubmissions(batch *storage.QueryBatch, data *storage.GovernanceData) error {
	proposalSubmissionInsertQuery := m.qf.ConsensusProposalSubmissionInsertQuery()
	proposalSubmissionCancelInsertQuery := m.qf.ConsensusProposalSubmissionCancelInsertQuery()

	for _, submission := range data.ProposalSubmissions {
		if submission.Content.Upgrade != nil {
			batch.Queue(proposalSubmissionInsertQuery,
				submission.ID,
				submission.Submitter.String(),
				submission.State.String(),
				submission.Deposit.String(),
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
				submission.Deposit.String(),
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
			fmt.Sprintf("%d", finalization.InvalidVotes),
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

func (m *Main) queueGovernanceEventInserts(batch *storage.QueryBatch, data *storage.GovernanceData) error {
	for _, event := range data.Events {
		hash := util.SanitizeTxHash(event.TxHash.Hex())
		if hash != nil {
			continue // Events associated with a tx are processed in queueTxEventInserts
		}

		eventData, err := extractGovernanceEvent(event)
		if err != nil {
			return err
		}

		if err = m.queueSingleEventInserts(batch, eventData, data.Height); err != nil {
			return err
		}
	}

	return nil
}

func (m *Main) queueSingleEventInserts(batch *storage.QueryBatch, eventData *parsedEvent, height int64) error {
	accounts := extractUniqueAddresses(eventData.relatedAddresses)
	body, err := json.Marshal(eventData.body)
	if err != nil {
		return err
	}

	batch.Queue(m.qf.ConsensusEventInsertQuery(),
		string(eventData.ty),
		string(body),
		height,
		nil,
		nil,
		accounts,
	)

	return nil
}

func extractUniqueAddresses(accounts []staking.Address) []string {
	var uniqueAccounts []string
	seen := make(map[string]struct{})
	for _, addr := range accounts {
		account := addr.String()
		_, exists := seen[account]
		if !exists {
			uniqueAccounts = append(uniqueAccounts, account)
			seen[account] = struct{}{}
		}
	}

	return uniqueAccounts
}

// extractEventData extracts the type, the body (JSON-serialized), and the related accounts of an event.
func (m *Main) extractEventData(event *results.Event) (*parsedEvent, error) {
	switch e := event; {
	case e.Staking != nil:
		return extractStakingEvent(event.Staking)
	case e.Registry != nil:
		return extractRegistryEvent(event.Registry)
	case e.RootHash != nil:
		return extractRootHashEvent(event.RootHash)
	case e.Governance != nil:
		return extractGovernanceEvent(event.Governance)
	}

	m.logger.Error("cannot infer consensus event type from event struct", "event", event)

	return &parsedEvent{
		ty:               EventType("unknown"),
		body:             []byte{},
		relatedAddresses: []staking.Address{},
	}, errors.New("unknown event type")
}

func extractGovernanceEvent(event *governance.Event) (*parsedEvent, error) {
	var eventData parsedEvent
	var err error
	switch event := event; {
	case event.ProposalSubmitted != nil:
		eventData.ty = apiTypes.ConsensusEventTypeGovernanceProposalSubmitted
		eventData.body = event.ProposalSubmitted
		eventData.relatedAddresses = []staking.Address{event.ProposalSubmitted.Submitter}
	case event.ProposalExecuted != nil:
		eventData.ty = apiTypes.ConsensusEventTypeGovernanceProposalExecuted
		eventData.body = event.ProposalExecuted
	case event.ProposalFinalized != nil:
		eventData.ty = apiTypes.ConsensusEventTypeGovernanceProposalFinalized
		eventData.body = event.ProposalFinalized
	case event.Vote != nil:
		eventData.ty = apiTypes.ConsensusEventTypeGovernanceVote
		eventData.body = event.Vote
		eventData.relatedAddresses = []staking.Address{event.Vote.Submitter}
	default:
		err = fmt.Errorf("unsupported registry event type: %#v", event)
	}
	return &eventData, err
}

func extractRootHashEvent(event *roothash.Event) (*parsedEvent, error) {
	var eventData parsedEvent
	var err error
	switch event := event; {
	case event.ExecutorCommitted != nil:
		eventData.ty = apiTypes.ConsensusEventTypeRoothashExecutorCommitted
		nodeAddr := staking.NewAddress(event.ExecutorCommitted.Commit.NodeID)
		eventData.body = event.ExecutorCommitted
		eventData.relatedAddresses = []staking.Address{nodeAddr}
	case event.ExecutionDiscrepancyDetected != nil:
		eventData.ty = apiTypes.ConsensusEventTypeRoothashExecutionDiscrepancy
		eventData.body = event.ExecutionDiscrepancyDetected
	case event.Finalized != nil:
		eventData.ty = apiTypes.ConsensusEventTypeRoothashFinalized
		eventData.body = event.Finalized
	default:
		err = fmt.Errorf("unsupported registry event type: %#v", event)
	}
	return &eventData, err
}

func extractRegistryEvent(event *registry.Event) (*parsedEvent, error) {
	var eventData parsedEvent
	var err error
	switch event := event; {
	case event.RuntimeEvent != nil:
		eventData.ty = apiTypes.ConsensusEventTypeRegistryRuntime
		eventData.body = event.RuntimeEvent
	case event.EntityEvent != nil:
		eventData.ty = apiTypes.ConsensusEventTypeRegistryEntity
		eventData.body = event.EntityEvent
		addr := staking.NewAddress(event.EntityEvent.Entity.ID)
		accounts := []staking.Address{addr}
		for _, node := range event.EntityEvent.Entity.Nodes {
			nodeAddr := staking.NewAddress(node)
			accounts = append(accounts, nodeAddr)
		}
		eventData.relatedAddresses = accounts
	case event.NodeEvent != nil:
		eventData.ty = apiTypes.ConsensusEventTypeRegistryNode
		eventData.body = event.NodeEvent
		nodeAddr := staking.NewAddress(event.NodeEvent.Node.EntityID)
		entityAddr := staking.NewAddress(event.NodeEvent.Node.ID)
		eventData.relatedAddresses = []staking.Address{nodeAddr, entityAddr}
	case event.NodeUnfrozenEvent != nil:
		eventData.ty = apiTypes.ConsensusEventTypeRegistryNodeUnfrozen
		eventData.body = event.NodeUnfrozenEvent
		nodeAddr := staking.NewAddress(event.NodeUnfrozenEvent.NodeID)
		eventData.relatedAddresses = []staking.Address{nodeAddr}
	default:
		err = fmt.Errorf("unsupported registry event type: %#v", event)
	}
	return &eventData, err
}

func extractStakingEvent(event *staking.Event) (*parsedEvent, error) {
	var eventData parsedEvent
	var err error
	switch event := event; {
	case event.Transfer != nil:
		eventData.ty = apiTypes.ConsensusEventTypeStakingTransfer
		eventData.body = event.Transfer
		eventData.relatedAddresses = []staking.Address{event.Transfer.From, event.Transfer.To}
	case event.Burn != nil:
		eventData.ty = apiTypes.ConsensusEventTypeStakingBurn
		eventData.body = event.Burn
		eventData.relatedAddresses = []staking.Address{event.Burn.Owner}
	case event.Escrow != nil:
		switch t := event.Escrow; {
		case t.Add != nil:
			eventData.ty = apiTypes.ConsensusEventTypeStakingEscrowAdd
			eventData.body = event.Escrow.Add
			eventData.relatedAddresses = []staking.Address{t.Add.Owner, t.Add.Escrow}
		case t.Take != nil:
			eventData.ty = apiTypes.ConsensusEventTypeStakingEscrowTake
			eventData.body = event.Escrow.Take
			eventData.relatedAddresses = []staking.Address{t.Take.Owner}
		case t.DebondingStart != nil:
			eventData.ty = apiTypes.ConsensusEventTypeStakingEscrowDebondingStart
			eventData.body = event.Escrow.DebondingStart
			eventData.relatedAddresses = []staking.Address{t.DebondingStart.Owner, t.DebondingStart.Escrow}
		case t.Reclaim != nil:
			eventData.ty = apiTypes.ConsensusEventTypeStakingEscrowReclaim
			eventData.body = event.Escrow.Reclaim
			eventData.relatedAddresses = []staking.Address{t.Reclaim.Owner, t.Reclaim.Escrow}
		}
	case event.AllowanceChange != nil:
		eventData.ty = apiTypes.ConsensusEventTypeStakingAllowanceChange
		eventData.body, err = json.Marshal(event.AllowanceChange)
		eventData.relatedAddresses = []staking.Address{event.AllowanceChange.Owner, event.AllowanceChange.Beneficiary}
	default:
		err = fmt.Errorf("unsupported registry event type: %#v", event)
	}
	return &eventData, err
}
