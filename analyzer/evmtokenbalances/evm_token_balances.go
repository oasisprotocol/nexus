package evmtokenbalances

import (
	"context"
	"fmt"
	"math/big"

	sdkConfig "github.com/oasisprotocol/oasis-sdk/client-sdk/go/config"

	"github.com/oasisprotocol/nexus/analyzer"
	"github.com/oasisprotocol/nexus/analyzer/item"
	"github.com/oasisprotocol/nexus/analyzer/queries"
	"github.com/oasisprotocol/nexus/analyzer/runtime/evm"
	"github.com/oasisprotocol/nexus/common"
	"github.com/oasisprotocol/nexus/config"
	"github.com/oasisprotocol/nexus/log"
	"github.com/oasisprotocol/nexus/storage"
	"github.com/oasisprotocol/nexus/storage/client"
	"github.com/oasisprotocol/nexus/storage/oasis/nodeapi"
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
// round_. This takes place while the rest of the analyzer continues to run.
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
// misbehavior or the balance changing beyond the analyzer's understanding. It
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
	evmTokenBalancesAnalyzerPrefix = "evm_token_balances_"
)

type processor struct {
	runtime common.Runtime
	sdkPT   *sdkConfig.ParaTime
	source  nodeapi.RuntimeApiLite
	target  storage.TargetStorage
	logger  *log.Logger
}

var _ item.ItemProcessor[*StaleTokenBalance] = (*processor)(nil)

func NewAnalyzer(
	runtime common.Runtime,
	cfg config.ItemBasedAnalyzerConfig,
	sdkPT *sdkConfig.ParaTime,
	sourceClient nodeapi.RuntimeApiLite,
	target storage.TargetStorage,
	logger *log.Logger,
) (analyzer.Analyzer, error) {
	logger = logger.With("analyzer", evmTokenBalancesAnalyzerPrefix+runtime)
	p := &processor{
		runtime: runtime,
		sdkPT:   sdkPT,
		source:  sourceClient,
		target:  target,
		logger:  logger,
	}

	return item.NewAnalyzer[*StaleTokenBalance](
		evmTokenBalancesAnalyzerPrefix+string(runtime),
		cfg,
		p,
		target,
		logger,
	)
}

type StaleTokenBalance struct {
	TokenAddr                  string
	AccountAddr                string
	Type                       common.TokenType
	Balance                    *big.Int
	TokenAddrContextIdentifier string
	TokenAddrContextVersion    int
	TokenAddrData              []byte
	DownloadRound              uint64

	// Not necessary for native tokens.
	AccountAddrContextIdentifier *string
	AccountAddrContextVersion    *int
	AccountAddrData              []byte
}

