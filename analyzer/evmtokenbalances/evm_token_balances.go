package evmtokenbalances

import (
	"context"
	"fmt"
	"math/big"
	"time"

	oasisConfig "github.com/oasisprotocol/oasis-sdk/client-sdk/go/config"
	"golang.org/x/sync/errgroup"

	"github.com/oasisprotocol/oasis-indexer/analyzer"
	"github.com/oasisprotocol/oasis-indexer/analyzer/modules"
	"github.com/oasisprotocol/oasis-indexer/analyzer/queries"
	"github.com/oasisprotocol/oasis-indexer/analyzer/util"
	"github.com/oasisprotocol/oasis-indexer/common"
	"github.com/oasisprotocol/oasis-indexer/config"
	"github.com/oasisprotocol/oasis-indexer/log"
	"github.com/oasisprotocol/oasis-indexer/storage"
	"github.com/oasisprotocol/oasis-indexer/storage/oasis"
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
	runtime analyzer.Runtime
	cfg     analyzer.RuntimeConfig
	target  storage.TargetStorage
	logger  *log.Logger
}

var _ analyzer.Analyzer = (*Main)(nil)

func NewMain(
	runtime analyzer.Runtime,
	nodeCfg *config.NodeConfig,
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
	ac := analyzer.RuntimeConfig{
		Source: client,
	}

	return &Main{
		runtime: runtime,
		cfg:     ac,
		target:  target,
		logger:  logger.With("analyzer", EvmTokenBalancesAnalyzerPrefix+runtime.String()),
	}, nil
}

type StaleTokenBalance struct {
	TokenAddr                    string
	AccountAddr                  string
	LastMutateRound              uint64
	Type                         *modules.EVMTokenType
	Balance                      *big.Int
	TokenAddrContextIdentifier   string
	TokenAddrContextVersion      int
	TokenAddrData                []byte
	AccountAddrContextIdentifier string
	AccountAddrContextVersion    int
	AccountAddrData              []byte
}

func (m Main) getStaleTokenBalances(ctx context.Context, limit int) ([]*StaleTokenBalance, error) {
	var staleTokenBalances []*StaleTokenBalance
	rows, err := m.target.Query(ctx, queries.RuntimeEVMTokenBalanceAnalysisStale, m.runtime, limit)
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
			&staleTokenBalance.LastMutateRound,
			&staleTokenBalance.Type,
			&balanceC,
			&staleTokenBalance.TokenAddrContextIdentifier,
			&staleTokenBalance.TokenAddrContextVersion,
			&staleTokenBalance.TokenAddrData,
			&staleTokenBalance.AccountAddrContextIdentifier,
			&staleTokenBalance.AccountAddrContextVersion,
			&staleTokenBalance.AccountAddrData,
		); err != nil {
			return nil, fmt.Errorf("scanning stale token balance: %w", err)
		}
		staleTokenBalance.Balance = &balanceC.Int
		staleTokenBalances = append(staleTokenBalances, &staleTokenBalance)
	}
	return staleTokenBalances, nil
}

func (m Main) processStaleTokenBalance(ctx context.Context, batch *storage.QueryBatch, staleTokenBalance *StaleTokenBalance) error {
	m.logger.Info("downloading", "stale_token_balance", staleTokenBalance)
	// todo: assert that token addr and account addr contexts are secp256k1
	tokenEthAddr, err := modules.EVMEthAddrFromPreimage(staleTokenBalance.TokenAddrContextIdentifier, staleTokenBalance.TokenAddrContextVersion, staleTokenBalance.TokenAddrData)
	if err != nil {
		return fmt.Errorf("token address: %w", err)
	}
	accountEthAddr, err := modules.EVMEthAddrFromPreimage(staleTokenBalance.AccountAddrContextIdentifier, staleTokenBalance.AccountAddrContextVersion, staleTokenBalance.AccountAddrData)
	if err != nil {
		return fmt.Errorf("account address: %w", err)
	}
	if staleTokenBalance.Type != nil {
		balanceData, err := modules.EVMDownloadTokenBalance(
			ctx,
			m.logger,
			m.cfg.Source,
			staleTokenBalance.LastMutateRound,
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
				m.logger.Info("correcting reckoned balance of token to downloaded balance",
					"token_addr", staleTokenBalance.TokenAddr,
					"account_addr", staleTokenBalance.AccountAddr,
					"download_round", staleTokenBalance.LastMutateRound,
					"reckoned_balance", staleTokenBalance.Balance,
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
		}
	}
	batch.Queue(queries.RuntimeEVMTokenBalanceAnalysisUpdate,
		m.runtime,
		staleTokenBalance.TokenAddr,
		staleTokenBalance.AccountAddr,
		staleTokenBalance.LastMutateRound,
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
	return EvmTokenBalancesAnalyzerPrefix + m.runtime.String()
}