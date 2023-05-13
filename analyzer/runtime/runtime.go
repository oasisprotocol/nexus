package runtime

import (
	"context"
	"fmt"

	sdkConfig "github.com/oasisprotocol/oasis-sdk/client-sdk/go/config"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/modules/rewards"
	sdkTypes "github.com/oasisprotocol/oasis-sdk/client-sdk/go/types"

	"github.com/oasisprotocol/oasis-indexer/analyzer"
	"github.com/oasisprotocol/oasis-indexer/analyzer/block"
	"github.com/oasisprotocol/oasis-indexer/analyzer/queries"
	uncategorized "github.com/oasisprotocol/oasis-indexer/analyzer/uncategorized"
	"github.com/oasisprotocol/oasis-indexer/common"
	"github.com/oasisprotocol/oasis-indexer/config"
	"github.com/oasisprotocol/oasis-indexer/log"
	"github.com/oasisprotocol/oasis-indexer/metrics"
	"github.com/oasisprotocol/oasis-indexer/storage"
	source "github.com/oasisprotocol/oasis-indexer/storage/oasis"
)

// processor is the block processor for runtimes.
type processor struct {
	runtime         common.Runtime
	runtimeMetadata *sdkConfig.ParaTime
	source          storage.RuntimeSourceStorage
	target          storage.TargetStorage
	logger          *log.Logger
	metrics         metrics.DatabaseMetrics
}

var _ block.BlockProcessor = (*processor)(nil)

// NewRuntimeAnalyzer returns a new runtime analyzer for a runtime.
func NewRuntimeAnalyzer(
	runtime common.Runtime,
	runtimeMetadata *sdkConfig.ParaTime,
	cfg *config.BlockBasedAnalyzerConfig,
	sourceClient *source.RuntimeClient,
	target storage.TargetStorage,
	logger *log.Logger,
) (analyzer.Analyzer, error) {
	// Initialize runtime block processor.
	processor := &processor{
		runtime:         runtime,
		runtimeMetadata: runtimeMetadata,
		source:          sourceClient,
		target:          target,
		logger:          logger.With("analyzer", runtime),
		metrics:         metrics.NewDefaultDatabaseMetrics(string(runtime)),
	}

	return block.NewAnalyzer(cfg, string(runtime), processor, target, logger, true)
}

func (m *processor) nativeTokenSymbol() string {
	return m.runtimeMetadata.Denominations[sdkConfig.NativeDenominationKey].Symbol
}

// StringifyDenomination returns a string representation of the given denomination.
// This is simply the denomination's symbol; notably, for the native denomination,
// this is looked up from network config.
func (m *processor) StringifyDenomination(d sdkTypes.Denomination) string {
	if d.IsNative() {
		return m.nativeTokenSymbol()
	}

	return d.String()
}

// Implements BlockProcessor interface.
func (m *processor) PreWork(ctx context.Context) error {
	batch := &storage.QueryBatch{}

	// Register special addresses.
	batch.Queue(
		queries.AddressPreimageInsert,
		rewards.RewardPoolAddress.String(),             // oasis1qp7x0q9qahahhjas0xde8w0v04ctp4pqzu5mhjav on mainnet oasis-3
		sdkTypes.AddressV0ModuleContext.Identifier,     // context_identifier
		int32(sdkTypes.AddressV0ModuleContext.Version), // context_version
		"rewards.reward-pool",                          // address_data (reconstructed from NewAddressForModule())
	)
	if err := m.target.SendBatch(ctx, batch); err != nil {
		return err
	}
	m.logger.Info("registered special addresses")

	return nil
}

// Implements BlockProcessor interface.
func (m *processor) ProcessBlock(ctx context.Context, round uint64) error {
	// Fetch all data.
	data, err := m.source.AllData(ctx, round)
	if err != nil {
		return err
	}

	// Preprocess data.
	blockData, err := ExtractRound(data.BlockHeader, data.TransactionsWithResults, data.RawEvents, m.logger)
	if err != nil {
		return err
	}

	// Prepare DB queries.
	batch := &storage.QueryBatch{}
	m.queueDbUpdates(batch, blockData)
	m.queueAccountsEvents(batch, blockData)
	m.queueConsensusAccountsEvents(batch, blockData)

	// Update indexing progress.
	batch.Queue(
		queries.IndexingProgress,
		round,
		m.runtime,
	)

	opName := fmt.Sprintf("process_block_%s", m.runtime)
	timer := m.metrics.DatabaseTimer(m.target.Name(), opName)
	defer timer.ObserveDuration()

	if err := m.target.SendBatch(ctx, batch); err != nil {
		m.metrics.DatabaseCounter(m.target.Name(), opName, "failure").Inc()
		return err
	}
	m.metrics.DatabaseCounter(m.target.Name(), opName, "success").Inc()
	return nil
}

