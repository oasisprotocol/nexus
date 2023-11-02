// Package consensus implements an analyzer for the consensus layer.
package consensus

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"strings"

	"github.com/oasisprotocol/oasis-core/go/common/cbor"
	"github.com/oasisprotocol/oasis-core/go/common/crypto/signature"

	"github.com/oasisprotocol/nexus/coreapi/v22.2.11/consensus/api/transaction"
	genesis "github.com/oasisprotocol/nexus/coreapi/v22.2.11/genesis/api"
	staking "github.com/oasisprotocol/nexus/coreapi/v22.2.11/staking/api"

	"github.com/oasisprotocol/nexus/analyzer"
	"github.com/oasisprotocol/nexus/analyzer/block"
	"github.com/oasisprotocol/nexus/analyzer/queries"
	"github.com/oasisprotocol/nexus/analyzer/util"
	apiTypes "github.com/oasisprotocol/nexus/api/v1/types"
	"github.com/oasisprotocol/nexus/config"
	"github.com/oasisprotocol/nexus/log"
	"github.com/oasisprotocol/nexus/metrics"
	"github.com/oasisprotocol/nexus/storage"
	source "github.com/oasisprotocol/nexus/storage/oasis"
	"github.com/oasisprotocol/nexus/storage/oasis/nodeapi"
)

const (
	consensusAnalyzerName = "consensus"
)

type EventType = apiTypes.ConsensusEventType // alias for brevity

type parsedEvent struct {
	ty               EventType
	rawBody          json.RawMessage
	relatedAddresses []staking.Address
}

// OpenSignedTxNoVerify decodes the Transaction inside a Signed transaction
// without verifying the signature. Callers should be sure to check if the
// transaction actually succeeded. Nexus trusts its oasis-node to
// provide the correct transaction result, which will indicate if there was an
// authentication problem. Skipping the verification saves CPU on the analyzer.
// Due to the chain context being global, we cannot verify transactions for
// multiple networks anyway.
func OpenSignedTxNoVerify(signedTx *transaction.SignedTransaction) (*transaction.Transaction, error) {
	var tx transaction.Transaction
	if err := cbor.Unmarshal(signedTx.Blob, &tx); err != nil {
		return nil, fmt.Errorf("signed tx unmarshal: %w", err)
	}
	return &tx, nil
}

// processor is the block processor for the consensus layer.
type processor struct {
	mode    analyzer.BlockAnalysisMode
	history config.History
	source  storage.ConsensusSourceStorage
	target  storage.TargetStorage
	logger  *log.Logger
	metrics metrics.StorageMetrics
}

var _ block.BlockProcessor = (*processor)(nil)

// NewAnalyzer returns a new analyzer for the consensus layer.
func NewAnalyzer(blockRange config.BlockRange, batchSize uint64, mode analyzer.BlockAnalysisMode, history config.History, sourceClient *source.ConsensusClient, target storage.TargetStorage, logger *log.Logger) (analyzer.Analyzer, error) {
	processor := &processor{
		mode:    mode,
		history: history,
		source:  sourceClient,
		target:  target,
		logger:  logger.With("analyzer", consensusAnalyzerName),
		metrics: metrics.NewDefaultStorageMetrics(consensusAnalyzerName),
	}

	return block.NewAnalyzer(blockRange, batchSize, mode, consensusAnalyzerName, processor, target, logger)
}

// Implements BlockProcessor interface.
func (m *processor) PreWork(ctx context.Context) error {
	batch := &storage.QueryBatch{}

	// Register special addresses.
	zeroKey := signature.PublicKey{}
	zeroKeyAddr := staking.NewAddress(zeroKey).String()
	zeroKeyData, err := zeroKey.MarshalBinary()
	if err != nil {
		panic(fmt.Errorf("zero key marshal binary: %w", err))
	}
	batch.Queue(
		queries.AddressPreimageInsert,
		zeroKeyAddr,                             // oasis1qpg3hpf3vtuueyl8f8jzgsy8clqqw6qgxgurwfy5
		staking.AddressV0Context.Identifier,     // context_identifier
		int32(staking.AddressV0Context.Version), // context_version
		zeroKeyData,                             // address_data
	)
	if err = m.target.SendBatch(ctx, batch); err != nil {
		return err
	}
	m.logger.Info("registered special addresses")

	return nil
}

