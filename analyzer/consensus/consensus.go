// Package consensus implements an analyzer for the consensus layer.
package consensus

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"reflect"
	"strings"

	coreCommon "github.com/oasisprotocol/oasis-core/go/common"
	"github.com/oasisprotocol/oasis-core/go/common/cbor"
	"github.com/oasisprotocol/oasis-core/go/common/crypto/signature"
	"github.com/oasisprotocol/oasis-core/go/common/quantity"
	"github.com/oasisprotocol/oasis-core/go/consensus/cometbft/crypto"
	sdkConfig "github.com/oasisprotocol/oasis-sdk/client-sdk/go/config"
	sdkTypes "github.com/oasisprotocol/oasis-sdk/client-sdk/go/types"

	"github.com/oasisprotocol/nexus/analyzer/consensus/static"
	"github.com/oasisprotocol/nexus/analyzer/util/addresses"
	beacon "github.com/oasisprotocol/nexus/coreapi/v22.2.11/beacon/api"
	"github.com/oasisprotocol/nexus/coreapi/v22.2.11/consensus/api/transaction"
	staking "github.com/oasisprotocol/nexus/coreapi/v22.2.11/staking/api"
	cometbft "github.com/oasisprotocol/nexus/coreapi/v24.0/consensus/cometbft/api"

	"github.com/oasisprotocol/nexus/analyzer"
	"github.com/oasisprotocol/nexus/analyzer/block"
	"github.com/oasisprotocol/nexus/analyzer/queries"
	"github.com/oasisprotocol/nexus/analyzer/util"
	apiTypes "github.com/oasisprotocol/nexus/api/v1/types"
	"github.com/oasisprotocol/nexus/common"
	"github.com/oasisprotocol/nexus/config"
	"github.com/oasisprotocol/nexus/log"
	"github.com/oasisprotocol/nexus/metrics"
	"github.com/oasisprotocol/nexus/storage"
	"github.com/oasisprotocol/nexus/storage/oasis/nodeapi"
)

const (
	consensusAnalyzerName = "consensus"
)

type EventType = apiTypes.ConsensusEventType // alias for brevity

type parsedEvent struct {
	eventIdx             int
	ty                   EventType
	rawBody              json.RawMessage
	roothashRuntimeID    *coreCommon.Namespace
	roothashRuntime      *common.Runtime
	roothashRuntimeRound *uint64
	relatedAddresses     []staking.Address
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
	source  nodeapi.ConsensusApiLite
	network sdkConfig.Network
	target  storage.TargetStorage
	logger  *log.Logger
	metrics metrics.AnalysisMetrics
}

var _ block.BlockProcessor = (*processor)(nil)

