package runtime

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	sdkConfig "github.com/oasisprotocol/oasis-sdk/client-sdk/go/config"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/modules/rewards"
	sdkTypes "github.com/oasisprotocol/oasis-sdk/client-sdk/go/types"

	"github.com/oasisprotocol/oasis-indexer/analyzer"
	"github.com/oasisprotocol/oasis-indexer/analyzer/queries"
	uncategorized "github.com/oasisprotocol/oasis-indexer/analyzer/uncategorized"
	"github.com/oasisprotocol/oasis-indexer/analyzer/util"
	"github.com/oasisprotocol/oasis-indexer/common"
	"github.com/oasisprotocol/oasis-indexer/config"
	"github.com/oasisprotocol/oasis-indexer/log"
	"github.com/oasisprotocol/oasis-indexer/metrics"
	"github.com/oasisprotocol/oasis-indexer/storage"
	source "github.com/oasisprotocol/oasis-indexer/storage/oasis"
)

const (
	ProcessRoundTimeout = 61 * time.Second
)

// runtimeAnalyzer is the main Analyzer for runtimes.
type runtimeAnalyzer struct {
	cfg     analyzer.RuntimeConfig
	target  storage.TargetStorage
	logger  *log.Logger
	metrics metrics.DatabaseMetrics
}

var _ analyzer.Analyzer = (*runtimeAnalyzer)(nil)

// NewRuntimeAnalyzer returns a new main analyzer for a runtime.
func NewRuntimeAnalyzer(
	runtime common.Runtime,
	runtimeMetadata *sdkConfig.ParaTime,
	cfg config.BlockRange,
	sourceClient *source.RuntimeClient,
	target storage.TargetStorage,
	logger *log.Logger,
) (analyzer.Analyzer, error) {
	roundRange := analyzer.RoundRange{
		From: uint64(cfg.From),
		To:   uint64(cfg.To),
	}
	ac := analyzer.RuntimeConfig{
		RuntimeName: runtime,
		ParaTime:    runtimeMetadata,
		Range:       roundRange,
		Source:      sourceClient,
	}

	return &runtimeAnalyzer{
		cfg:     ac,
		target:  target,
		logger:  logger.With("analyzer", runtime),
		metrics: metrics.NewDefaultDatabaseMetrics(string(runtime)),
	}, nil
}

func (m *runtimeAnalyzer) Start(ctx context.Context) {
	if err := m.prework(ctx); err != nil {
		m.logger.Error("error doing prework",
			"err", err,
		)
		return
	}

	// Get round to be indexed.
	var round uint64

	latest, err := m.latestRound(ctx)
	if err != nil {
		if err != pgx.ErrNoRows {
			m.logger.Error("last round not found",
				"err", err,
			)
			return
		}
		m.logger.Debug("setting round using range config")
		round = m.cfg.Range.From
	} else {
		m.logger.Debug("setting round using latest round")
		round = latest + 1
	}

	backoff, err := util.NewBackoff(
		100*time.Millisecond,
		// Cap the timeout at the expected round time. All runtimes currently have the same round time.
		6*time.Second,
	)
	if err != nil {
		m.logger.Error("error configuring indexer backoff policy",
			"err", err,
		)
		return
	}

	for m.cfg.Range.To == 0 || round <= m.cfg.Range.To {
		select {
		case <-time.After(backoff.Timeout()):
			// Process next block.
		case <-ctx.Done():
			m.logger.Warn("shutting down runtime analyzer", "reason", ctx.Err())
			return
		}
		m.logger.Info("attempting block", "round", round)

		if err := m.processRound(ctx, round); err != nil {
			if err == analyzer.ErrOutOfRange {
				m.logger.Info("no data available; will retry",
					"round", round,
					"retry_interval_ms", backoff.Timeout().Milliseconds(),
				)
			} else {
				m.logger.Error("error processing round",
					"round", round,
					"err", err.Error(),
				)
			}
			backoff.Failure()
			continue
		}

		m.logger.Info("processed block", "round", round)
		backoff.Success()
		round++
	}
	m.logger.Info(
		fmt.Sprintf("finished processing all blocks in the configured range [%d, %d]",
			m.cfg.Range.From, m.cfg.Range.To))
}

// Name returns the name of the Main.
func (m *runtimeAnalyzer) Name() string {
	return string(m.cfg.RuntimeName)
}

func (m *runtimeAnalyzer) nativeTokenSymbol() string {
	return m.cfg.ParaTime.Denominations[sdkConfig.NativeDenominationKey].Symbol
}

// StringifyDenomination returns a string representation of the given denomination.
// This is simply the denomination's symbol; notably, for the native denomination,
// this is looked up from network config.
func (m *runtimeAnalyzer) StringifyDenomination(d sdkTypes.Denomination) string {
	if d.IsNative() {
		return m.nativeTokenSymbol()
	}

	return d.String()
}

// latestRound returns the latest round processed by the consensus analyzer.
func (m *runtimeAnalyzer) latestRound(ctx context.Context) (uint64, error) {
	var latest uint64
	if err := m.target.QueryRow(
		ctx,
		queries.LatestBlock,
		// ^analyzers should only analyze for a single chain ID, and we anchor this
		// at the starting round.
		m.cfg.RuntimeName,
	).Scan(&latest); err != nil {
		return 0, err
	}
	return latest, nil
}

// prework performs tasks that need to be done before the main loop starts.
func (m *runtimeAnalyzer) prework(ctx context.Context) error {
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

// processRound processes the provided round, retrieving all required information
// from source storage and committing an atomically-executed batch of queries
// to target storage.
func (m *runtimeAnalyzer) processRound(ctx context.Context, round uint64) error {
	ctxWithTimeout, cancel := context.WithTimeout(ctx, ProcessRoundTimeout)
	defer cancel()

	// Fetch all data.
	data, err := m.cfg.Source.AllData(ctxWithTimeout, round)
	if err != nil {
		if strings.Contains(err.Error(), "roothash: block not found") {
			return analyzer.ErrOutOfRange
		}
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
		m.cfg.RuntimeName,
	)

	opName := fmt.Sprintf("process_block_%s", m.cfg.RuntimeName)
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
func (m *runtimeAnalyzer) queueDbUpdates(batch *storage.QueryBatch, data *BlockData) {
	// Block metadata.
	batch.Queue(
		queries.RuntimeBlockInsert,
		m.cfg.RuntimeName,
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
				m.cfg.RuntimeName,
				data.Header.Round,
				transactionData.Index,
				signerData.Index,
				signerData.Address,
				signerData.Nonce,
			)
		}
		for addr := range transactionData.RelatedAccountAddresses {
			batch.Queue(queries.RuntimeRelatedTransactionInsert, m.cfg.RuntimeName, addr, data.Header.Round, transactionData.Index)
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
			m.cfg.RuntimeName,
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
			m.cfg.RuntimeName,
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
			batch.Queue(queries.RuntimeEVMTokenAnalysisMutateInsert, m.cfg.RuntimeName, addr, data.Header.Round)
		} else {
			batch.Queue(queries.RuntimeEVMTokenAnalysisInsert, m.cfg.RuntimeName, addr, data.Header.Round)
		}
	}

	// Update EVM token balances (dead reckoning).
	for key, change := range data.TokenBalanceChanges {
		batch.Queue(queries.RuntimeEVMTokenBalanceUpdate, m.cfg.RuntimeName, key.TokenAddress, key.AccountAddress, change.String())
		batch.Queue(queries.RuntimeEVMTokenBalanceAnalysisInsert, m.cfg.RuntimeName, key.TokenAddress, key.AccountAddress, data.Header.Round)
	}
}