func (p *processor) GetItems(ctx context.Context, limit uint64) ([]*StaleTokenBalance, error) {
	var staleTokenBalances []*StaleTokenBalance
	rows, err := p.target.Query(ctx, queries.RuntimeEVMTokenBalanceAnalysisStale,
		p.runtime,
		nativeTokenSymbol(p.sdkPT),
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

func (p *processor) ProcessItem(ctx context.Context, batch *storage.QueryBatch, staleTokenBalance *StaleTokenBalance) error {
	switch staleTokenBalance.Type {
	case common.TokenTypeUnsupported:
		// Do nothing; we'll just mark this token as processed so we remove it from the queue.
	case common.TokenTypeNative:
		// Query native balance.
		addr := nodeapi.Address{}
		if err := addr.UnmarshalText([]byte(staleTokenBalance.AccountAddr)); err != nil {
			p.logger.Error("invalid account address bech32 '%s': %w", staleTokenBalance.AccountAddr, err)
			// Do not return; mark this token as processed later on in the func, so we remove it from the DB queue.
			break
		}
		balances, err := p.source.GetBalances(ctx, staleTokenBalance.DownloadRound, addr)
		if err != nil {
			return fmt.Errorf("getting native runtime balance: %w", err)
		}
		balance := common.NativeBalance(balances)
		if balance.Cmp(staleTokenBalance.Balance) != 0 {
			correction := (&common.BigInt{}).Sub(&balance.Int, staleTokenBalance.Balance)
			// Native token balance changes do not produce events, so our dead reckoning
			// is expected to often be off. Log discrepancies at Info level only.
			p.logger.Info("correcting reckoned native runtime balance to downloaded balance",
				"account_addr", staleTokenBalance.AccountAddr,
				"download_round", staleTokenBalance.DownloadRound,
				"reckoned_balance", staleTokenBalance.Balance.String(),
				"downloaded_balance", balance.String(),
			)
			batch.Queue(queries.RuntimeNativeBalanceUpsert,
				p.runtime,
				staleTokenBalance.AccountAddr,
				nativeTokenSymbol(p.sdkPT),
				correction,
			)
		} else {
			p.logger.Debug("native balance: nothing to correct", "account_addr", staleTokenBalance.AccountAddr, "balance", balance)
		}
	default:
		// All other ERC-X tokens. Query the token contract.
		tokenEthAddr, err := client.EVMEthAddrFromPreimage(staleTokenBalance.TokenAddrContextIdentifier, staleTokenBalance.TokenAddrContextVersion, staleTokenBalance.TokenAddrData)
		if err != nil {
			return fmt.Errorf("token address: %w", err)
		}
		if staleTokenBalance.AccountAddrContextIdentifier == nil || staleTokenBalance.AccountAddrContextVersion == nil || staleTokenBalance.AccountAddrData == nil {
			return fmt.Errorf("account address: missing preimage for: '%s' (token address: '%s')", staleTokenBalance.AccountAddr, staleTokenBalance.TokenAddr)
		}
		accountEthAddr, err := client.EVMEthAddrFromPreimage(*staleTokenBalance.AccountAddrContextIdentifier, *staleTokenBalance.AccountAddrContextVersion, staleTokenBalance.AccountAddrData)
		if err != nil {
			return fmt.Errorf("account address: %w", err)
		}
		balanceData, err := evm.EVMDownloadTokenBalance(
			ctx,
			p.logger,
			p.source,
			staleTokenBalance.DownloadRound,
			tokenEthAddr,
			accountEthAddr,
			staleTokenBalance.Type,
		)
		if err != nil {
			return fmt.Errorf("downloading token balance %s %s: %w", staleTokenBalance.TokenAddr, staleTokenBalance.AccountAddr, err)
		}
		if balanceData != nil {
			if balanceData.Balance.Cmp(staleTokenBalance.Balance) != 0 {
				correction := &big.Int{}
				correction.Sub(balanceData.Balance, staleTokenBalance.Balance)
				// Can happen when contracts misbehave, or if we haven't indexed all the runtime blocks from the very first one on.
				p.logger.Warn("correcting reckoned balance of token to downloaded balance",
					"token_addr", staleTokenBalance.TokenAddr,
					"account_addr", staleTokenBalance.AccountAddr,
					"download_round", staleTokenBalance.DownloadRound,
					"reckoned_balance", staleTokenBalance.Balance.String(),
					"downloaded_balance", balanceData,
					"correction", correction,
				)
				batch.Queue(queries.RuntimeEVMTokenBalanceUpdate,
					p.runtime,
					staleTokenBalance.TokenAddr,
					staleTokenBalance.AccountAddr,
					correction.String(),
				)
			}
		} else {
			p.logger.Debug("EVM token balance: nothing to correct", "token_addr", staleTokenBalance.TokenAddr, "account_addr", staleTokenBalance.AccountAddr, "balance", staleTokenBalance.Balance.String())
		}
	}
	batch.Queue(queries.RuntimeEVMTokenBalanceAnalysisUpdate,
		p.runtime,
		staleTokenBalance.TokenAddr,
		staleTokenBalance.AccountAddr,
		staleTokenBalance.DownloadRound,
	)
	return nil
}

func (p *processor) QueueLength(ctx context.Context) (int, error) {
	var queueLength int
	if err := p.target.QueryRow(ctx, queries.RuntimeEVMTokenBalanceAnalysisStaleCount, p.runtime).Scan(&queueLength); err != nil {
		return 0, fmt.Errorf("querying number of stale token balances: %w", err)
	}
	return queueLength, nil
}

func nativeTokenSymbol(sdkPT *sdkConfig.ParaTime) string {
	return sdkPT.Denominations[sdkConfig.NativeDenominationKey].Symbol
}
