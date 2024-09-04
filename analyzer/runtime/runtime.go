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
	"github.com/oasisprotocol/nexus/analyzer/runtime/static"
	"github.com/oasisprotocol/nexus/analyzer/util/addresses"
	apiTypes "github.com/oasisprotocol/nexus/api/v1/types"
	"github.com/oasisprotocol/nexus/common"
	"github.com/oasisprotocol/nexus/config"
	"github.com/oasisprotocol/nexus/log"
	"github.com/oasisprotocol/nexus/metrics"
	"github.com/oasisprotocol/nexus/storage"
	"github.com/oasisprotocol/nexus/storage/oasis/nodeapi"
)

// processor is the block processor for runtimes.
type processor struct {
	chain   common.ChainName
	runtime common.Runtime
	sdkPT   *sdkConfig.ParaTime
	mode    analyzer.BlockAnalysisMode
	source  nodeapi.RuntimeApiLite
	target  storage.TargetStorage
	logger  *log.Logger
	metrics metrics.AnalysisMetrics
}

var _ block.BlockProcessor = (*processor)(nil)

// NewRuntimeAnalyzer returns a new runtime analyzer for a runtime.
func NewRuntimeAnalyzer(
	chain common.ChainName,
	runtime common.Runtime,
	sdkPT *sdkConfig.ParaTime,
	blockRange config.BlockRange,
	batchSize uint64,
	mode analyzer.BlockAnalysisMode,
	sourceClient nodeapi.RuntimeApiLite,
	target storage.TargetStorage,
	logger *log.Logger,
) (analyzer.Analyzer, error) {
	// Initialize runtime block processor.
	processor := &processor{
		chain:   chain,
		runtime: runtime,
		sdkPT:   sdkPT,
		mode:    mode,
		source:  sourceClient,
		target:  target,
		logger:  logger.With("analyzer", runtime),
		metrics: metrics.NewDefaultAnalysisMetrics(string(runtime)),
	}

	return block.NewAnalyzer(blockRange, batchSize, mode, string(runtime), processor, target, logger)
}

func nativeTokenSymbol(sdkPT *sdkConfig.ParaTime) string {
	return sdkPT.Denominations[sdkConfig.NativeDenominationKey].Symbol
}

// stringifyDenomination returns a string representation of the given denomination.
// This is simply the denomination's symbol; notably, for the native denomination,
// this is looked up from network config.
func stringifyDenomination(sdkPT *sdkConfig.ParaTime, d sdkTypes.Denomination) string {
	if d.IsNative() {
		return nativeTokenSymbol(sdkPT)
	}

	return d.String()
}

// Extends `batch` with a query that will register a module address derived from "<module>.<kind>".
// See `Address::from_module` in runtime-sdk (Rust) or equivalently NewAddressForModule in client-sdk (Go).
func registerModuleAddress(batch *storage.QueryBatch, daddr derivedAddr) {
	batch.Queue(
		queries.AddressPreimageInsert,
		daddr.address,                                  // address.            This is how the actual runtime derives its own special addresses. The computed value is often not exposed in client-sdk.
		sdkTypes.AddressV0ModuleContext.Identifier,     // context_identifier. Input to the derivation.
		int32(sdkTypes.AddressV0ModuleContext.Version), // context_version.    Input to the derivation.
		daddr.module+"."+daddr.kind,                    // address_data.       Input to the derivation; the part that's the most interesting to human readers.
	)
}

// Implements BlockProcessor interface.
func (m *processor) PreWork(ctx context.Context) error {
	batch := &storage.QueryBatch{}

	// Register special addresses.
	registerModuleAddress(batch, rewardsRewardPool)
	registerModuleAddress(batch, consensusAccountsPendingWithdrawal)
	registerModuleAddress(batch, consensusAccountsPendingDelegation)
	registerModuleAddress(batch, accountsCommonPool)
	registerModuleAddress(batch, accountsFeeAccumulator)
	// (Another "special" address: ethereum 0x0 derives to oasis1qq2v39p9fqk997vk6742axrzqyu9v2ncyuqt8uek. In practice, it is registered because people submitted txs to it.)

	if err := m.target.SendBatch(ctx, batch); err != nil {
		return err
	}
	m.logger.Info("registered special addresses")

	return nil
}

