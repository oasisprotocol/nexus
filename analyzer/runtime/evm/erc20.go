package evm

import (
	"context"
	"encoding/hex"
	"fmt"

	ethCommon "github.com/ethereum/go-ethereum/common"
	"github.com/oasisprotocol/oasis-core/go/common/errors"

	"github.com/oasisprotocol/nexus/analyzer/evmabi"
	"github.com/oasisprotocol/nexus/log"
	"github.com/oasisprotocol/nexus/storage/oasis/nodeapi"
)

const EVMTokenTypeERC20 EVMTokenType = 20

func evmDownloadTokenERC20Mutable(ctx context.Context, logger *log.Logger, source nodeapi.RuntimeApiLite, round uint64, tokenEthAddr []byte) (*EVMTokenMutableData, error) {
	var mutable EVMTokenMutableData
	// These mandatory methods must succeed, or we do not count this as an ERC-20 token.
	if err := evmCallWithABI(ctx, source, round, tokenEthAddr, evmabi.ERC20, &mutable.TotalSupply, "totalSupply"); err != nil {
		if !errors.Is(err, EVMDeterministicError{}) {
			return nil, fmt.Errorf("calling totalSupply: %w", err)
		}
		logDeterministicError(logger, round, tokenEthAddr, "ERC20", "totalSupply", err)
		return nil, nil
	}
	return &mutable, nil
}

func evmDownloadTokenERC20(ctx context.Context, logger *log.Logger, source nodeapi.RuntimeApiLite, round uint64, tokenEthAddr []byte) (*EVMTokenData, error) {
	tokenData := EVMTokenData{
		Type: EVMTokenTypeERC20,
	}
	// These optional methods may fail.
	if err := evmCallWithABI(ctx, source, round, tokenEthAddr, evmabi.ERC20, &tokenData.Name, "name"); err != nil {
		// Propagate the error (so that token-info fetching may be retried
		// later) only if the error is non-deterministic, e.g. network
		// failure. Otherwise, accept that the token contract doesn't expose
		// this info.
		if !errors.Is(err, EVMDeterministicError{}) {
			return nil, fmt.Errorf("calling name: %w", err)
		}
		logDeterministicError(logger, round, tokenEthAddr, "ERC20", "name", err)
	}
	if err := evmCallWithABI(ctx, source, round, tokenEthAddr, evmabi.ERC20, &tokenData.Symbol, "symbol"); err != nil {
		if !errors.Is(err, EVMDeterministicError{}) {
			return nil, fmt.Errorf("calling symbol: %w", err)
		}
		logDeterministicError(logger, round, tokenEthAddr, "ERC20", "symbol", err)
	}
	if err := evmCallWithABI(ctx, source, round, tokenEthAddr, evmabi.ERC20, &tokenData.Decimals, "decimals"); err != nil {
		if !errors.Is(err, EVMDeterministicError{}) {
			return nil, fmt.Errorf("calling decimals: %w", err)
		}
		logDeterministicError(logger, round, tokenEthAddr, "ERC20", "decimals", err)
	}
	mutable, err := evmDownloadTokenERC20Mutable(ctx, logger, source, round, tokenEthAddr)
	if err != nil {
		return nil, err
	}
	if mutable == nil {
		return nil, nil
	}
	tokenData.EVMTokenMutableData = mutable
	return &tokenData, nil
}

func evmDownloadTokenBalanceERC20(ctx context.Context, logger *log.Logger, source nodeapi.RuntimeApiLite, round uint64, tokenEthAddr []byte, accountEthAddr []byte) (*EVMTokenBalanceData, error) {
	var balanceData EVMTokenBalanceData
	accountECAddr := ethCommon.BytesToAddress(accountEthAddr)
	if err := evmCallWithABI(ctx, source, round, tokenEthAddr, evmabi.ERC20, &balanceData.Balance, "balanceOf", accountECAddr); err != nil {
		if !errors.Is(err, EVMDeterministicError{}) {
			return nil, fmt.Errorf("calling balanceOf: %w", err)
		}
		logDeterministicError(logger, round, tokenEthAddr, "ERC20", "balanceOf", err,
			"account_eth_addr_hex", hex.EncodeToString(accountEthAddr),
		)
		return nil, nil
	}
	return &balanceData, nil
}