// queueDbUpdates extends `batch` with queries that reflect `data`.
func (m *processor) queueDbUpdates(batch *storage.QueryBatch, data *BlockData) {
	// Block metadata.
	batch.Queue(
		queries.RuntimeBlockInsert,
		m.runtime,
		data.Header.Round,
		data.Header.Version,
		data.Header.Timestamp,
		data.Header.Hash,
		data.Header.PreviousHash.Hex(),
		data.Header.IORoot.Hex(),
		data.Header.StateRoot.Hex(),
		data.Header.MessagesHash.Hex(),
		data.Header.InMessagesHash.Hex(),
		data.NumTransactions,
		fmt.Sprintf("%d", data.GasUsed),
		data.Size,
	)

	// Insert transactions and associated data (without events).
	for _, transactionData := range data.TransactionData {
		for _, signerData := range transactionData.SignerData {
			batch.Queue(
				queries.RuntimeTransactionSignerInsert,
				m.runtime,
				data.Header.Round,
				transactionData.Index,
				signerData.Index,
				signerData.Address,
				signerData.Nonce,
			)
		}
		for addr := range transactionData.RelatedAccountAddresses {
			batch.Queue(queries.RuntimeRelatedTransactionInsert, m.runtime, addr, data.Header.Round, transactionData.Index)
		}
		var (
			evmEncryptedFormat      *common.CallFormat
			evmEncryptedPublicKey   *[]byte
			evmEncryptedDataNonce   *[]byte
			evmEncryptedDataData    *[]byte
			evmEncryptedResultNonce *[]byte
			evmEncryptedResultData  *[]byte
		)
		if transactionData.EVMEncrypted != nil {
			evmEncryptedFormat = &transactionData.EVMEncrypted.Format
			evmEncryptedPublicKey = &transactionData.EVMEncrypted.PublicKey
			evmEncryptedDataNonce = &transactionData.EVMEncrypted.DataNonce
			evmEncryptedDataData = &transactionData.EVMEncrypted.DataData
			evmEncryptedResultNonce = &transactionData.EVMEncrypted.ResultNonce
			evmEncryptedResultData = &transactionData.EVMEncrypted.ResultData
		}
		var error_module string
		var error_code uint32
		var error_message *string
		if transactionData.Error != nil {
			error_module = transactionData.Error.Module
			error_code = transactionData.Error.Code
			error_message = transactionData.Error.Message
		}
		batch.Queue(
			queries.RuntimeTransactionInsert,
			m.runtime,
			data.Header.Round,
			transactionData.Index,
			transactionData.Hash,
			transactionData.EthHash,
			&transactionData.Fee, // pgx bug? Needs a *BigInt (not BigInt) to know how to serialize.
			transactionData.GasLimit,
			transactionData.GasUsed,
			transactionData.Size,
			data.Header.Timestamp,
			transactionData.Method,
			transactionData.Body,
			transactionData.To,
			transactionData.Amount,
			evmEncryptedFormat,
			evmEncryptedPublicKey,
			evmEncryptedDataNonce,
			evmEncryptedDataData,
			evmEncryptedResultNonce,
			evmEncryptedResultData,
			transactionData.Success,
			error_module,
			error_code,
			error_message,
		)
	}

	// Insert events.
	for _, eventData := range data.EventData {
		eventRelatedAddresses := uncategorized.ExtractAddresses(eventData.RelatedAddresses)
		batch.Queue(
			queries.RuntimeEventInsert,
			m.runtime,
			data.Header.Round,
			eventData.TxIndex,
			eventData.TxHash,
			eventData.Type,
			eventData.Body,
			eventData.EvmLogName,
			eventData.EvmLogParams,
			eventRelatedAddresses,
		)
	}

	// Insert address preimages.
	for addr, preimageData := range data.AddressPreimages {
		batch.Queue(queries.AddressPreimageInsert, addr, preimageData.ContextIdentifier, preimageData.ContextVersion, preimageData.Data)
	}

	// Insert EVM token addresses.
	for addr, possibleToken := range data.PossibleTokens {
		if possibleToken.Mutated {
			batch.Queue(queries.RuntimeEVMTokenAnalysisMutateInsert, m.runtime, addr, data.Header.Round)
		} else {
			batch.Queue(queries.RuntimeEVMTokenAnalysisInsert, m.runtime, addr, data.Header.Round)
		}
	}

	// Update EVM token balances (dead reckoning).
	for key, change := range data.TokenBalanceChanges {
		batch.Queue(queries.RuntimeEVMTokenBalanceUpdate, m.runtime, key.TokenAddress, key.AccountAddress, change.String())
		batch.Queue(queries.RuntimeEVMTokenBalanceAnalysisInsert, m.runtime, key.TokenAddress, key.AccountAddress, data.Header.Round)
	}
}

// Implements BlockProcessor interface.
func (m *processor) SourceLatestBlockHeight(ctx context.Context) (uint64, error) {
	return m.source.LatestBlockHeight(ctx)
}
