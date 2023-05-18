package evmtokenbalances

import (
	"context"
	"fmt"
	"math/big"
	"time"

	"golang.org/x/sync/errgroup"

	sdkConfig "github.com/oasisprotocol/oasis-sdk/client-sdk/go/config"

	"github.com/oasisprotocol/oasis-indexer/analyzer"
	"github.com/oasisprotocol/oasis-indexer/analyzer/queries"
	"github.com/oasisprotocol/oasis-indexer/analyzer/runtime/evm"
	"github.com/oasisprotocol/oasis-indexer/analyzer/util"
	"github.com/oasisprotocol/oasis-indexer/common"
	"github.com/oasisprotocol/oasis-indexer/log"
	"github.com/oasisprotocol/oasis-indexer/storage"
	"github.com/oasisprotocol/oasis-indexer/storage/client"
	source "github.com/oasisprotocol/oasis-indexer/storage/oasis"
	"github.com/oasisprotocol/oasis-indexer/storage/oasis/nodeapi"
)

// Imagine a timeline starting from a `balanceOf` output `v0` followed by
// a delta `d1` encountered in a `Transfer` log.
// [v0][d1]
//    ^   ^
//    r0  r1
// Consider two points in history:
// - `r0` is the round that the token balances analyzer last downloaded the
//   balance for.
// - `r1` is a later round that the block scanner scanned where the balance
//   changed.
// Additionally, at this time:
// - `last_download_round` is `r0`
// - `last_mutate_round` is `r1`
// - `balance` is `v0` + `d1`
//
// Suppose at this point the token balances analyzer runs. It notices that
// this account is _stale_, because `last_mutate_round` >
// `last_download_round` on this account. It does the following atomically
// (regardless of what the block scanner is doing):
// 1. Set _download round_ = `last_mutate_round` (= `r1`).
// 2. Set _reckoned balance_ = `balance` (= `v0` + `d1`).
// It then tries to download the balance (i.e. call balanceOf) at _download
// round_. This takes place while the rest of the indexer continues to run.
//
// Suppose the block scanner then reaches round `r2` and encounters a
// `Transfer` log.
// [v0][d1][d2]
//    ^   ^   ^
//    r0  r1  r2
// The block scanner does the following atomically (regardless of what the
// token balances analyzer is doing):
// 1. Update `balance` = `balance` + `d2` (= `v0` + `d1` + `d2`).
// 2. Set `last_mutate_round` = `r2`.
//
// Suppose the token balances analyzer then succeeds at downloading the
// balance of the account, getting the balance `v1` (from round `r1`).
// [v0][d1][d2]
//     [v1][d2]
//    ^   ^   ^
//    r0  r1  r2
// The balance `v1` may not equal _reckoned balance_, as a result of contract
// misbehavior or the balance changing beyond the indexer's understanding. It
// does the following atomically:
// 1. Update `balance` = `balance` - _reckoned balance_ + `v1` (= (`v0` +
//    `d1` + `d2`) - (`v0` + `d1`) + `v1` = `v1` + `d2`). This is equivalent
//    to substituting the _reckoned balance_ `v0` + `d1` with the downloaded
//    balance `v1` while keeping subsequent reckoning `d2` in place.
// 2. Set `last_download_round` = _download round_ (= `r1`). Note that
//    `last_download_round` < `last_mutate_round` still, and the token
//    balances analyzer will do another pass of downloading after this.

const (
	//nolint:gosec // thinks this is a hardcoded credential
	EvmTokenBalancesAnalyzerPrefix = "evm_token_balances_"
	MaxDownloadBatch               = 20
	DownloadTimeout                = 61 * time.Second
)

type Main struct {
	runtime         common.Runtime
	runtimeMetadata sdkConfig.ParaTime
	source          storage.RuntimeSourceStorage
	target          storage.TargetStorage
	logger          *log.Logger
}

var _ analyzer.Analyzer = (*Main)(nil)