// Implements block.BlockProcessor interface. Downloads and processes the genesis document
// that immediately precedes block `lastFastSyncHeight`+1, i.e. the first slow-sync block
// we're about to process.
// If that block is the first block of a chain (cobalt, damask, etc), we download the chain's
// original genesis document. Otherwise, we download the genesis-formatted state at `lastFastSyncHeight`.
func (m *processor) FinalizeFastSync(ctx context.Context, lastFastSyncHeight int64) error {
	firstSlowSyncHeight := lastFastSyncHeight + 1
	r, err := m.history.RecordForHeight(firstSlowSyncHeight)
	if err != nil {
		return fmt.Errorf("no history record for first slow-sync height %d: %w", firstSlowSyncHeight, err)
	}
	var genesisDoc *genesis.Document
	var nodes []nodeapi.Node
	if r.GenesisHeight == firstSlowSyncHeight {
		m.logger.Info("fetching genesis document before starting with the first block of a chain", "chain_context", r.ChainContext, "genesis_height", r.GenesisHeight)
		genesisDoc, err = m.source.GenesisDocument(ctx, r.ChainContext)
		if err != nil {
			return err
		}
		m.debugDumpGenesisJSON(genesisDoc, r.ArchiveName)
	} else {
		m.logger.Info("fetching state at last fast-sync height, using StateToGenesis; this can take a while, up to an hour on mainnet", "state_to_genesis_height", lastFastSyncHeight, "chain_genesis_height", r.GenesisHeight, "first_slow_sync_height", firstSlowSyncHeight)
		genesisDoc, err = m.source.StateToGenesis(ctx, lastFastSyncHeight)
		if err != nil {
			return err
		}
		nodes, err = m.source.GetNodes(ctx, lastFastSyncHeight)
		if err != nil {
			return err
		}
		m.debugDumpGenesisJSON(genesisDoc, fmt.Sprintf("%d", lastFastSyncHeight))
	}

	return m.processGenesis(ctx, genesisDoc, nodes)
}

// Dumps the genesis document to a JSON file if instructed via env variables. For debug only.
func (m *processor) debugDumpGenesisJSON(genesisDoc *genesis.Document, heightOrName string) {
	debugPath := os.Getenv("NEXUS_DUMP_GENESIS") // can be templatized with "{{height}}"
	if debugPath == "" {
		return
	}
	debugPath = strings.ReplaceAll(debugPath, "{{height}}", heightOrName)
	prettyJSON, err := json.MarshalIndent(genesisDoc, "", "  ")
	if err != nil {
		m.logger.Error("failed to marshal genesis document", "err", err)
		return
	}
	if err := os.WriteFile(debugPath, prettyJSON, 0o600 /* Permissions: rw------- */); err != nil {
		m.logger.Error("failed to write genesis JSON to file", "err", err)
	} else {
		m.logger.Info("wrote genesis JSON to file", "path", debugPath, "height_or_name", heightOrName)
	}
}

// Executes SQL queries to index the contents of the genesis document.
// If nodesOverride is non-nil, it is used instead of the nodes from the genesis document.
func (m *processor) processGenesis(ctx context.Context, genesisDoc *genesis.Document, nodesOverride []nodeapi.Node) error {
	m.logger.Info("processing genesis document")
	gen := NewGenesisProcessor(m.logger.With("height", "genesis"))
	queries, err := gen.Process(genesisDoc, nodesOverride)
	if err != nil {
		return err
	}

	// Debug: log the SQL into a file if requested.
	debugPath := os.Getenv("NEXUS_DUMP_GENESIS_SQL")
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
	if err := m.target.SendBatch(ctx, batch); err != nil {
		return err
	}
	m.logger.Info("genesis document processed")

	return nil
}

