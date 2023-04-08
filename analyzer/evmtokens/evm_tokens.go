package evmtokens

import (
	"context"
	"fmt"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/oasisprotocol/oasis-indexer/analyzer"
	"github.com/oasisprotocol/oasis-indexer/analyzer/queries"
	"github.com/oasisprotocol/oasis-indexer/analyzer/runtime"
	"github.com/oasisprotocol/oasis-indexer/analyzer/util"
	"github.com/oasisprotocol/oasis-indexer/config"
	"github.com/oasisprotocol/oasis-indexer/log"
	"github.com/oasisprotocol/oasis-indexer/storage"
	"github.com/oasisprotocol/oasis-indexer/storage/client"
	"github.com/oasisprotocol/oasis-indexer/storage/oasis"
)

// The token analyzer (1) gets a list from the database of tokens to download
// the info for, (2) downloads that info, and (3) saves the info in the
// database. Note that steps (1) and (3) don't happen in a single transaction,
// so that we don't hold the tables locked and disrupt the block scanner. So
// be careful of block analyzer potentially making further updates in between.
// That's why, for example, there are separate last_mutate_round and
// last_download_round columns. The block analyzer updates last_mutate_round,
// this token analyzer updates last_download_round.

const (
	//nolint:gosec // thinks this is a hardcoded credential
	EvmTokensAnalyzerPrefix = "evm_tokens_"
	MaxDownloadBatch        = 20
	DownloadTimeout         = 61 * time.Second
)

type Main struct {
	runtime analyzer.Runtime
	cfg     analyzer.RuntimeConfig
	target  storage.TargetStorage
	logger  *log.Logger
}

var _ analyzer.Analyzer = (*Main)(nil)

func NewMain(
	runtime analyzer.Runtime,
	sourceConfig *config.SourceConfig,
	target storage.TargetStorage,
	logger *log.Logger,
) (*Main, error) {
	ctx := context.Background()

	// Initialize source storage.
	client, err := oasis.NewRuntimeClient(ctx, sourceConfig, runtime)
	if err != nil {
		logger.Error("error creating runtime client",
			"err", err,
		)
		return nil, err
	}
	ac := analyzer.RuntimeConfig{
		Source: client,
	}

	return &Main{
		runtime: runtime,
		cfg:     ac,
		target:  target,
		logger:  logger.With("analyzer", EvmTokensAnalyzerPrefix+runtime.String()),
	}, nil
}

type StaleToken struct {
	Addr                  string
	LastMutateRound       uint64
	LastDownloadRound     *uint64
	Type                  *runtime.EVMTokenType
	AddrContextIdentifier string
	AddrContextVersion    int
	AddrData              []byte
}

func (m Main) getStaleTokens(ctx context.Context, limit int) ([]*StaleToken, error) {
	var staleTokens []*StaleToken
	rows, err := m.target.Query(ctx, queries.RuntimeEVMTokenAnalysisStale, m.runtime, limit)
	if err != nil {
		return nil, fmt.Errorf("querying discovered tokens: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var staleToken StaleToken
		if err = rows.Scan(
			&staleToken.Addr,
			&staleToken.LastMutateRound,
			&staleToken.LastDownloadRound,
			&staleToken.Type,
			&staleToken.AddrContextIdentifier,
			&staleToken.AddrContextVersion,
			&staleToken.AddrData,
		); err != nil {
			return nil, fmt.Errorf("scanning discovered token: %w", err)
		}
		staleTokens = append(staleTokens, &staleToken)
	}
	return staleTokens, nil
}

func (m Main) processStaleToken(ctx context.Context, batch *storage.QueryBatch, staleToken *StaleToken) error {
	m.logger.Info("downloading", "stale_token", staleToken)
	tokenEthAddr, err := client.EVMEthAddrFromPreimage(staleToken.AddrContextIdentifier, staleToken.AddrContextVersion, staleToken.AddrData)
	if err != nil {
		return fmt.Errorf("token address: %w", err)
	}
	//nolint:nestif
	if staleToken.LastDownloadRound == nil {
		tokenData, err := runtime.EVMDownloadNewToken(
			ctx,
			m.logger,
			m.cfg.Source,
			staleToken.LastMutateRound,
			tokenEthAddr,
		)
		if err != nil {
			return fmt.Errorf("downloading new token %s: %w", staleToken.Addr, err)
		}
		if tokenData != nil {
			batch.Queue(queries.RuntimeEVMTokenInsert,
				m.runtime,
				staleToken.Addr,
				tokenData.Type,
				tokenData.Name,
				tokenData.Symbol,
				tokenData.Decimals,
				tokenData.TotalSupply.String(),
			)
		}
	} else if staleToken.Type != nil {
		mutable, err := runtime.EVMDownloadMutatedToken(
			ctx,
			m.logger,
			m.cfg.Source,
			staleToken.LastMutateRound,
			tokenEthAddr,
			*staleToken.Type,
		)
		if err != nil {
			return fmt.Errorf("downloading mutated token %s: %w", staleToken.Addr, err)
		}
		if mutable != nil {
			batch.Queue(queries.RuntimeEVMTokenUpdate,
				m.runtime,
				staleToken.Addr,
				mutable.TotalSupply.String(),
			)
		}
	}
	batch.Queue(queries.RuntimeEVMTokenAnalysisUpdate, m.runtime, staleToken.Addr, staleToken.LastMutateRound)
	return nil
}

func (m Main) processBatch(ctx context.Context) (int, error) {
	staleTokens, err := m.getStaleTokens(ctx, MaxDownloadBatch)
	if err != nil {
		return 0, fmt.Errorf("getting discovered tokens: %w", err)
	}
	m.logger.Info("processing", "num_stale_tokens", len(staleTokens))
	if len(staleTokens) == 0 {
		return 0, nil
	}

	ctxWithTimeout, cancel := context.WithTimeout(ctx, DownloadTimeout)
	defer cancel()
	group, groupCtx := errgroup.WithContext(ctxWithTimeout)

	batches := make([]*storage.QueryBatch, 0, len(staleTokens))

	for _, st := range staleTokens {
		// Redeclare `st` for unclobbered use within goroutine.
		staleToken := st
		batch := &storage.QueryBatch{}
		batches = append(batches, batch)
		group.Go(func() error {
			return m.processStaleToken(groupCtx, batch, staleToken)
		})
	}

	if err := group.Wait(); err != nil {
		return 0, err
	}

	batch := &storage.QueryBatch{}
	for _, b := range batches {
		batch.Extend(b)
	}
	if err := m.target.SendBatch(ctx, batch); err != nil {
		return 0, fmt.Errorf("sending batch: %w", err)
	}
	return len(staleTokens), nil
}

func (m Main) Start() {
	ctx := context.Background()

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

	for {
		backoff.Wait()

		numProcessed, err := m.processBatch(ctx)
		if err != nil {
			m.logger.Error("error processing batch", "err", err)
			backoff.Failure()
			continue
		}

		if numProcessed == 0 {
			// Count this as a failure to reduce the polling when we are
			// running faster than the block analyzer can find new tokens.
			backoff.Failure()
			continue
		}

		backoff.Success()
	}
}

func (m Main) Name() string {
	return EvmTokensAnalyzerPrefix + m.runtime.String()
}
