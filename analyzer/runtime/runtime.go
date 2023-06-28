package runtime

import (
	"context"
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"

	sdkConfig "github.com/oasisprotocol/oasis-sdk/client-sdk/go/config"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/modules/rewards"
	sdkTypes "github.com/oasisprotocol/oasis-sdk/client-sdk/go/types"

	"github.com/oasisprotocol/nexus/analyzer"
	"github.com/oasisprotocol/nexus/analyzer/block"
	"github.com/oasisprotocol/nexus/analyzer/queries"
	evm "github.com/oasisprotocol/nexus/analyzer/runtime/evm"
	uncategorized "github.com/oasisprotocol/nexus/analyzer/uncategorized"
	"github.com/oasisprotocol/nexus/common"
	"github.com/oasisprotocol/nexus/config"
	"github.com/oasisprotocol/nexus/log"
	"github.com/oasisprotocol/nexus/metrics"
	"github.com/oasisprotocol/nexus/storage"
	"github.com/oasisprotocol/nexus/storage/oasis/nodeapi"
)

// processor is the block processor for runtimes.
type processor struct {
	runtime         common.Runtime
	runtimeMetadata *sdkConfig.ParaTime
	source          nodeapi.RuntimeApiLite
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
	sourceClient nodeapi.RuntimeApiLite,
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
	blockHeader, err := m.source.GetBlockHeader(ctx, round)
	if err != nil {
		if strings.Contains(err.Error(), "roothash: block not found") {
			return analyzer.ErrOutOfRange
		}
		return err
	}
	transactionsWithResults, err := m.source.GetTransactionsWithResults(ctx, round)
	if err != nil {
		return err
	}
	rawEvents, err := m.source.GetEventsRaw(ctx, round)
	if err != nil {
		return err
	}

	// Preprocess data.
	blockData, err := ExtractRound(*blockHeader, transactionsWithResults, rawEvents, m.logger)
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
		if error_message != nil {
			// Apparently the message does need to be valid UTF-8.
			// In the rare case it's not, hex encode it.
			// https://github.com/oasisprotocol/nexus/issues/439
			if !storage.IsValidText(*error_message) {
				*error_message = hex.EncodeToString([]byte(*error_message))
			}
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

		if transactionData.ContractCandidate != nil {
			// Transaction potentially refers to a contract. Enqueue it for fetching its bytecode.
			batch.Queue(
				queries.RuntimeEVMContractCodeAnalysisInsert,
				m.runtime,
				transactionData.ContractCandidate,
			)
		}

		if transactionData.EVMContract != nil {
			batch.Queue(
				queries.RuntimeEVMContractInsert,
				m.runtime,
				transactionData.EVMContract.Address,
				transactionData.EVMContract.CreationTx,
				transactionData.EVMContract.CreationBytecode,
			)
		}
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
			eventData.TxEthHash,
			eventData.Type,
			eventData.Body,
			eventRelatedAddresses,
			eventData.EvmLogName,
			eventData.EvmLogParams,
			eventData.EvmLogSignature,
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
		// Update the DB balance only if it's actually changed.
		if change != big.NewInt(0) {
			if key.TokenAddress == evm.NativeRuntimeTokenAddress {
				batch.Queue(queries.RuntimeNativeBalanceUpdate, m.runtime, key.AccountAddress, m.nativeTokenSymbol(), change.String())
			} else {
				batch.Queue(queries.RuntimeEVMTokenBalanceUpdate, m.runtime, key.TokenAddress, key.AccountAddress, change.String())
			}
		}
		// Even for a (suspected) non-change, notify the evm_token_balances analyzer to
		// verify the correct balance by querying the EVM.
		batch.Queue(queries.RuntimeEVMTokenBalanceAnalysisInsert, m.runtime, key.TokenAddress, key.AccountAddress, data.Header.Round)
	}
}
