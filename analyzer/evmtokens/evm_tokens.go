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
	"github.com/oasisprotocol/oasis-indexer/common"
	"github.com/oasisprotocol/oasis-indexer/log"
	"github.com/oasisprotocol/oasis-indexer/storage"
	"github.com/oasisprotocol/oasis-indexer/storage/client"
	source "github.com/oasisprotocol/oasis-indexer/storage/oasis"
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

type evmTokensAnalyzer struct {
	cfg    analyzer.RuntimeConfig
	target storage.TargetStorage
	logger *log.Logger
}

var _ analyzer.Analyzer = (*evmTokensAnalyzer)(nil)

func NewEvmTokensAnalyzer(
	runtime common.Runtime,
	sourceClient *source.RuntimeClient,
	target storage.TargetStorage,
	logger *log.Logger,
) (analyzer.Analyzer, error) {
	ac := analyzer.RuntimeConfig{
		RuntimeName: runtime,
		Source:      sourceClient,
	}

	return &evmTokensAnalyzer{
		cfg:    ac,
		target: target,
		logger: logger.With("analyzer", EvmTokensAnalyzerPrefix+runtime),
	}, nil
}

type StaleToken struct {
	Addr                  string
	LastDownloadRound     *uint64
	Type                  *runtime.EVMTokenType
	AddrContextIdentifier string
	AddrContextVersion    int
	AddrData              []byte
	DownloadRound         uint64
}

func (m evmTokensAnalyzer) getStaleTokens(ctx context.Context, limit int) ([]*StaleToken, error) {
	var staleTokens []*StaleToken
	rows, err := m.target.Query(ctx, queries.RuntimeEVMTokenAnalysisStale, m.cfg.RuntimeName, limit)
	if err != nil {
		return nil, fmt.Errorf("querying discovered tokens: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var staleToken StaleToken
		if err = rows.Scan(
			&staleToken.Addr,
			&staleToken.LastDownloadRound,
			&staleToken.Type,
			&staleToken.AddrContextIdentifier,
			&staleToken.AddrContextVersion,
			&staleToken.AddrData,
			&staleToken.DownloadRound,
		); err != nil {
			return nil, fmt.Errorf("scanning discovered token: %w", err)
		}
		staleTokens = append(staleTokens, &staleToken)
	}
	return staleTokens, nil
}

func (m evmTokensAnalyzer) processStaleToken(ctx context.Context, batch *storage.QueryBatch, staleToken *StaleToken) error {
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
			staleToken.DownloadRound,
			tokenEthAddr,
		)
		if err != nil {
			return fmt.Errorf("downloading new token %s: %w", staleToken.Addr, err)
		}
		if tokenData != nil {
			batch.Queue(queries.RuntimeEVMTokenInsert,
				m.cfg.RuntimeName,
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
			staleToken.DownloadRound,
			tokenEthAddr,
			*staleToken.Type,
		)
		if err != nil {
			return fmt.Errorf("downloading mutated token %s: %w", staleToken.Addr, err)
		}
		if mutable != nil {
			batch.Queue(queries.RuntimeEVMTokenUpdate,
				m.cfg.RuntimeName,
				staleToken.Addr,
				mutable.TotalSupply.String(),
			)
		}
	}
	batch.Queue(queries.RuntimeEVMTokenAnalysisUpdate, m.cfg.RuntimeName, staleToken.Addr, staleToken.DownloadRound)
	return nil
}

func (m evmTokensAnalyzer) processBatch(ctx context.Context) (int, error) {
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

func (m evmTokensAnalyzer) Start(ctx context.Context) {
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
		select {
		case <-time.After(backoff.Timeout()):
			// Process next block.
		case <-ctx.Done():
			m.logger.Warn("shutting down evm_tokens analyzer", "reason", ctx.Err())
			return
		}

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

func (m evmTokensAnalyzer) Name() string {
	return EvmTokensAnalyzerPrefix + string(m.cfg.RuntimeName)
}