func NewMain(
	runtime common.Runtime,
	runtimeMetadata sdkConfig.ParaTime,
	sourceClient *source.RuntimeClient,
	target storage.TargetStorage,
	logger *log.Logger,
) (*Main, error) {
	return &Main{
		runtime:         runtime,
		runtimeMetadata: runtimeMetadata,
		source:          sourceClient,
		target:          target,
		logger:          logger.With("analyzer", EvmTokenBalancesAnalyzerPrefix+runtime),
	}, nil
}

type StaleTokenBalance struct {
	TokenAddr                    string
	AccountAddr                  string
	Type                         *evm.EVMTokenType
	Balance                      *big.Int
	TokenAddrContextIdentifier   string
	TokenAddrContextVersion      int
	TokenAddrData                []byte
	AccountAddrContextIdentifier string
	AccountAddrContextVersion    int
	AccountAddrData              []byte
	DownloadRound                uint64
}

func (m Main) getStaleTokenBalances(ctx context.Context, limit int) ([]*StaleTokenBalance, error) {
	var staleTokenBalances []*StaleTokenBalance
	rows, err := m.target.Query(ctx, queries.RuntimeEVMTokenBalanceAnalysisStale,
		m.runtime,
		nativeTokenSymbol(&m.runtimeMetadata),
		limit)
	if err != nil {
		return nil, fmt.Errorf("querying stale token balances: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var staleTokenBalance StaleTokenBalance
		var balanceC common.BigInt
		if err = rows.Scan(
			&staleTokenBalance.TokenAddr,
			&staleTokenBalance.AccountAddr,
			&staleTokenBalance.Type,
			&balanceC,
			&staleTokenBalance.TokenAddrContextIdentifier,
			&staleTokenBalance.TokenAddrContextVersion,
			&staleTokenBalance.TokenAddrData,
			&staleTokenBalance.AccountAddrContextIdentifier,
			&staleTokenBalance.AccountAddrContextVersion,
			&staleTokenBalance.AccountAddrData,
			&staleTokenBalance.DownloadRound,
		); err != nil {
			return nil, fmt.Errorf("scanning stale token balance: %w", err)
		}
		staleTokenBalance.Balance = &balanceC.Int
		staleTokenBalances = append(staleTokenBalances, &staleTokenBalance)
	}
	return staleTokenBalances, nil
}

func (m Main) processStaleTokenBalance(ctx context.Context, batch *storage.QueryBatch, staleTokenBalance *StaleTokenBalance) error {
	accountEthAddr, err := client.EVMEthAddrFromPreimage(staleTokenBalance.AccountAddrContextIdentifier, staleTokenBalance.AccountAddrContextVersion, staleTokenBalance.AccountAddrData)
	if err != nil {
		return fmt.Errorf("account address: %w", err)
	}
	switch *staleTokenBalance.Type {
	case evm.EVMTokenTypeUnsupported:
		// Do nothing; we'll just mark this token as processed so we remove it from the queue.
	case evm.EVMTokenTypeNative:
		// Query native balance.
		addr := nodeapi.Address{}
		if err := addr.UnmarshalText([]byte(staleTokenBalance.AccountAddr)); err != nil {
			m.logger.Error("invalid account address bech32 '%s': %w", staleTokenBalance.AccountAddr, err)
			break
		}
		balance, err := m.source.GetNativeBalance(ctx, staleTokenBalance.DownloadRound, addr)
		if err != nil || balance == nil {
			return fmt.Errorf("getting native runtime balance: %w", err)
		}
		if balance.Cmp(staleTokenBalance.Balance) != 0 {
			correction := (&common.BigInt{}).Sub(&balance.Int, staleTokenBalance.Balance)
			// Native token balance changes do not produce events, so our dead reckoning
			// is expected to often be off. Log discrepancies at Info level only.
			m.logger.Info("correcting reckoned native runtime balance to downloaded balance",
				"account_addr", staleTokenBalance.AccountAddr,
				"download_round", staleTokenBalance.DownloadRound,
				"reckoned_balance", staleTokenBalance.Balance.String(),
				"downloaded_balance", balance.String(),
			)
			batch.Queue(queries.RuntimeNativeBalanceUpdate,
				m.runtime,
				staleTokenBalance.AccountAddr,
				nativeTokenSymbol(&m.runtimeMetadata),
				correction,
			)
		} else {
			m.logger.Debug("native balance: nothing to correct", "account_addr", staleTokenBalance.AccountAddr, "balance", balance)
		}
	default:
		// All other ERC-X tokens. Query the token contract.
		tokenEthAddr, err := client.EVMEthAddrFromPreimage(staleTokenBalance.TokenAddrContextIdentifier, staleTokenBalance.TokenAddrContextVersion, staleTokenBalance.TokenAddrData)
		if err != nil {
			return fmt.Errorf("token address: %w", err)
		}
		balanceData, err := evm.EVMDownloadTokenBalance(
			ctx,
			m.logger,
			m.source,
			staleTokenBalance.DownloadRound,
			tokenEthAddr,
			accountEthAddr,
			*staleTokenBalance.Type,
		)
		if err != nil {
			return fmt.Errorf("downloading token balance %s %s: %w", staleTokenBalance.TokenAddr, staleTokenBalance.AccountAddr, err)
		}
		if balanceData != nil {
			if balanceData.Balance.Cmp(staleTokenBalance.Balance) != 0 {
				correction := &big.Int{}
				correction.Sub(balanceData.Balance, staleTokenBalance.Balance)
				// Can happen when contracts misbehave, or if we haven't indexed all the runtime blocks from the very first one on.
				m.logger.Warn("correcting reckoned balance of token to downloaded balance",
					"token_addr", staleTokenBalance.TokenAddr,
					"account_addr", staleTokenBalance.AccountAddr,
					"download_round", staleTokenBalance.DownloadRound,
					"reckoned_balance", staleTokenBalance.Balance.String(),
					"downloaded_balance", balanceData,
					"correction", correction,
				)
				batch.Queue(queries.RuntimeEVMTokenBalanceUpdate,
					m.runtime,
					staleTokenBalance.TokenAddr,
					staleTokenBalance.AccountAddr,
					correction.String(),
				)
			}
		} else {
			m.logger.Debug("EVM token balance: nothing to correct", "token_addr", staleTokenBalance.TokenAddr, "account_addr", staleTokenBalance.AccountAddr, "balance", balanceData.Balance)
		}
	}
	batch.Queue(queries.RuntimeEVMTokenBalanceAnalysisUpdate,
		m.runtime,
		staleTokenBalance.TokenAddr,
		staleTokenBalance.AccountAddr,
		staleTokenBalance.DownloadRound,
	)
	return nil
}

func (m Main) processBatch(ctx context.Context) (int, error) {
	staleTokenBalances, err := m.getStaleTokenBalances(ctx, MaxDownloadBatch)
	if err != nil {
		return 0, fmt.Errorf("getting stale token balances: %w", err)
	}
	m.logger.Info("processing", "num_stale_token_balances", len(staleTokenBalances))
	if len(staleTokenBalances) == 0 {
		return 0, nil
	}

	ctxWithTimeout, cancel := context.WithTimeout(ctx, DownloadTimeout)
	defer cancel()
	group, groupCtx := errgroup.WithContext(ctxWithTimeout)

	batches := make([]*storage.QueryBatch, 0, len(staleTokenBalances))

	for _, stb := range staleTokenBalances {
		// Redeclare `stb` for unclobbered use within goroutine.
		staleTokenBalance := stb
		batch := &storage.QueryBatch{}
		batches = append(batches, batch)
		group.Go(func() error {
			return m.processStaleTokenBalance(groupCtx, batch, staleTokenBalance)
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
	return len(staleTokenBalances), nil
}

func (m Main) Start(ctx context.Context) {
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
			// Process another batch of token balances.
		case <-ctx.Done():
			m.logger.Warn("shutting down evm_token_balances analyzer", "reason", ctx.Err())
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

func (m Main) Name() string {
	return EvmTokenBalancesAnalyzerPrefix + string(m.runtime)
}

func nativeTokenSymbol(sdkPT *sdkConfig.ParaTime) string {
	return sdkPT.Denominations[sdkConfig.NativeDenominationKey].Symbol
}