func (m *processor) UpdateHighTrafficAccounts(ctx context.Context, batch *storage.QueryBatch, height int64) error {
	if height < 0 {
		panic(fmt.Sprintf("negative height: %d", height))
	}
	for _, addr := range veryHighTrafficAccounts {
		balances, err := m.source.GetBalances(ctx, uint64(height), nodeapi.Address(addr))
		if err != nil {
			return fmt.Errorf("failed to get native balance for %s: %w", addr, err)
		}
		balance := common.NativeBalance(balances)

		batch.Queue(
			queries.RuntimeNativeBalanceAbsoluteUpsert,
			m.runtime,
			addr,
			nativeTokenSymbol(m.sdkPT),
			balance.String(),
		)
	}

	return nil
}

// Implements block.BlockProcessor interface.
func (m *processor) FinalizeFastSync(ctx context.Context, lastFastSyncHeight int64) error {
	// Runtimes don't have a genesis document. So if we're starting slow sync at the beginning
	// of the chain (round 0), there's no pre-work to do.
	if lastFastSyncHeight == -1 {
		return nil
	}

	batch := &storage.QueryBatch{}

	// Recompute the account stats for all runtime accounts. (During slow-sync, these are dead-reckoned.)
	m.logger.Info("recomputing number of txs for every account")
	batch.Queue(queries.RuntimeAccountNumTxsRecompute, m.runtime, lastFastSyncHeight)

	m.logger.Info("recomputing total_sent for every account")
	batch.Queue(queries.RuntimeAccountTotalSentRecompute, m.runtime, lastFastSyncHeight)

	m.logger.Info("recomputing total_received for every account")
	batch.Queue(queries.RuntimeAccountTotalReceivedRecompute, m.runtime, lastFastSyncHeight, nativeTokenSymbol(m.sdkPT))

	m.logger.Info("recomputing gas_for_calling for every contract")
	batch.Queue(queries.RuntimeAccountGasForCallingRecompute, m.runtime, lastFastSyncHeight)

	// Re-fetch the balance for high-traffic accounts. (Those are ignored during fast-sync)
	m.logger.Info("fetching current native balances of high-traffic accounts")
	if err := m.UpdateHighTrafficAccounts(ctx, batch, lastFastSyncHeight); err != nil {
		return err
	}

	// Update tables where fast-sync was not updating their columns in-place, and was instead writing
	// a log of updates to a temporary table.
	m.logger.Info("recomputing last_mutate_round for EVM token balances")
	batch.Queue(queries.RuntimeEVMTokenBalanceAnalysisMutateRoundRecompute, m.runtime)
	batch.Queue("DELETE FROM todo_updates.evm_token_balances WHERE runtime = $1", m.runtime)

	m.logger.Info("recomputing properties for EVM tokens")
	batch.Queue(queries.RuntimeEVMTokenRecompute, m.runtime)
	batch.Queue("DELETE FROM todo_updates.evm_tokens WHERE runtime = $1", m.runtime)

	if err := m.target.SendBatch(ctx, batch); err != nil {
		return err
	}
	return nil
}