// Implements BlockProcessor interface.
func (m *processor) ProcessBlock(ctx context.Context, uheight uint64) error {
	if uheight > math.MaxInt64 {
		return fmt.Errorf("height %d is too large", uheight)
	}
	height := int64(uheight)

	// Fetch all data.
	data, err := m.source.AllData(ctx, height, m.mode == analyzer.FastSyncMode)
	if err != nil {
		if strings.Contains(err.Error(), fmt.Sprintf("%d must be less than or equal to the current blockchain height", height)) {
			return analyzer.ErrOutOfRange
		}
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
			return err
		}
	}

	for _, f := range []func(*storage.QueryBatch, *storage.RegistryData) error{
		m.queueEntityEvents,
		m.queueRuntimeRegistrations,
		m.queueRegistryEventInserts,
	} {
		if err := f(batch, data.RegistryData); err != nil {
			return err
		}
	}
	if err := m.queueNodeEvents(batch, data.RegistryData, uint64(data.BlockData.Epoch)); err != nil {
		return err
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
		queries.IndexingProgress,
		height,
		consensusAnalyzerName,
		m.mode == analyzer.FastSyncMode,
	)

	// Apply updates to DB.
	opName := "process_block_consensus"
	timer := m.metrics.DatabaseLatencies(m.target.Name(), opName)
	defer timer.ObserveDuration()

	if err := m.target.SendBatch(ctx, batch); err != nil {
		m.metrics.DatabaseOperations(m.target.Name(), opName, "failure").Inc()
		return err
	}
	m.metrics.DatabaseOperations(m.target.Name(), opName, "success").Inc()
	return nil
}

func (m *processor) queueBlockInserts(batch *storage.QueryBatch, data *storage.ConsensusBlockData) error {
	batch.Queue(
		queries.ConsensusBlockInsert,
		data.BlockHeader.Height,
		data.BlockHeader.Hash.Hex(),
		data.BlockHeader.Time.UTC(),
		len(data.TransactionsWithResults),
		data.BlockHeader.StateRoot.Namespace.String(),
		int64(data.BlockHeader.StateRoot.Version),
		data.BlockHeader.StateRoot.Type.String(),
		data.BlockHeader.StateRoot.Hash.Hex(),
	)

	return nil
}

func (m *processor) queueEpochInserts(batch *storage.QueryBatch, data *storage.ConsensusBlockData) error {
	batch.Queue(
		queries.ConsensusEpochUpsert,
		data.Epoch,
		data.BlockHeader.Height,
	)

	return nil
}

func (m *processor) queueTransactionInserts(batch *storage.QueryBatch, data *storage.ConsensusBlockData) error {
	for i, txr := range data.TransactionsWithResults {
		signedTx := txr.Transaction
		result := txr.Result

		tx, err := OpenSignedTxNoVerify(&signedTx)
		if err != nil {
			m.logger.Info("couldn't parse transaction",
				"err", err,
				"height", data.Height,
				"tx_index", i,
			)
			continue
		}

		sender := staking.NewAddress(
			signedTx.Signature.PublicKey,
		).String()

		bodyBytes, err := tx.Body.MarshalCBOR()
		if err != nil {
			m.logger.Warn("failed to marshal transaction body", "err", err, "tx_hash", signedTx.Hash().Hex(), "height", data.Height)
			bodyBytes = []byte{}
		}
		var module *string
		if len(result.Error.Module) > 0 {
			module = &result.Error.Module
		}
		var message *string
		if len(result.Error.Message) > 0 {
			// The message should be well-formed since it comes from oasis-core.
			// However postgres requires valid UTF-8 with no 0x00, so we sanitize the message just in case.
			sanitizedMsg := strings.ToValidUTF8(strings.ReplaceAll(result.Error.Message, "\x00", "?"), "?")
			message = &sanitizedMsg
		}
		// Use default values for fee if tx.Fee is absent.
		fee := &transaction.Fee{}
		if tx.Fee != nil {
			fee = tx.Fee
		}
		batch.Queue(queries.ConsensusTransactionInsert,
			data.BlockHeader.Height,
			signedTx.Hash().Hex(),
			i,
			tx.Nonce,
			fee.Amount.String(),
			fmt.Sprintf("%d", fee.Gas),
			tx.Method,
			sender,
			bodyBytes,
			module,
			result.Error.Code,
			message,
		)
		if m.mode != analyzer.FastSyncMode {
			// Skip during fast sync; will be provided by the genesis.
			batch.Queue(queries.ConsensusAccountNonceUpsert,
				sender,
				tx.Nonce+1,
			)
		}

		// TODO: Use event when available
		// https://github.com/oasisprotocol/oasis-core/issues/4818
		if tx.Method == "staking.AmendCommissionSchedule" && result.IsSuccess() {
			var rawSchedule staking.AmendCommissionSchedule
			if err := cbor.Unmarshal(tx.Body, &rawSchedule); err != nil {
				return err
			}

			schedule, err := json.Marshal(rawSchedule)
			if err != nil {
				return err
			}

			if m.mode != analyzer.FastSyncMode {
				// Skip during fast sync; will be provided by the genesis.
				batch.Queue(queries.ConsensusCommissionsUpsert,
					staking.NewAddress(signedTx.Signature.PublicKey).String(),
					string(schedule),
				)
			}
		}
	}

	return nil
}

