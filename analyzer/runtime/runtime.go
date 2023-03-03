package runtime

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v4"
	oasisConfig "github.com/oasisprotocol/oasis-sdk/client-sdk/go/config"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/modules/rewards"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/types"
	"golang.org/x/sync/errgroup"

	"github.com/oasisprotocol/oasis-indexer/analyzer"
	"github.com/oasisprotocol/oasis-indexer/analyzer/modules"
	"github.com/oasisprotocol/oasis-indexer/analyzer/queries"
	common "github.com/oasisprotocol/oasis-indexer/analyzer/uncategorized"
	"github.com/oasisprotocol/oasis-indexer/analyzer/util"
	apiTypes "github.com/oasisprotocol/oasis-indexer/api/v1/types"
	"github.com/oasisprotocol/oasis-indexer/config"
	"github.com/oasisprotocol/oasis-indexer/log"
	"github.com/oasisprotocol/oasis-indexer/metrics"
	"github.com/oasisprotocol/oasis-indexer/storage"
	"github.com/oasisprotocol/oasis-indexer/storage/oasis"
)

const (
	ProcessRoundTimeout = 61 * time.Second
)

// Main is the main Analyzer for runtimes.
type Main struct {
	runtime analyzer.Runtime
	cfg     analyzer.RuntimeConfig
	target  storage.TargetStorage
	logger  *log.Logger
	metrics metrics.DatabaseMetrics

	moduleHandlers []modules.ModuleHandler
}

var _ analyzer.Analyzer = (*Main)(nil)

// NewRuntimeAnalyzer returns a new main analyzer for a runtime.
func NewRuntimeAnalyzer(
	runtime analyzer.Runtime,
	nodeCfg config.NodeConfig,
	cfg *config.BlockBasedAnalyzerConfig,
	target storage.TargetStorage,
	logger *log.Logger,
) (*Main, error) {
	ctx := context.Background()

	// Initialize source storage.
	networkCfg := oasisConfig.Network{
		ChainContext: nodeCfg.ChainContext,
		RPC:          nodeCfg.RPC,
	}
	factory, err := oasis.NewClientFactory(ctx, &networkCfg, nodeCfg.FastStartup)
	if err != nil {
		logger.Error("error creating client factory",
			"err", err,
		)
		return nil, err
	}

	network, err := analyzer.FromChainContext(nodeCfg.ChainContext)
	if err != nil {
		return nil, err
	}

	id, err := runtime.ID(network)
	if err != nil {
		return nil, err
	}
	logger.Info("Runtime ID determined", "runtime", runtime.String(), "runtime_id", id)

	client, err := factory.Runtime(id)
	if err != nil {
		logger.Error("error creating runtime client",
			"err", err,
		)
		return nil, err
	}
	roundRange := analyzer.RoundRange{
		From: uint64(cfg.From),
		To:   uint64(cfg.To),
	}
	ac := analyzer.RuntimeConfig{
		Range:  roundRange,
		Source: client,
	}

	return &Main{
		runtime: runtime,
		cfg:     ac,
		target:  target,
		logger:  logger.With("analyzer", runtime.String()),
		metrics: metrics.NewDefaultDatabaseMetrics(runtime.String()),

		// module handlers
		moduleHandlers: []modules.ModuleHandler{
			modules.NewCoreHandler(client, runtime.String(), logger),
			modules.NewAccountsHandler(client, runtime.String(), logger),
			modules.NewConsensusAccountsHandler(client, runtime.String(), logger),
		},
	}, nil
}