// Implements BlockProcessor interface.
func (m *processor) ProcessBlock(ctx context.Context, round uint64) error {
	// Fetch all data.
	fetchTimer := m.metrics.BlockFetchLatencies()
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
	fetchTimer.ObserveDuration() // We make no observation in case of a data fetch error; those timings are misleading.

	// Preprocess data.
	analysisTimer := m.metrics.BlockAnalysisLatencies()
	blockData, err := ExtractRound(*blockHeader, transactionsWithResults, rawEvents, m.sdkPT, m.logger)
	if err != nil {
		return err
	}

	// Prepare DB queries.
	batch := &storage.QueryBatch{}
	m.queueDbUpdates(batch, blockData)
	m.queueAccountsEvents(batch, blockData)
	analysisTimer.ObserveDuration()

	// Update indexing progress.
	batch.Queue(
		queries.IndexingProgress,
		round,
		m.runtime,
		m.mode == analyzer.FastSyncMode,
	)

	// Perform one-off fixes: Refetch native balances that are known to be stale at a fixed height.
	if err := static.QueueEVMKnownStaleAccounts(batch, m.chain, m.runtime, round, m.logger); err != nil {
		return fmt.Errorf("queue eden accounts: %w", err)
	}

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
		var errorModule *string
		var errorCode *uint32
		var errorMessage *string
		var errorMessageRaw *string
		if transactionData.Error != nil {
			errorModule = &transactionData.Error.Module
			errorCode = &transactionData.Error.Code
			errorMessage = transactionData.Error.Message
			errorMessageRaw = transactionData.Error.RawMessage
		}
		batch.Queue(
			queries.RuntimeTransactionInsert,
			m.runtime,
			data.Header.Round,
			transactionData.Index,
			transactionData.Hash,
			transactionData.EthHash,
			&transactionData.Fee, // pgx bug? Needs a *BigInt (not BigInt) to know how to serialize.
			transactionData.FeeSymbol,
			transactionData.FeeProxyModule,
			transactionData.FeeProxyID,
			transactionData.GasLimit,
			transactionData.GasUsed,
			transactionData.Size,
			data.Header.Timestamp,
			transactionData.Method,
			transactionData.Body,
			transactionData.To,
			transactionData.Amount,
			transactionData.AmountSymbol,
			evmEncryptedFormat,
			evmEncryptedPublicKey,
			evmEncryptedDataNonce,
			evmEncryptedDataData,
			evmEncryptedResultNonce,
			evmEncryptedResultData,
			transactionData.Success,
			errorModule,
			errorCode,
			errorMessageRaw,
			errorMessage,
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
				queries.RuntimeEVMContractCreationUpsert,
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
		eventRelatedAddresses := addresses.SliceFromSet(eventData.RelatedAddresses)
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
		if m.mode != analyzer.FastSyncMode {
			// In slow-sync mode, directly update or dead-reckon values.
			batch.Queue(
				queries.RuntimeEVMTokenDeltaUpsert,
				m.runtime, addr, totalSupplyChange, numTransfersChange, lastMutateRound,
			)
		} else {
			// In fast-sync mode, just record the intent to update the values.
			batch.Queue(
				queries.RuntimeFastSyncEVMTokenDeltaInsert,
				m.runtime, addr, totalSupplyChange, numTransfersChange, lastMutateRound,
			)
		}
	}

	// Update EVM token balances (dead reckoning).
	for key, change := range data.TokenBalanceChanges {
		// Update (dead-reckon) the DB balance only if it's actually changed.
		if change != big.NewInt(0) && m.mode != analyzer.FastSyncMode {
			if key.TokenAddress == evm.NativeRuntimeTokenAddress {
				batch.Queue(queries.RuntimeNativeBalanceUpsert, m.runtime, key.AccountAddress, nativeTokenSymbol(m.sdkPT), change.String())
			} else {
				batch.Queue(queries.RuntimeEVMTokenBalanceUpdate, m.runtime, key.TokenAddress, key.AccountAddress, change.String())
			}
		}
		// Even for a (suspected) non-change, notify the evm_token_balances analyzer to
		// verify the correct balance by querying the EVM.
		if m.mode == analyzer.SlowSyncMode {
			// In slow-sync mode, directly update `last_mutate_round`.
			batch.Queue(
				queries.RuntimeEVMTokenBalanceAnalysisMutateRoundUpsert,
				m.runtime, key.TokenAddress, key.AccountAddress, data.Header.Round,
			)
		} else {
			// In fast-sync mode, just record the intent to update the value.
			batch.Queue(
				queries.RuntimeFastSyncEVMTokenBalanceAnalysisMutateRoundInsert,
				m.runtime, key.TokenAddress, key.AccountAddress, data.Header.Round,
			)
		}
	}

	// Insert NFTs.
	for key, possibleNFT := range data.PossibleNFTs {
		batch.Queue(
			queries.RuntimeEVMNFTUpsert,
			m.runtime,
			key.TokenAddress,
			key.TokenID,
			data.Header.Round,
		)
		if possibleNFT.NumTransfers > 0 {
			var newOwner *apiTypes.Address
			if !possibleNFT.Burned {
				newOwner = &possibleNFT.NewOwner
			}
			batch.Queue(
				queries.RuntimeEVMNFTUpdateTransfer,
				m.runtime,
				key.TokenAddress,
				key.TokenID,
				possibleNFT.NumTransfers,
				newOwner,
			)
		}
	}

	// Insert swap pairs.
	for creationKey, creation := range data.SwapCreations {
		batch.Queue(
			queries.RuntimeEVMSwapPairUpsertCreated,
			m.runtime,
			creationKey.Factory,
			creationKey.Token0,
			creationKey.Token1,
			creation.Pair,
			data.Header.Round,
		)
	}
	for pairAddress, sync := range data.SwapSyncs {
		batch.Queue(
			queries.RuntimeEVMSwapPairUpsertSync,
			m.runtime,
			pairAddress,
			sync.Reserve0,
			sync.Reserve1,
			data.Header.Round,
		)
	}
}