// Enqueue DB statements to store events that were generated as the result of a TX execution.
func (m *processor) queueTxEventInserts(batch *storage.QueryBatch, data *storage.ConsensusBlockData) error {
	for i, txr := range data.TransactionsWithResults {
		var txAccounts []staking.Address
		for _, event := range txr.Result.Events {
			eventData := m.extractEventData(event)
			txAccounts = append(txAccounts, eventData.relatedAddresses...)
			accounts := extractUniqueAddresses(eventData.relatedAddresses)
			body, err := json.Marshal(eventData.rawBody)
			if err != nil {
				return err
			}

			batch.Queue(queries.ConsensusEventInsert,
				string(eventData.ty),
				string(body),
				data.Height,
				txr.Transaction.Hash().Hex(),
				i,
				accounts,
			)
		}
		uniqueTxAccounts := extractUniqueAddresses(txAccounts)
		for _, addr := range uniqueTxAccounts {
			batch.Queue(queries.ConsensusAccountRelatedTransactionInsert,
				addr,
				data.Height,
				i,
			)
		}
	}

	return nil
}

func (m *processor) queueRuntimeRegistrations(batch *storage.QueryBatch, data *storage.RegistryData) error {
	// Runtime registered or (re)started.
	for _, runtimeEvent := range data.RuntimeStartedEvents {
		var keyManager *string

		if runtimeEvent.KeyManager != nil {
			km := runtimeEvent.KeyManager.String()
			keyManager = &km
		}

		if m.mode != analyzer.FastSyncMode {
			// Skip during fast sync; will be provided by the genesis.
			batch.Queue(queries.ConsensusRuntimeUpsert,
				runtimeEvent.ID.String(),
				false, // suspended
				runtimeEvent.Kind,
				runtimeEvent.TEEHardware,
				keyManager,
			)
		}
	}

	// Runtime got suspended.
	for _, runtimeEvent := range data.RuntimeSuspendedEvents {
		if m.mode != analyzer.FastSyncMode {
			// Skip during fast sync; will be provided by the genesis.
			batch.Queue(queries.ConsensusRuntimeSuspendedUpdate,
				runtimeEvent.RuntimeID.String(),
				true, // suspended
			)
		}
	}
	return nil
}

func (m *processor) queueEntityEvents(batch *storage.QueryBatch, data *storage.RegistryData) error {
	for _, entityEvent := range data.EntityEvents {
		entityID := entityEvent.Entity.ID.String()

		for _, node := range entityEvent.Entity.Nodes {
			batch.Queue(queries.ConsensusClaimedNodeInsert,
				entityID,
				node.String(),
			)
		}
		batch.Queue(queries.ConsensusEntityUpsert,
			entityID,
			staking.NewAddress(entityEvent.Entity.ID).String(),
		)
	}

	return nil
}