func (m *Main) Start() {
	ctx := context.Background()

	if err := m.prework(); err != nil {
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
		backoff.Wait()
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
func (m *Main) Name() string {
	return m.runtime.String()
}

// latestRound returns the latest round processed by the consensus analyzer.
func (m *Main) latestRound(ctx context.Context) (uint64, error) {
	var latest uint64
	if err := m.target.QueryRow(
		ctx,
		queries.LatestBlock,
		// ^analyzers should only analyze for a single chain ID, and we anchor this
		// at the starting round.
		m.runtime.String(),
	).Scan(&latest); err != nil {
		return 0, err
	}
	return latest, nil
}

// prework performs tasks that need to be done before the main loop starts.
func (m *Main) prework() error {
	batch := &storage.QueryBatch{}
	ctx := context.Background()

	// Register special addresses.
	batch.Queue(
		queries.AddressPreimageInsert,
		rewards.RewardPoolAddress.String(),          // oasis1qp7x0q9qahahhjas0xde8w0v04ctp4pqzu5mhjav on mainnet oasis-3
		types.AddressV0ModuleContext.Identifier,     // context_identifier
		int32(types.AddressV0ModuleContext.Version), // context_version
		"rewards.reward-pool",                       // address_data (reconstructed from NewAddressForModule())
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
func (m *Main) processRound(ctx context.Context, round uint64) error {
	ctxWithTimeout, cancel := context.WithTimeout(ctx, ProcessRoundTimeout)
	defer cancel()
	group, groupCtx := errgroup.WithContext(ctxWithTimeout)

	// Prepare and perform updates.
	batch := &storage.QueryBatch{}

	type prepareFunc = func(context.Context, uint64, *storage.QueryBatch) error
	for _, f := range []prepareFunc{
		m.prepareBlockData,
		m.prepareEventData,
	} {
		func(f prepareFunc) {
			group.Go(func() error {
				return f(groupCtx, round, batch)
			})
		}(f)
	}

	for _, h := range m.moduleHandlers {
		func(f prepareFunc) {
			group.Go(func() error {
				return f(groupCtx, round, batch)
			})
		}(h.PrepareData)
	}

	// Update indexing progress.
	group.Go(func() error {
		batch.Queue(
			queries.IndexingProgress,
			round,
			m.runtime.String(),
		)
		return nil
	})

	if err := group.Wait(); err != nil {
		if strings.Contains(err.Error(), "roothash: block not found") {
			return analyzer.ErrOutOfRange
		}
		return err
	}

	opName := fmt.Sprintf("process_block_%s", m.runtime.String())
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
func (m *Main) prepareBlockData(ctx context.Context, round uint64, batch *storage.QueryBatch) error {
	data, err := m.cfg.Source.BlockData(ctx, round)
	if err != nil {
		return err
	}

	for _, f := range []func(context.Context, *storage.QueryBatch, *storage.RuntimeBlockData) error{
		m.queueBlockAndTransactionInserts,
	} {
		if err := f(ctx, batch, data); err != nil {
			return err
		}
	}

	return nil
}

// prepareEventData adds non-tx event data queries to the batch.
func (m *Main) prepareEventData(ctx context.Context, round uint64, batch *storage.QueryBatch) error {
	events, err := m.cfg.Source.GetEventsRaw(ctx, round)
	if err != nil {
		return err
	}
	nonTxEvents := []*types.Event{}
	for _, e := range events.Events {
		if e.TxHash.String() == util.ZeroTxHash {
			nonTxEvents = append(nonTxEvents, e)
		}
	}

	blockData := BlockData{
		AddressPreimages: map[apiTypes.Address]*AddressPreimageData{},
	}
	res, err := extractEvents(&blockData, map[apiTypes.Address]bool{}, nonTxEvents)
	if err != nil {
		return err
	}

	// Insert non-tx event data.
	for _, eventData := range res.ExtractedEvents {
		eventRelatedAddresses := common.ExtractAddresses(eventData.RelatedAddresses)
		batch.Queue(
			queries.RuntimeEventInsert,
			m.runtime,
			round,
			nil, // non-tx event has no tx index
			nil, // non-tx event has no tx hash
			eventData.Type,
			eventData.Body,
			eventData.EvmLogName,
			eventData.EvmLogParams,
			eventRelatedAddresses,
		)
	}

	// Insert any eth address preimages found in non-tx events.
	for addr, preimageData := range blockData.AddressPreimages {
		batch.Queue(queries.AddressPreimageInsert, addr, preimageData.ContextIdentifier, preimageData.ContextVersion, preimageData.Data)
	}

	return nil
}

func (m *Main) queueBlockAndTransactionInserts(ctx context.Context, batch *storage.QueryBatch, data *storage.RuntimeBlockData) error {
	blockData, err := extractRound(data.BlockHeader, data.TransactionsWithResults, m.logger)
	if err != nil {
		return fmt.Errorf("extract round %d: %w", data.Round, err)
	}

	batch.Queue(
		queries.RuntimeBlockInsert,
		m.runtime,
		data.Round,
		data.BlockHeader.Header.Version,
		time.Unix(int64(data.BlockHeader.Header.Timestamp), 0 /* nanos */),
		data.BlockHeader.Header.EncodedHash().Hex(),
		data.BlockHeader.Header.PreviousHash.Hex(),
		data.BlockHeader.Header.IORoot.Hex(),
		data.BlockHeader.Header.StateRoot.Hex(),
		data.BlockHeader.Header.MessagesHash.Hex(),
		data.BlockHeader.Header.InMessagesHash.Hex(),
		blockData.NumTransactions,
		fmt.Sprintf("%d", blockData.GasUsed),
		blockData.Size,
	)

	m.emitRoundBatch(batch, data.Round, blockData)

	return nil
}
