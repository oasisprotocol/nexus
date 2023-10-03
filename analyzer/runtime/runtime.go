package runtime

import (
	"context"
	"fmt"
	"math/big"
	"strings"

	sdkConfig "github.com/oasisprotocol/oasis-sdk/client-sdk/go/config"
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
	mode            analyzer.BlockAnalysisMode
	source          nodeapi.RuntimeApiLite
	target          storage.TargetStorage
	logger          *log.Logger
	metrics         metrics.StorageMetrics
}

var _ block.BlockProcessor = (*processor)(nil)

// NewRuntimeAnalyzer returns a new runtime analyzer for a runtime.
func NewRuntimeAnalyzer(
	runtime common.Runtime,
	runtimeMetadata *sdkConfig.ParaTime,
	blockRange config.BlockRange,
	batchSize uint64,
	mode analyzer.BlockAnalysisMode,
	sourceClient nodeapi.RuntimeApiLite,
	target storage.TargetStorage,
	logger *log.Logger,
) (analyzer.Analyzer, error) {
	// Initialize runtime block processor.
	processor := &processor{
		runtime:         runtime,
		runtimeMetadata: runtimeMetadata,
		mode:            mode,
		source:          sourceClient,
		target:          target,
		logger:          logger.With("analyzer", runtime),
		metrics:         metrics.NewDefaultStorageMetrics(string(runtime)),
	}

	return block.NewAnalyzer(blockRange, batchSize, mode, string(runtime), processor, target, logger)
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

// Extends `batch` with a query that will register a module address derived from "<module>.<kind>".
// See `Address::from_module` in runtime-sdk (Rust) or equivalently NewAddressForModule in client-sdk (Go).
func registerModuleAddress(batch *storage.QueryBatch, module string, kind string) {
	batch.Queue(
		queries.AddressPreimageInsert,
		sdkTypes.NewAddressForModule(module, []byte(kind)), // address.            This is how the actual runtime derives its own special addresses. The computed value is often not exposed in client-sdk.
		sdkTypes.AddressV0ModuleContext.Identifier,         // context_identifier. Input to the derivation.
		int32(sdkTypes.AddressV0ModuleContext.Version),     // context_version.    Input to the derivation.
		module+"."+kind,                                    // address_data.       Input to the derivation; the part that's the most interesting to human readers.
	)
}

// Implements BlockProcessor interface.
func (m *processor) PreWork(ctx context.Context) error {
	batch := &storage.QueryBatch{}

	// Register special addresses. Not all of these are exposed in the Go client SDK; search runtime-sdk (Rust) for `Address::from_module`.
	registerModuleAddress(batch, "rewards", "reward-pool")                   // oasis1qp7x0q9qahahhjas0xde8w0v04ctp4pqzu5mhjav
	registerModuleAddress(batch, "consensus_accounts", "pending-withdrawal") // oasis1qr677rv0dcnh7ys4yanlynysvnjtk9gnsyhvm6ln
	registerModuleAddress(batch, "consensus_accounts", "pending-delegation") // oasis1qzcdegtf7aunxr5n5pw7n5xs3u7cmzlz9gwmq49r
	registerModuleAddress(batch, "accounts", "common-pool")                  // oasis1qz78phkdan64g040cvqvqpwkplfqf6tj6uwcsh30
	registerModuleAddress(batch, "accounts", "fee-accumulator")              // oasis1qp3r8hgsnphajmfzfuaa8fhjag7e0yt35cjxq0u4

	if err := m.target.SendBatch(ctx, batch); err != nil {
		return err
	}
	m.logger.Info("registered special addresses")

	return nil
}

// Implements block.BlockProcessor interface.
func (m *processor) FinalizeFastSync(ctx context.Context, lastFastSyncHeight int64) error {
	batch := &storage.QueryBatch{}

	// Recompute the account stats for all runtime accounts. (During slow-sync, these are dead-reckoned.)
	batch.Queue(queries.RuntimeAccountNumTxsRecompute, m.runtime, lastFastSyncHeight)
	batch.Queue(queries.RuntimeAccountGasForCallingRecompute, m.runtime, lastFastSyncHeight)

	if err := m.target.SendBatch(ctx, batch); err != nil {
		return err
	}
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

	// Update indexing progress.
	batch.Queue(
		queries.IndexingProgress,
		round,
		m.runtime,
		m.mode == analyzer.FastSyncMode,
	)

	opName := fmt.Sprintf("process_block_%s", m.runtime)
	timer := m.metrics.DatabaseLatencies(m.target.Name(), opName)
	defer timer.ObserveDuration()

	if err := m.target.SendBatch(ctx, batch); err != nil {
		m.metrics.DatabaseOperations(m.target.Name(), opName, "failure").Inc()
		return err
	}
	m.metrics.DatabaseOperations(m.target.Name(), opName, "success").Inc()
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
			if m.mode != analyzer.FastSyncMode {
				// We do not dead-reckon the number of transactions for accounts in fast sync mode because there are some
				// "heavy hitter" accounts (system, etc) that are involved in a large fraction of transactions, resulting in
				// DB deadlocks with as few as 2 parallel analyzers.
				// We recalculate the number of transactions for all accounts at the end of fast-sync, by aggregating the tx data.
				batch.Queue(queries.RuntimeAccountNumTxsUpsert, m.runtime, addr, 1)
			}
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
		var error_message_raw *string
		if transactionData.Error != nil {
			error_module = transactionData.Error.Module
			error_code = transactionData.Error.Code
			error_message = transactionData.Error.Message
			error_message_raw = transactionData.Error.RawMessage
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
			error_message_raw,
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

		if (transactionData.Method == "evm.Call" || transactionData.Method == "evm.Create") && transactionData.To != nil /* is nil for reverted evm.Create */ {
			// Dead-reckon gas used for calling contracts
			if m.mode != analyzer.FastSyncMode {
				batch.Queue(queries.RuntimeAccountGasForCallingUpsert,
					m.runtime,
					transactionData.To,
					transactionData.GasUsed,
				)
			}
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
			data.Header.Timestamp,
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

	// Insert EVM token addresses, and possibly dead-reckon its mutable properties (total_supply and num_transfers).
	// Implementing totalSupply() is optional for ERC721 contracts, so we have to maintain this dead-reckoned fallback.
	for addr, possibleToken := range data.PossibleTokens {
		totalSupplyChange := possibleToken.TotalSupplyChange.String()
		numTransfersChange := possibleToken.NumTransfersChange
		lastMutateRound := uint64(0)
		if possibleToken.Mutated || possibleToken.TotalSupplyChange.Cmp(&big.Int{}) != 0 || possibleToken.NumTransfersChange != 0 {
			// One of the mutable-and-queriable-from-the-runtime properties has changed in this round; mark that in the DB.
			// NOTE: If _only_ numTransfers (which we cannot query from the EVM) changed, there's seemingly little point in
			//       requesting a re-download. But in practice, totalSupply will change without an explicit burn/mint event,
			//       so it's good to re-download the token anyway.
			lastMutateRound = data.Header.Round
		}
		batch.Queue(
			queries.RuntimeEVMTokenDeltaUpsert,
			m.runtime,
			addr,
			totalSupplyChange,
			numTransfersChange,
			lastMutateRound,
		)
	}

	// Update EVM token balances (dead reckoning).
	for key, change := range data.TokenBalanceChanges {
		// Update (dead-reckon) the DB balance only if it's actually changed.
		if change != big.NewInt(0) && m.mode != analyzer.FastSyncMode {
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

	// Insert NFTs.
	for key := range data.PossibleNFTs {
		batch.Queue(queries.RuntimeEVMNFTInsert, m.runtime, key.TokenAddress, key.TokenID, data.Header.Round)
	}
}