// Performs bookkeeping related to node (de)registrations, ignoring registrations that are already expired.
func (m *processor) queueNodeEvents(batch *storage.QueryBatch, data *storage.RegistryData, currentEpoch uint64) error {
	if m.mode == analyzer.FastSyncMode {
		// Skip node updates during fast sync; this function only modifies chain.nodes and chain.runtime_nodes,
		// which are both recreated from scratch by the genesis.
		return nil
	}

	for _, nodeEvent := range data.NodeEvents {
		if nodeEvent.IsRegistration && nodeEvent.Expiration >= currentEpoch {
			// A new node is registered; the expiration check above is needed because oasis-node sometimes returns
			// obsolete registration events, i.e. registrations that are already expired when they are produced.
			batch.Queue(queries.ConsensusNodeUpsert,
				nodeEvent.NodeID.String(),
				nodeEvent.EntityID.String(),
				nodeEvent.Expiration,
				nodeEvent.TLSPubKey.String(),
				nodeEvent.TLSNextPubKey.String(),
				nodeEvent.TLSAddresses,
				nodeEvent.P2PID.String(),
				nodeEvent.P2PAddresses,
				nodeEvent.ConsensusID.String(),
				strings.Join(nodeEvent.ConsensusAddresses, ","), // TODO: store as array
				nodeEvent.VRFPubKey,
				strings.Join(nodeEvent.Roles, ","), // TODO: store as array
				nodeEvent.SoftwareVersion,
				0,
			)

			// Update the node's runtime associations by deleting
			// previous node records and inserting new ones.
			batch.Queue(queries.ConsensusRuntimeNodesDelete, nodeEvent.NodeID.String())
			for _, runtimeID := range nodeEvent.RuntimeIDs {
				// XXX: Include other fields here if needed in the future.
				batch.Queue(queries.ConsensusRuntimeNodesUpsert, runtimeID.String(), nodeEvent.NodeID.String())
			}
		} else {
			// An existing node is expired.
			batch.Queue(queries.ConsensusRuntimeNodesDelete, nodeEvent.NodeID.String())
			batch.Queue(queries.ConsensusNodeDelete,
				nodeEvent.NodeID.String(),
			)
		}
	}

	return nil
}

func (m *processor) queueRegistryEventInserts(batch *storage.QueryBatch, data *storage.RegistryData) error {
	for _, event := range data.Events {
		hash := util.SanitizeTxHash(event.TxHash.Hex())
		if hash != nil {
			continue // Events associated with a tx are processed in queueTxEventInserts
		}

		eventData := m.extractEventData(event)

		if err := m.queueSingleEventInserts(batch, &eventData, data.Height); err != nil {
			return err
		}
	}

	return nil
}