// NewAnalyzer returns a new analyzer for the consensus layer.
func NewAnalyzer(blockRange config.BlockRange, batchSize uint64, mode analyzer.BlockAnalysisMode, history config.History, source nodeapi.ConsensusApiLite, network sdkConfig.Network, target storage.TargetStorage, logger *log.Logger) (analyzer.Analyzer, error) {
	processor := &processor{
		mode:    mode,
		history: history,
		source:  source,
		network: network,
		target:  target,
		logger:  logger.With("analyzer", consensusAnalyzerName),
		metrics: metrics.NewDefaultAnalysisMetrics(consensusAnalyzerName),
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

	// Insert static account first activity timestamps.
	batch = &storage.QueryBatch{}
	if err = static.QueueConsensusAccountsFirstActivity(batch, m.history.ChainName, m.logger); err != nil {
		return err
	}
	if err = m.target.SendBatch(ctx, batch); err != nil {
		return err
	}
	m.logger.Info("inserted static account first active timestamps")

	return nil
}

// Implements block.BlockProcessor interface. Downloads and processes the genesis document
// that immediately precedes block `lastFastSyncHeight`+1, i.e. the first slow-sync block
// we're about to process.
// If that block is the first block of a chain (cobalt, damask, etc), we download the chain's
// original genesis document. Otherwise, we download the genesis-formatted state at `lastFastSyncHeight`.
func (m *processor) FinalizeFastSync(ctx context.Context, lastFastSyncHeight int64) error {
	// Aggregate append-only tables that were used during fast sync.
	if err := m.aggregateFastSyncTables(ctx); err != nil {
		return err
	}

	// Recompute account first activity for all accounts scanned during fast-sync.
	batch := &storage.QueryBatch{}
	m.logger.Info("computing account first activity for all accounts scanned during fast-sync")
	batch.Queue(queries.ConsensusAccountsFirstActivityRecompute)
	if err := m.target.SendBatch(ctx, batch); err != nil {
		return fmt.Errorf("recomputing consensus accounts first activity: %w", err)
	}

	// Recompute tx count for all accounts scanned during fast-sync.
	batch = &storage.QueryBatch{}
	m.logger.Info("computing tx count for all accounts scanned during fast-sync")
	batch.Queue(queries.ConsensusAccountTxCountRecompute)
	if err := m.target.SendBatch(ctx, batch); err != nil {
		return fmt.Errorf("recomputing consensus accounts tx count: %w", err)
	}

	// Fetch a data snapshot (= genesis doc) from the node; see function docstring.
	firstSlowSyncHeight := lastFastSyncHeight + 1
	r, err := m.history.RecordForHeight(firstSlowSyncHeight)
	if err != nil {
		return fmt.Errorf("no history record for first slow-sync height %d: %w", firstSlowSyncHeight, err)
	}
	var genesisDoc *nodeapi.GenesisDocument
	var nodes []nodeapi.Node
	if r.GenesisHeight == firstSlowSyncHeight {
		m.logger.Info("fetching genesis document before starting with the first block of a chain", "chain_context", r.ChainContext, "genesis_height", r.GenesisHeight)
		genesisDoc, err = m.source.GetGenesisDocument(ctx, r.ChainContext)
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

// Aggregates rows from the temporary, append-only  `todo_updates.*` tables, and appropriately updates
// the regular DB tables.
func (m *processor) aggregateFastSyncTables(ctx context.Context) error {
	batch := &storage.QueryBatch{}

	m.logger.Info("computing epoch boundaries for epochs scanned during fast-sync")
	batch.Queue(queries.ConsensusEpochsRecompute)
	batch.Queue("DELETE FROM todo_updates.epochs")
	batch.Queue(queries.ConsensusBlockSignersFinalize)
	batch.Queue("DELETE FROM todo_updates.block_signers")

	if err := m.target.SendBatch(ctx, batch); err != nil {
		return err
	}

	return nil
}

// Dumps the genesis document to a JSON file if instructed via env variables. For debug only.
func (m *processor) debugDumpGenesisJSON(genesisDoc *nodeapi.GenesisDocument, heightOrName string) {
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
func (m *processor) processGenesis(ctx context.Context, genesisDoc *nodeapi.GenesisDocument, nodesOverride []nodeapi.Node) error {
	m.logger.Info("processing genesis document")
	gen := NewGenesisProcessor(m.logger.With("height", "genesis"))
	batch, err := gen.Process(genesisDoc, nodesOverride)
	if err != nil {
		return err
	}

	// Debug: log the SQL into a file if requested.
	debugPath := os.Getenv("NEXUS_DUMP_GENESIS_SQL")
	if debugPath != "" {
		queries, err := json.Marshal(batch.Queries())
		if err != nil {
			return err
		}
		if err := os.WriteFile(debugPath, queries, 0o600 /* Permissions: rw------- */); err != nil {
			gen.logger.Error("failed to write genesis queries to file", "err", err)
		} else {
			gen.logger.Info("wrote genesis queries to file", "path", debugPath)
		}
	}

	if err := m.target.SendBatch(ctx, batch); err != nil {
		return err
	}
	m.logger.Info("genesis document processed")

	return nil
}

// Expands `batch` with DB statements that reflect the contents of `data`.
func (m *processor) queueDbUpdates(batch *storage.QueryBatch, data allData) error {
	if err := m.queueBlockInserts(batch, data.BlockData, data.StakingData.TotalSupply); err != nil {
		return err
	}

	for _, f := range []func(*storage.QueryBatch, *consensusBlockData) error{
		m.queueEpochInserts,
		m.queueTransactionInserts,
	} {
		if err := f(batch, data.BlockData); err != nil {
			return err
		}
	}
	if err := m.queueTxEventInserts(batch, &data); err != nil {
		return err
	}

	for _, f := range []func(*storage.QueryBatch, *registryData) error{
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

	for _, f := range []func(*storage.QueryBatch, *stakingData, beacon.EpochTime) error{
		m.queueRegularTransfers,
		m.queueBurns,
		m.queueEscrows,
		m.queueAllowanceChanges,
		m.queueStakingEventInserts,
		m.queueDisbursementTransfers,
	} {
		if err := f(batch, data.StakingData, data.BeaconData.Epoch); err != nil {
			return err
		}
	}

	for _, f := range []func(*storage.QueryBatch, *schedulerData) error{
		m.queueValidatorUpdates,
		m.queueCommitteeUpdates,
	} {
		if err := f(batch, data.SchedulerData); err != nil {
			return err
		}
	}

	for _, f := range []func(*storage.QueryBatch, *governanceData) error{
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

	for _, f := range []func(*storage.QueryBatch, *rootHashData) error{
		m.queueRootHashMessageUpserts,
		m.queueRootHashEventInserts,
	} {
		if err := f(batch, data.RootHashData); err != nil {
			return err
		}
	}

	return nil
}

// Implements BlockProcessor interface.
func (m *processor) ProcessBlock(ctx context.Context, uheight uint64) error {
	if uheight > math.MaxInt64 {
		return fmt.Errorf("height %d is too large", uheight)
	}
	height := int64(uheight)
	batch := &storage.QueryBatch{}

	if _, isBlockAbsent := m.history.MissingBlocks[uheight]; !isBlockAbsent {
		// Fetch all data.
		fetchTimer := m.metrics.BlockFetchLatencies()
		data, err := fetchAllData(ctx, m.source, m.network, height, m.mode == analyzer.FastSyncMode)
		if err != nil {
			m.metrics.BlockFetches(metrics.BlockFetchStatusError).Inc()
			return err
		}
		// We make no observation in case of a data fetch error; those timings are misleading.
		fetchTimer.ObserveDuration()
		m.metrics.BlockFetches(metrics.BlockFetchStatusSuccess).Inc()

		// Process data, prepare updates.
		analysisTimer := m.metrics.BlockAnalysisLatencies()
		err = m.queueDbUpdates(batch, *data)
		analysisTimer.ObserveDuration()
		if err != nil {
			return err
		}
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

func (m *processor) queueBlockInserts(batch *storage.QueryBatch, data *consensusBlockData, totalSupply *quantity.Quantity) error {
	// Prepare a mapping of node consensus addresses.
	//
	// CometBFT (formerly Tendermint) uses a truncated hash of the entity's
	// public key as its address format. Specifically, the address (a
	// [20]byte) is the first 20 bytes of the SHA-256 of the public key.
	// Since CometBFT is what oasis-core uses for its consensus mechanism,
	// these addresses are what we're given in the block metadata for the
	// proposer and signers. Because the address derivation is one-way, we
	// need a map to convert them to Oasis-style addresses (base64 public
	// keys).
	consensusToEntity := map[string]signature.PublicKey{}
	for _, n := range data.Nodes {
		consensusAddress := crypto.PublicKeyToCometBFT(common.Ptr(n.Consensus.ID)).Address().String()
		consensusToEntity[consensusAddress] = n.EntityID
	}

	var cmtMeta cometbft.BlockMeta
	if err := cmtMeta.TryUnmarshal(data.BlockHeader.Meta); err != nil {
		m.logger.Warn("could not unmarshal block meta, may be incompatible version",
			"height", data.BlockHeader.Height,
			"err", err,
		)
		// We just skip indexing the block metadata if we cannot unmarshal it
		// and don't stop indexing the rest of the block.
	}

	var proposerAddr *string
	if cmtMeta.Header != nil {
		entity, ok := consensusToEntity[cmtMeta.Header.ProposerAddress.String()]
		if !ok {
			m.logger.Warn("could not convert block proposer address to entity id (address not found)",
				"height", data.BlockHeader.Height,
				"proposer", cmtMeta.Header.ProposerAddress.String(),
			)
		} else {
			proposerAddr = common.Ptr(entity.String())
		}
	}

	var gasUsed uint64
	for _, txr := range data.TransactionsWithResults {
		gasUsed += txr.Result.GasUsed
	}

	batch.Queue(
		queries.ConsensusBlockUpsert,
		data.BlockHeader.Height,
		data.BlockHeader.Hash.Hex(),
		data.BlockHeader.Time.UTC(),
		len(data.TransactionsWithResults),
		data.GasLimit,
		gasUsed,
		data.SizeLimit,
		data.BlockHeader.Size,
		data.Epoch,
		data.BlockHeader.StateRoot.Namespace.String(),
		int64(data.BlockHeader.StateRoot.Version),
		data.BlockHeader.StateRoot.Hash.Hex(),
		proposerAddr,
		totalSupply,
	)

	if cmtMeta.LastCommit != nil && cmtMeta.LastCommit.BlockID.IsComplete() {
		prevSigners := make([]string, 0, len(cmtMeta.LastCommit.Signatures))
		for _, cs := range cmtMeta.LastCommit.Signatures {
			if cs.Absent() {
				continue
			}
			entity, ok := consensusToEntity[cs.ValidatorAddress.String()]
			if !ok {
				m.logger.Warn("could not convert block signer address to entity id (address not found)",
					"height", data.BlockHeader.Height,
					"signer", cs.ValidatorAddress.String(),
				)
				continue
			}
			prevSigners = append(prevSigners, entity.String())
		}
		switch m.mode {
		case analyzer.FastSyncMode:
			// During fast-sync, blocks are processed out of order, meaning the parent block may not yet be available.
			// To avoid missing dependencies, signers are stored in a temporary table.
			// These entries will be finalized during the fast-sync completion phase.
			batch.Queue(
				queries.ConsensusBlockAddSignersFastSync,
				cmtMeta.LastCommit.Height,
				prevSigners,
			)

		case analyzer.SlowSyncMode:
			batch.Queue(
				queries.ConsensusBlockAddSigners,
				cmtMeta.LastCommit.Height,
				prevSigners,
			)
		}
	}

	return nil
}

func (m *processor) queueEpochInserts(batch *storage.QueryBatch, data *consensusBlockData) error {
	if m.mode == analyzer.SlowSyncMode {
		// In slow-sync mode, update our knowledge about the epoch in-place.
		batch.Queue(
			queries.ConsensusEpochUpsert,
			data.Epoch,
			data.BlockHeader.Height,
		)
	} else {
		// In fast-sync mode, record the association between the height and the epoch in a temporary table, to reduce write contention.
		batch.Queue(
			queries.ConsensusFastSyncEpochHeightInsert,
			data.Epoch,
			data.BlockHeader.Height,
		)
	}

	return nil
}

// Adapted from https://github.com/oasisprotocol/oasis-core/blob/master/go/consensus/api/transaction/transaction.go#L58
func unpackTxBody(t *transaction.Transaction) (interface{}, error) {
	err := fmt.Errorf("unknown tx method")
	for _, mapping := range []map[string]interface{}{bodyTypeForTxMethodEden, bodyTypeForTxMethodDamask, bodyTypeForTxMethodCobalt} {
		bodyType, ok := mapping[string(t.Method)]
		if !ok {
			continue
		}
		v := reflect.New(reflect.TypeOf(bodyType)).Interface()
		if err = cbor.Unmarshal(t.Body, v); err != nil {
			continue
		}
		return v, nil
	}

	return nil, fmt.Errorf("unable to cbor-decode consensus tx body: %w, method: %s, body: %x", err, t.Method, t.Body)
}

func (m *processor) queueTransactionInserts(batch *storage.QueryBatch, data *consensusBlockData) error {
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

		body, err := unpackTxBody(tx)
		if err != nil {
			m.logger.Warn("failed to unpack tx body", "err", err, "tx_hash", signedTx.Hash().Hex(), "height", data.Height)
		}
		// We explicitly json-marshal the body here to ensure that any custom
		// MarshalJSON() added to the vendored types are used.
		var bodyJSON []byte
		bodyJSON, err = json.Marshal(body)
		if err != nil {
			m.logger.Warn("error json-marshalling struct", "err", err, "tx_hash", signedTx.Hash().Hex(), "height", data.Height)
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
			bodyJSON,
			module,
			result.Error.Code,
			message,
			result.GasUsed,
		)
		// Bump the nonce.
		if m.mode != analyzer.FastSyncMode { // Skip during fast sync; nonce will be provided by the genesis.
			if tx.Method != "consensus.Meta" { // consensus.Meta is a special internal tx that doesn't affect the nonce.
				batch.Queue(queries.ConsensusAccountNonceUpsert,
					sender,
					tx.Nonce+1,
				)
			}
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
func (m *processor) queueTxEventInserts(batch *storage.QueryBatch, data *allData) error {
	for i, txr := range data.BlockData.TransactionsWithResults {
		tx, err := OpenSignedTxNoVerify(&txr.Transaction)
		if err != nil {
			m.logger.Info("couldn't parse transaction",
				"err", err,
				"height", data.BlockData.Height,
				"tx_index", i,
			)
			continue
		}

		txAccounts := []staking.Address{
			// Always insert sender as a related address, some transactions (e.g. failed ones) might not have
			// any events associated.
			// TODO: this could also track the receiver (when applicable), but currently we don't do
			// much transaction parsing, where we could extract it for each transaction type.
			staking.NewAddress(txr.Transaction.Signature.PublicKey),
		}

		// Find all events associated with transaction.
		// We don't use txr.Result.Events, because those do not have the event index unique within the block.
		txEvents := make([]nodeapi.Event, 0, len(txr.Result.Events))
		for _, event := range data.GovernanceData.Events {
			if event.TxHash == txr.Transaction.Hash() {
				txEvents = append(txEvents, event)
			}
		}
		for _, event := range data.RegistryData.Events {
			if event.TxHash == txr.Transaction.Hash() {
				txEvents = append(txEvents, event)
			}
		}
		for _, event := range data.RootHashData.Events {
			if event.TxHash == txr.Transaction.Hash() {
				txEvents = append(txEvents, event)
			}
		}
		for _, event := range data.StakingData.Events {
			if event.TxHash == txr.Transaction.Hash() {
				txEvents = append(txEvents, event)
			}
		}
		// Sanity check that the number of event matches.
		if len(txEvents) != len(txr.Result.Events) {
			return fmt.Errorf("transaction %s has %d events, but only %d were found", txr.Transaction.Hash().Hex(), len(txr.Result.Events), len(txEvents))
		}

		for _, event := range txEvents {
			eventData := m.extractEventData(event)
			txAccounts = append(txAccounts, eventData.relatedAddresses...)
			accounts := extractUniqueAddresses(eventData.relatedAddresses)
			body, err := json.Marshal(eventData.rawBody)
			if err != nil {
				return err
			}

			batch.Queue(queries.ConsensusEventInsert,
				data.BlockData.Height,
				string(eventData.ty),
				eventData.eventIdx,
				string(body),
				txr.Transaction.Hash().Hex(),
				i,
				common.StringOrNil(eventData.roothashRuntimeID),
				eventData.roothashRuntime,
				eventData.roothashRuntimeRound,
			)
			batch.Queue(queries.ConsensusEventRelatedAccountsInsert,
				data.BlockData.Height,
				string(eventData.ty),
				eventData.eventIdx,
				i,
				accounts,
			)
		}
		uniqueTxAccounts := extractUniqueAddresses(txAccounts)
		for _, addr := range uniqueTxAccounts {
			batch.Queue(queries.ConsensusAccountRelatedTransactionInsert,
				addr,
				tx.Method,
				data.BlockData.Height,
				i,
			)

			if m.mode != analyzer.FastSyncMode {
				// Increment the tx count for the related account.
				// Skip in fast-sync mode; it will be recomputed at fast-sync finalization.
				batch.Queue(queries.ConsensusAccountTxCountIncrement,
					addr,
				)
				// Set the first activity for the related account if not set yet.
				// Skip in fast sync mode; it will be recomputed at fast-sync finalization.
				batch.Queue(
					queries.ConsensusAccountFirstActivityUpsert,
					addr,
					data.BlockData.BlockHeader.Time.UTC(),
				)
			}
		}
	}

	return nil
}

func (m *processor) queueRuntimeRegistrations(batch *storage.QueryBatch, data *registryData) error {
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

// RegisterConsensusAddress inserts the address preimage of the given consensus address and ID.
func RegisterConsensusAddress(batch *storage.QueryBatch, address staking.Address, id []byte) {
	batch.Queue(queries.AddressPreimageInsert,
		address,
		sdkTypes.AddressV0Ed25519Context.Identifier,
		sdkTypes.AddressV0Ed25519Context.Version,
		id,
	)
}

func (m *processor) queueEntityEvents(batch *storage.QueryBatch, data *registryData) error {
	for _, entityEvent := range data.EntityEvents {
		entityID := entityEvent.Entity.ID.String()

		for _, node := range entityEvent.Entity.Nodes {
			batch.Queue(queries.ConsensusClaimedNodeInsert,
				entityID,
				node.String(),
			)
		}

		entityAddress := staking.NewAddress(entityEvent.Entity.ID)
		batch.Queue(queries.ConsensusEntityUpsert,
			entityID,
			entityAddress.String(),
			data.Height,
		)
		RegisterConsensusAddress(batch, entityAddress, entityEvent.Entity.ID[:])
	}

	return nil
}

// Performs bookkeeping related to node (de)registrations, ignoring registrations that are already expired.
func (m *processor) queueNodeEvents(batch *storage.QueryBatch, data *registryData, currentEpoch uint64) error {
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
				nodeEvent.Roles,
				nodeEvent.SoftwareVersion,
				0,
			)
			RegisterConsensusAddress(batch, staking.NewAddress(nodeEvent.NodeID), nodeEvent.NodeID[:])

			// Update the node's runtime associations by deleting
			// previous node records and inserting new ones.
			batch.Queue(queries.ConsensusRuntimeNodesDelete, nodeEvent.NodeID.String())
			for _, rt := range nodeEvent.Runtimes {
				batch.Queue(queries.ConsensusRuntimeNodesUpsert, rt.ID.String(), nodeEvent.NodeID.String(), rt.Version, rt.RawCapabilities, rt.ExtraInfo)
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

func (m *processor) queueRegistryEventInserts(batch *storage.QueryBatch, data *registryData) error {
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

func (m *processor) queueRootHashMessageUpserts(batch *storage.QueryBatch, data *rootHashData) error {
	// Collect (I) roothash messages being scheduled and (II) roothash
	// messages being finalized. They're always scheduled in the first
	// ExecutorCommitedEvent (i.e. the proposal). They're finalized in (a)
	// MessageEvent in Cobalt and in (b) the last round results in Damask and
	// later.
	finalized := map[coreCommon.Namespace]uint64{}
	var roothashMessageEvents []nodeapi.Event
	for _, event := range data.Events {
		switch {
		case event.RoothashMisc != nil:
			switch event.Type { //nolint:gocritic,exhaustive // singleCaseSwitch, no special handling for other types
			case apiTypes.ConsensusEventTypeRoothashFinalized:
				// (II.a) MessageEvent does not have its own Round field, so
				// use the value from the FinalizedEvent that happens at the
				// same time.
				finalized[event.RoothashMisc.RuntimeID] = *event.RoothashMisc.Round
			}
		case event.RoothashExecutorCommitted != nil:
			runtime := RuntimeFromID(event.RoothashExecutorCommitted.RuntimeID, m.network)
			if runtime == nil {
				break
			}
			round := event.RoothashExecutorCommitted.Round
			// (I) Extract roothash messages from the ExecutorCommittedEvent.
			// Only the proposal has the messages, so the other commits will
			// harmlessly skip over this part.
			for i, message := range event.RoothashExecutorCommitted.Messages {
				logger := m.logger.With(
					"height", data.Height,
					"runtime", runtime,
					"round", round,
					"message_index", i,
				)
				messageData := extractMessageData(logger, message)
				// The runtime has its own staking account, which is what
				// performs these actions, e.g. when sending or receiving the
				// consensus token. Register that as related to the message.
				if runtimeAddr, err := addresses.RegisterRuntimeAddress(messageData.addressPreimages, event.RoothashExecutorCommitted.RuntimeID); err != nil {
					logger.Info("register runtime address failed",
						"runtime_id", event.RoothashExecutorCommitted.RuntimeID,
						"err", err,
					)
				} else {
					messageData.relatedAddresses[runtimeAddr] = struct{}{}
				}

				for addr, preimageData := range messageData.addressPreimages {
					batch.Queue(queries.AddressPreimageInsert,
						addr,
						preimageData.ContextIdentifier,
						preimageData.ContextVersion,
						preimageData.Data,
					)
				}
				batch.Queue(queries.ConsensusRoothashMessageScheduleUpsert,
					runtime,
					round,
					i,
					messageData.messageType,
					messageData.body,
					addresses.SliceFromSet(messageData.relatedAddresses),
				)
			}
		case event.RoothashMessage != nil:
			// (II.a) Extract message results from the MessageEvents.
			// Save these for after we collect all roothash finalized events.
			roothashMessageEvents = append(roothashMessageEvents, event)
		}
	}
	for _, event := range roothashMessageEvents {
		runtime := RuntimeFromID(event.RoothashMessage.RuntimeID, m.network)
		if runtime == nil {
			continue
		}
		batch.Queue(queries.ConsensusRoothashMessageFinalizeUpsert,
			runtime,
			finalized[event.RoothashMessage.RuntimeID],
			event.RoothashMessage.Index,
			event.RoothashMessage.Module,
			event.RoothashMessage.Code,
			nil,
		)
	}
	for rtid, results := range data.LastRoundResults {
		runtime := RuntimeFromID(rtid, m.network)
		if runtime == nil {
			// We shouldn't even have gathered last round results for unknown
			// runtimes. But prevent nil-runtime inserts anyway.
			continue
		}
		round, ok := finalized[rtid]
		if !ok {
			continue
		}
		// (II.b) Extract message results from the last round results.
		for _, message := range results.Messages {
			batch.Queue(queries.ConsensusRoothashMessageFinalizeUpsert,
				runtime,
				round,
				message.Index,
				message.Module,
				message.Code,
				cbor.Marshal(message.Result),
			)
		}
	}

	return nil
}

func (m *processor) queueRootHashEventInserts(batch *storage.QueryBatch, data *rootHashData) error {
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

func (m *processor) queueRegularTransfers(batch *storage.QueryBatch, data *stakingData, _ beacon.EpochTime) error {
	return m.queueTransfers(batch, data, TransferTypeOther)
}

func (m *processor) queueDisbursementTransfers(batch *storage.QueryBatch, data *stakingData, _ beacon.EpochTime) error {
	return m.queueTransfers(batch, data, TransferTypeAccumulatorDisbursement)
}

func (m *processor) queueTransfers(batch *storage.QueryBatch, data *stakingData, targetType TransferType) error {
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

func (m *processor) queueBurns(batch *storage.QueryBatch, data *stakingData, _ beacon.EpochTime) error {
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

func (m *processor) queueEscrows(batch *storage.QueryBatch, data *stakingData, epoch beacon.EpochTime) error {
	for _, e := range data.AddEscrows {
		owner := e.Owner.String()
		escrower := e.Escrow.String()
		amount := e.Amount.String()
		newShares := "0"
		if e.NewShares == nil {
			m.logger.Warn(
				"AddEscrowEvent with a missing `new_shares` field encountered. The absence of that field makes dead reckoning impossible. Skip over old heights with fast-sync to work around the issue.",
				"height", data.Height, "owner", owner, "escrower", escrower, "amount", amount,
			)
		} else {
			newShares = e.NewShares.String()
		}
		batch.Queue(queries.ConsensusEscrowEventInsert,
			data.Height,
			epoch,
			apiTypes.ConsensusEventTypeStakingEscrowAdd,
			escrower,
			owner,
			newShares,
			amount,
			nil, // debonding amount
		)
		// Update dead-reckoned accounts/delegations tables if in slow-sync mode.
		if m.mode == analyzer.SlowSyncMode {
			batch.Queue(queries.ConsensusDecreaseGeneralBalanceUpsert,
				owner,
				amount,
			)
			batch.Queue(queries.ConsensusAddEscrowBalanceUpsert,
				escrower,
				amount,
				newShares,
			)
			if newShares != "0" {
				// When rewards are distributed, an `AddEscrowEvent` with `new_shares` set to 0 is emitted. This makes sense,
				// since rewards increase the Escrow balance without adding new shares - it increases the existing shares price.
				//
				// Do not track these as delegations.
				batch.Queue(queries.ConsensusAddDelegationsUpsert,
					escrower,
					owner,
					newShares,
				)
			}
		}
	}
	for _, e := range data.TakeEscrows {
		debondingAmount := "0"
		if e.DebondingAmount != nil {
			debondingAmount = e.DebondingAmount.String()
		}
		batch.Queue(queries.ConsensusEscrowEventInsert,
			data.Height,
			epoch,
			apiTypes.ConsensusEventTypeStakingEscrowTake,
			e.Owner.String(),
			nil, // delegator
			nil,
			e.Amount.String(),
			debondingAmount,
		)
		// Update dead-reckoned accounts/delegations tables if in slow-sync mode.
		if m.mode == analyzer.SlowSyncMode {
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
	}
	for _, e := range data.DebondingStartEscrows {
		batch.Queue(queries.ConsensusEscrowEventInsert,
			data.Height,
			epoch,
			apiTypes.ConsensusEventTypeStakingEscrowDebondingStart,
			e.Escrow.String(),
			e.Owner.String(),
			e.ActiveShares.String(),
			e.Amount.String(),
			nil, // debonding amount
		)
		// Update dead-reckoned accounts/delegations tables if in slow-sync mode.
		if m.mode == analyzer.SlowSyncMode {
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
			// Ideally this would be merged with the ConsensusDebondingStartDelegationsUpdate query above,
			// something like:
			//
			// ```
			//   WITH updated AS (
			//       UPDATE chain.delegations
			//       SET shares = shares - $3
			//       WHERE delegatee = $1 AND delegator = $2
			//       RETURNING delegatee, delegator, shares
			//   )
			//
			//   -- Delete the delegation if the shares are now 0.
			//   DELETE FROM chain.delegations
			//     USING updated
			//     WHERE chain.delegations.delegatee = updated.delegatee
			//       AND chain.delegations.delegator = updated.delegator
			//       AND updated.shares = 0`
			// ```
			//
			// but it is not possible since CTE's cannot handle updating the same row twice:
			// https://www.postgresql.org/docs/current/queries-with.html#QUERIES-WITH-MODIFYING
			//
			// So we need to issue a separate query to delete if zero.
			batch.Queue(queries.ConsensusDelegationDeleteIfZeroShares,
				e.Escrow.String(),
				e.Owner.String(),
			)
			batch.Queue(queries.ConsensusDebondingStartDebondingDelegationsUpsert,
				e.Escrow.String(),
				e.Owner.String(),
				e.DebondingShares.String(),
				e.DebondEndTime,
			)
		}
	}
	for _, e := range data.ReclaimEscrows {
		// Update dead-reckoned accounts/delegations tables if in slow-sync mode.
		if m.mode == analyzer.SlowSyncMode {
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
				epoch,
			)
		}
	}

	return nil
}

func (m *processor) queueAllowanceChanges(batch *storage.QueryBatch, data *stakingData, _ beacon.EpochTime) error {
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

func (m *processor) queueStakingEventInserts(batch *storage.QueryBatch, data *stakingData, _ beacon.EpochTime) error {
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

func (m *processor) queueValidatorUpdates(batch *storage.QueryBatch, data *schedulerData) error {
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

func (m *processor) queueCommitteeUpdates(batch *storage.QueryBatch, data *schedulerData) error {
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

func (m *processor) queueSubmissions(batch *storage.QueryBatch, data *governanceData) error {
	if m.mode == analyzer.FastSyncMode {
		// Skip proposal tracking during fast sync.
		// The full state of proposals is present in the genesis.
		return nil
	}

	for _, submission := range data.ProposalSubmissions {
		var title, description *string
		if submission.Content.Metadata != nil {
			metadata := submission.Content.Metadata
			title = &metadata.Title
			description = &metadata.Description
		}

		switch {
		case submission.Content.Upgrade != nil:
			batch.Queue(queries.ConsensusProposalSubmissionInsert,
				submission.ID,
				submission.Submitter.String(),
				submission.State.String(),
				submission.Deposit.String(),
				title,
				description,
				submission.Content.Upgrade.Handler,
				submission.Content.Upgrade.Target.ConsensusProtocol.String(),
				submission.Content.Upgrade.Target.RuntimeHostProtocol.String(),
				submission.Content.Upgrade.Target.RuntimeCommitteeProtocol.String(),
				submission.Content.Upgrade.Epoch,
				submission.CreatedAt,
				submission.ClosesAt,
				0,
			)
		case submission.Content.CancelUpgrade != nil:
			batch.Queue(queries.ConsensusProposalSubmissionCancelInsert,
				submission.ID,
				submission.Submitter.String(),
				submission.State.String(),
				submission.Deposit.String(),
				title,
				description,
				submission.Content.CancelUpgrade.ProposalID,
				submission.CreatedAt,
				submission.ClosesAt,
				0,
			)
		case submission.Content.ChangeParameters != nil:
			batch.Queue(queries.ConsensusProposalSubmissionChangeParametersInsert,
				submission.ID,
				submission.Submitter.String(),
				submission.State.String(),
				submission.Deposit.String(),
				title,
				description,
				submission.Content.ChangeParameters.Module,
				[]byte(submission.Content.ChangeParameters.Changes),
				submission.CreatedAt,
				submission.ClosesAt,
				0,
			)
		default:
			m.logger.Warn("unknown proposal content type", "proposal_id", submission.ID, "content", submission.Content)
		}
	}

	return nil
}

func (m *processor) queueExecutions(batch *storage.QueryBatch, data *governanceData) error {
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

func (m *processor) queueFinalizations(batch *storage.QueryBatch, data *governanceData) error {
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

func (m *processor) queueVotes(batch *storage.QueryBatch, data *governanceData) error {
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
			data.Height,
		)
	}

	return nil
}

func (m *processor) queueGovernanceEventInserts(batch *storage.QueryBatch, data *governanceData) error {
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
		height,
		string(eventData.ty),
		eventData.eventIdx,
		string(body),
		nil,
		nil,
		common.StringOrNil(eventData.roothashRuntimeID),
		eventData.roothashRuntime,
		eventData.roothashRuntimeRound,
	)
	batch.Queue(queries.ConsensusEventRelatedAccountsInsert,
		height,
		string(eventData.ty),
		eventData.eventIdx,
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
		eventIdx: event.EventIdx,
		ty:       event.Type,
		rawBody:  event.RawBody,
	}

	// Fill in related accounts.
	switch {
	case event.GovernanceProposalSubmitted != nil:
		eventData.relatedAddresses = []staking.Address{event.GovernanceProposalSubmitted.Submitter}
	case event.GovernanceVote != nil:
		eventData.relatedAddresses = []staking.Address{event.GovernanceVote.Submitter}
	case event.RoothashMisc != nil:
		eventData.roothashRuntimeID = &event.RoothashMisc.RuntimeID
		eventData.roothashRuntime = RuntimeFromID(event.RoothashMisc.RuntimeID, m.network)
		eventData.roothashRuntimeRound = event.RoothashMisc.Round
	case event.RoothashExecutorCommitted != nil:
		eventData.roothashRuntimeID = &event.RoothashExecutorCommitted.RuntimeID
		eventData.roothashRuntime = RuntimeFromID(event.RoothashExecutorCommitted.RuntimeID, m.network)
		eventData.roothashRuntimeRound = &event.RoothashExecutorCommitted.Round
		if event.RoothashExecutorCommitted.NodeID != nil {
			// TODO: preimage?
			nodeAddr := staking.NewAddress(*event.RoothashExecutorCommitted.NodeID)
			eventData.relatedAddresses = []staking.Address{nodeAddr}
		}
	case event.RoothashMessage != nil:
		eventData.roothashRuntimeID = &event.RoothashMessage.RuntimeID
		eventData.roothashRuntime = RuntimeFromID(event.RoothashMessage.RuntimeID, m.network)
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