func (m *processor) queueRootHashEventInserts(batch *storage.QueryBatch, data *storage.RootHashData) error {
	for _, event := range data.Events {
		hash := util.SanitizeTxHash(event.TxHash.Hex())
		if hash != nil {
			continue // Events associated with a tx are processed in queueTxEventInserts
		}

		eventData := m.extractEventData(event)

		if err := m.queueSingleEventInserts(batch, &eventData, data.Height); err != nil {
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

func (m *processor) queueRegularTransfers(batch *storage.QueryBatch, data *storage.StakingData) error {
	return m.queueTransfers(batch, data, TransferTypeOther)
}

func (m *processor) queueDisbursementTransfers(batch *storage.QueryBatch, data *storage.StakingData) error {
	return m.queueTransfers(batch, data, TransferTypeAccumulatorDisbursement)
}

func (m *processor) queueTransfers(batch *storage.QueryBatch, data *storage.StakingData, targetType TransferType) error {
	if m.mode == analyzer.FastSyncMode {
		// Skip dead reckoning of balances during fast sync. Genesis contains consensus balances.
		return nil
	}

	for _, transfer := range data.Transfers {
		// Filter out transfers that are not of the target type.
		typ := TransferTypeOther // type of the current transfer
		if transfer.From == staking.FeeAccumulatorAddress {
			typ = TransferTypeAccumulatorDisbursement
		}
		if typ != targetType {
			continue
		}

		batch.Queue(queries.ConsensusDecreaseGeneralBalanceUpsert,
			transfer.From.String(),
			transfer.Amount.String(),
		)
		batch.Queue(queries.ConsensusIncreaseGeneralBalanceUpsert,
			transfer.To.String(),
			transfer.Amount.String(),
		)
	}

	return nil
}

func (m *processor) queueBurns(batch *storage.QueryBatch, data *storage.StakingData) error {
	if m.mode == analyzer.FastSyncMode {
		// Skip dead reckoning of balances during fast sync. Genesis contains consensus balances.
		return nil
	}

	for _, burn := range data.Burns {
		batch.Queue(queries.ConsensusDecreaseGeneralBalanceUpsert,
			burn.Owner.String(),
			burn.Amount.String(),
		)
	}

	return nil
}

func (m *processor) queueEscrows(batch *storage.QueryBatch, data *storage.StakingData) error {
	if m.mode == analyzer.FastSyncMode {
		// Skip dead reckoning of escrows during fast sync.
		// Genesis contains all info on escrow balances (active, debonding) and delegations.
		return nil
	}

	for _, e := range data.AddEscrows {
		owner := e.Owner.String()
		escrower := e.Escrow.String()
		amount := e.Amount.String()
		newShares := e.NewShares.String()
		batch.Queue(queries.ConsensusDecreaseGeneralBalanceUpsert,
			owner,
			amount,
		)
		batch.Queue(queries.ConsensusAddEscrowBalanceUpsert,
			escrower,
			amount,
			newShares,
		)
		batch.Queue(queries.ConsensusAddDelegationsUpsert,
			escrower,
			owner,
			newShares,
		)
	}
	for _, e := range data.TakeEscrows {
		if e.DebondingAmount == nil {
			// Old-style event; the breakdown of slashed amount between active vs debonding stake
			// needs to be computed based current active vs debonding stake balance. Potentially
			// a source of rounding errors. Only needed for Cobalt and Damask; Emerald introduces
			// the .DebondingAmount field.
			if m.mode == analyzer.FastSyncMode {
				// Superstitious / hyperlocal check: Make double-sure we're not in fast-sync mode.
				return errors.New("dead-reckoning for an old-style TakeEscrowsEvent cannot be performed in fast-sync as the reckoning operation is not commutative, so blocks must be processed in order")
			}
			batch.Queue(queries.ConsensusTakeEscrowUpdateGuessRatio,
				e.Owner.String(),
				e.Amount.String(),
			)
		} else {
			batch.Queue(queries.ConsensusTakeEscrowUpdateExact,
				e.Owner.String(),
				e.Amount.String(),
				e.DebondingAmount.String(),
			)
		}
	}
	for _, e := range data.DebondingStartEscrows {
		batch.Queue(queries.ConsensusDebondingStartEscrowBalanceUpdate,
			e.Escrow.String(),
			e.Amount.String(),
			e.ActiveShares.String(),
			e.DebondingShares.String(),
		)
		batch.Queue(queries.ConsensusDebondingStartDelegationsUpdate,
			e.Escrow.String(),
			e.Owner.String(),
			e.ActiveShares.String(),
		)
		batch.Queue(queries.ConsensusDebondingStartDebondingDelegationsInsert,
			e.Escrow.String(),
			e.Owner.String(),
			e.DebondingShares.String(),
			e.DebondEndTime,
		)
	}
	for _, e := range data.ReclaimEscrows {
		batch.Queue(queries.ConsensusIncreaseGeneralBalanceUpsert,
			e.Owner.String(),
			e.Amount.String(),
		)
		batch.Queue(queries.ConsensusReclaimEscrowBalanceUpdate,
			e.Escrow.String(),
			e.Amount.String(),
			e.Shares.String(),
		)
		batch.Queue(queries.ConsensusDeleteDebondingDelegations,
			e.Owner.String(),
			e.Escrow.String(),
			e.Shares.String(),
			data.Epoch,
		)
	}

	return nil
}

func (m *processor) queueAllowanceChanges(batch *storage.QueryBatch, data *storage.StakingData) error {
	if m.mode == analyzer.FastSyncMode {
		// Skip tracking of allowances during fast sync.
		// Genesis contains all info on current allowances, and we don't track the history of allowances.
		return nil
	}

	for _, allowanceChange := range data.AllowanceChanges {
		if allowanceChange.Allowance.IsZero() {
			batch.Queue(queries.ConsensusAllowanceChangeDelete,
				allowanceChange.Owner.String(),
				allowanceChange.Beneficiary.String(),
			)
		} else {
			// A new account with no funds can still submit allowance change transactions.
			// Ensure account exists and satisfy `allowances->accounts` foreign key.
			batch.Queue(queries.ConsensusAllowanceOwnerUpsert, allowanceChange.Owner.String())
			batch.Queue(queries.ConsensusAllowanceChangeUpdate,
				allowanceChange.Owner.String(),
				allowanceChange.Beneficiary.String(),
				allowanceChange.Allowance.String(),
			)
		}
	}

	return nil
}

func (m *processor) queueStakingEventInserts(batch *storage.QueryBatch, data *storage.StakingData) error {
	for _, event := range data.Events {
		hash := util.SanitizeTxHash(event.TxHash.Hex())
		if hash != nil {
			continue // Events associated with a tx are processed in queueTxEventInserts
		}

		eventData := m.extractEventData(event)

		if err := m.queueSingleEventInserts(batch, &eventData, data.Height); err != nil {
			return err
		}
	}

	return nil
}

func (m *processor) queueValidatorUpdates(batch *storage.QueryBatch, data *storage.SchedulerData) error {
	if m.mode == analyzer.FastSyncMode {
		// Skip validator updates during fast sync.
		// The state of validators is pulled from the node at every height, and is
		// self-contained (i.e. does require some previously tracked state to calculate current validators).
		return nil
	}

	// Set all nodes' voting power to 0; the non-zero ones will be updated below.
	batch.Queue(queries.ConsensusValidatorNodeResetVotingPowers)

	for _, validator := range data.Validators {
		batch.Queue(queries.ConsensusValidatorNodeUpdateVotingPower,
			validator.ID.String(),
			validator.VotingPower,
		)
	}

	return nil
}

func (m *processor) queueCommitteeUpdates(batch *storage.QueryBatch, data *storage.SchedulerData) error {
	if m.mode == analyzer.FastSyncMode {
		// Skip committee updates during fast sync.
		// The state of committees is pulled from the node at every height, and is
		// self-contained (i.e. does require some previously tracked state to calculate current committees).
		return nil
	}

	batch.Queue(queries.ConsensusCommitteeMembersTruncate)
	for namespace, committees := range data.Committees {
		runtime := namespace.String()
		for _, committee := range committees {
			kind, err := json.Marshal(committee)
			if err != nil {
				return fmt.Errorf("error marshaling committee: %w", err)
			}
			validFor := int64(committee.ValidFor)
			for _, member := range committee.Members {
				batch.Queue(queries.ConsensusCommitteeMemberInsert,
					member.PublicKey,
					validFor,
					runtime,
					kind, // TODO: store in DB as JSON
					member.Role.String(),
				)
			}
		}
	}

	return nil
}

func (m *processor) queueSubmissions(batch *storage.QueryBatch, data *storage.GovernanceData) error {
	if m.mode == analyzer.FastSyncMode {
		// Skip proposal tracking during fast sync.
		// The full state of proposals is present in the genesis.
		return nil
	}

	for _, submission := range data.ProposalSubmissions {
		if submission.Content.Upgrade != nil {
			batch.Queue(queries.ConsensusProposalSubmissionInsert,
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
			batch.Queue(queries.ConsensusProposalSubmissionCancelInsert,
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

func (m *processor) queueExecutions(batch *storage.QueryBatch, data *storage.GovernanceData) error {
	if m.mode == analyzer.FastSyncMode {
		// Skip proposal tracking during fast sync.
		// The full state of proposals is present in the genesis.
		return nil
	}

	for _, execution := range data.ProposalExecutions {
		batch.Queue(queries.ConsensusProposalExecutionsUpdate,
			execution.ID,
		)
	}

	return nil
}

func (m *processor) queueFinalizations(batch *storage.QueryBatch, data *storage.GovernanceData) error {
	if m.mode == analyzer.FastSyncMode {
		// Skip proposal tracking during fast sync.
		// The full state of proposals is present in the genesis.
		return nil
	}

	for _, finalization := range data.ProposalFinalizations {
		batch.Queue(queries.ConsensusProposalUpdate,
			finalization.ID,
			finalization.State.String(),
		)
		batch.Queue(queries.ConsensusProposalInvalidVotesUpdate,
			finalization.ID,
			fmt.Sprintf("%d", finalization.InvalidVotes),
		)
	}

	return nil
}

func (m *processor) queueVotes(batch *storage.QueryBatch, data *storage.GovernanceData) error {
	if m.mode == analyzer.FastSyncMode {
		// Skip proposal tracking during fast sync.
		// The full state of proposals is present in the genesis.
		return nil
	}

	for _, vote := range data.Votes {
		batch.Queue(queries.ConsensusVoteUpsert,
			vote.ID,
			vote.Submitter.String(),
			vote.Vote,
		)
	}

	return nil
}

func (m *processor) queueGovernanceEventInserts(batch *storage.QueryBatch, data *storage.GovernanceData) error {
	for _, event := range data.Events {
		hash := util.SanitizeTxHash(event.TxHash.Hex())
		if hash != nil {
			continue // Events associated with a tx are processed in queueTxEventInserts
		}

		eventData := m.extractEventData(event)

		if err := m.queueSingleEventInserts(batch, &eventData, data.Height); err != nil {
			return err
		}
	}

	return nil
}

func (m *processor) queueSingleEventInserts(batch *storage.QueryBatch, eventData *parsedEvent, height int64) error {
	accounts := extractUniqueAddresses(eventData.relatedAddresses)
	body, err := json.Marshal(eventData.rawBody)
	if err != nil {
		return err
	}

	batch.Queue(queries.ConsensusEventInsert,
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
func (m *processor) extractEventData(event nodeapi.Event) parsedEvent {
	eventData := parsedEvent{
		ty:      event.Type,
		rawBody: event.RawBody,
	}

	// Fill in related accounts.
	switch {
	case event.GovernanceProposalSubmitted != nil:
		eventData.relatedAddresses = []staking.Address{event.GovernanceProposalSubmitted.Submitter}
	case event.GovernanceVote != nil:
		eventData.relatedAddresses = []staking.Address{event.GovernanceVote.Submitter}
	case event.RoothashExecutorCommitted != nil && event.RoothashExecutorCommitted.NodeID != nil:
		nodeAddr := staking.NewAddress(*event.RoothashExecutorCommitted.NodeID)
		eventData.relatedAddresses = []staking.Address{nodeAddr}
	case event.RegistryEntity != nil:
		addr := staking.NewAddress(event.RegistryEntity.Entity.ID)
		accounts := []staking.Address{addr}
		for _, node := range event.RegistryEntity.Entity.Nodes {
			nodeAddr := staking.NewAddress(node)
			accounts = append(accounts, nodeAddr)
		}
		eventData.relatedAddresses = accounts
	case event.RegistryNode != nil:
		nodeAddr := staking.NewAddress(event.RegistryNode.EntityID)
		entityAddr := staking.NewAddress(event.RegistryNode.NodeID)
		eventData.relatedAddresses = []staking.Address{nodeAddr, entityAddr}
	case event.RegistryNodeUnfrozen != nil:
		nodeAddr := staking.NewAddress(event.RegistryNodeUnfrozen.NodeID)
		eventData.relatedAddresses = []staking.Address{nodeAddr}
	case event.StakingTransfer != nil:
		eventData.relatedAddresses = []staking.Address{event.StakingTransfer.From, event.StakingTransfer.To}
	case event.StakingBurn != nil:
		eventData.relatedAddresses = []staking.Address{event.StakingBurn.Owner}
	case event.StakingAddEscrow != nil:
		eventData.relatedAddresses = []staking.Address{event.StakingAddEscrow.Owner, event.StakingAddEscrow.Escrow}
	case event.StakingTakeEscrow != nil:
		eventData.relatedAddresses = []staking.Address{event.StakingTakeEscrow.Owner}
	case event.StakingDebondingStart != nil:
		eventData.relatedAddresses = []staking.Address{event.StakingDebondingStart.Owner, event.StakingDebondingStart.Escrow}
	case event.StakingReclaimEscrow != nil:
		eventData.relatedAddresses = []staking.Address{event.StakingReclaimEscrow.Owner, event.StakingReclaimEscrow.Escrow}
	case event.StakingAllowanceChange != nil:
		eventData.relatedAddresses = []staking.Address{event.StakingAllowanceChange.Owner, event.StakingAllowanceChange.Beneficiary}
	}
	return eventData
}
