package modules

import (
	"context"
	"encoding/hex"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/oasisprotocol/oasis-core/go/common/errors"
	apiTypes "github.com/oasisprotocol/oasis-indexer/api/v1/types"

	"github.com/oasisprotocol/oasis-indexer/analyzer/evmabi"
	"github.com/oasisprotocol/oasis-indexer/log"
	"github.com/oasisprotocol/oasis-indexer/storage"
)

type EVMTokenType int

const (
	EVMTokenTypeERC20 EVMTokenType = 20
)

type EVMPossibleToken struct {
	Mutated bool
}

type EVMTokenData struct {
	Type     EVMTokenType
	Name     string
	Symbol   string
	Decimals uint8
	*EVMTokenMutableData
}

type EVMTokenMutableData struct {
	TotalSupply *big.Int
}

type EVMBlockTokenData struct {
	TokenData map[apiTypes.Address]*EVMTokenData
}

type EVMDeterministicError struct {
	// Note: .error is the implementation of .Error, .Unwrap etc. It is not
	// in the Unwrap chain. Use something like
	// `EVMDeterministicError{fmt.Errorf("...: %w", err)}` to set up an
	// instance with `err` in the Unwrap chain.
	error
}

func (err EVMDeterministicError) Is(target error) bool {
	if _, ok := target.(EVMDeterministicError); ok {
		return true
	}
	return false
}

// TODO: can we move this to oasis-sdk/client-sdk/go/modules/evm?
const EVMModuleName = "evm"

var (
	// https://github.com/oasisprotocol/oasis-sdk/blob/runtime-sdk/v0.2.0/runtime-sdk/modules/evm/src/lib.rs#L123
	ErrEVMExecutionFailed = errors.New(EVMModuleName, 2, "execution failed")
	// https://github.com/oasisprotocol/oasis-sdk/blob/runtime-sdk/v0.2.0/runtime-sdk/modules/evm/src/lib.rs#L147
	ErrEVMReverted = errors.New(EVMModuleName, 8, "reverted")
)

func evmCallWithABICustom(
	ctx context.Context,
	source storage.RuntimeSourceStorage,
	round uint64,
	gasPrice []byte,
	gasLimit uint64,
	caller []byte,
	contractEthAddr []byte,
	value []byte,
	contractABI *abi.ABI,
	result interface{},
	method string,
	params ...interface{},
) error {
	inPacked, err := contractABI.Pack(method, params...)
	if err != nil {
		return fmt.Errorf("packing evm simulate call data: %w", err)
	}
	outPacked, err := source.EVMSimulateCall(ctx, round, gasPrice, gasLimit, caller, contractEthAddr, value, inPacked)
	if err != nil {
		err = fmt.Errorf("runtime client evm simulate call: %w", err)
		if errors.Is(err, ErrEVMExecutionFailed) || errors.Is(err, ErrEVMReverted) {
			err = EVMDeterministicError{err}
		}
		return err
	}
	if err = contractABI.UnpackIntoInterface(result, method, outPacked); err != nil {
		err = fmt.Errorf("unpacking evm simulate call output: %w", err)
		err = EVMDeterministicError{err}
		return err
	}
	return nil
}

// evmCallWithABI: Given a runtime `source` and `round`, and given an EVM
// smart contract (at `contractEthAddr`, with `contractABI`) deployed in that
// runtime, invokes `method(params...)` in that smart contract. The method
// output is unpacked into `result`, so its type must match the output type of
// `method`.
func evmCallWithABI(
	ctx context.Context,
	source storage.RuntimeSourceStorage,
	round uint64,
	contractEthAddr []byte,
	contractABI *abi.ABI,
	result interface{},
	method string,
	params ...interface{}, //nolint:unparam
) error {
	// https://github.com/oasisprotocol/oasis-web3-gateway/blob/v3.0.0/rpc/eth/api.go#L403-L408
	gasPrice := []byte{1}
	gasLimit := uint64(30_000_000)
	caller := common.Address{1}.Bytes()
	value := []byte{0}

	return evmCallWithABICustom(ctx, source, round, gasPrice, gasLimit, caller, contractEthAddr, value, contractABI, result, method, params...)
}

func evmDownloadTokenERC20Mutable(ctx context.Context, logger *log.Logger, source storage.RuntimeSourceStorage, round uint64, tokenEthAddr []byte) (*EVMTokenMutableData, error) {
	var mutable EVMTokenMutableData
	logError := func(method string, err error) {
		logger.Info("ERC20 call failed",
			"round", round,
			"token_eth_addr_hex", hex.EncodeToString(tokenEthAddr),
			"method", method,
			"err", err,
		)
	}
	// These mandatory methods must succeed, or we do not count this as an ERC-20 token.
	if err := evmCallWithABI(ctx, source, round, tokenEthAddr, evmabi.ERC20, &mutable.TotalSupply, "totalSupply"); err != nil {
		logError("totalSupply", err)
		if !errors.Is(err, EVMDeterministicError{}) {
			return nil, fmt.Errorf("calling totalSupply: %w", err)
		}
		return nil, nil
	}
	return &mutable, nil
}

func evmDownloadTokenERC20(ctx context.Context, logger *log.Logger, source storage.RuntimeSourceStorage, round uint64, tokenEthAddr []byte) (*EVMTokenData, error) {
	var tokenData EVMTokenData
	logError := func(method string, err error) {
		logger.Info("ERC20 call failed",
			"round", round,
			"token_eth_addr_hex", hex.EncodeToString(tokenEthAddr),
			"method", method,
			"err", err,
		)
	}
	// These optional methods may fail.
	if err := evmCallWithABI(ctx, source, round, tokenEthAddr, evmabi.ERC20, &tokenData.Name, "name"); err != nil {
		logError("name", err)
		// Propagate the error (so that token-info fetching may be retried
		// later) only if the error is non-deterministic, e.g. network
		// failure. Otherwise, accept that the token contract doesn't expose
		// this info.
		if !errors.Is(err, EVMDeterministicError{}) {
			return nil, fmt.Errorf("calling name: %w", err)
		}
	}
	if err := evmCallWithABI(ctx, source, round, tokenEthAddr, evmabi.ERC20, &tokenData.Symbol, "symbol"); err != nil {
		logError("symbol", err)
		if !errors.Is(err, EVMDeterministicError{}) {
			return nil, fmt.Errorf("calling symbol: %w", err)
		}
	}
	if err := evmCallWithABI(ctx, source, round, tokenEthAddr, evmabi.ERC20, &tokenData.Decimals, "decimals"); err != nil {
		logError("decimals", err)
		if !errors.Is(err, EVMDeterministicError{}) {
			return nil, fmt.Errorf("calling decimals: %w", err)
		}
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

func EVMDownloadNewToken(ctx context.Context, logger *log.Logger, source storage.RuntimeSourceStorage, round uint64, tokenEthAddr []byte) (*EVMTokenData, error) {
	// todo: check ERC-165 0xffffffff compliance
	// todo: try other token standards based on ERC-165
	// see https://github.com/oasisprotocol/oasis-indexer/issues/225

	// Check ERC-20.
	tokenData, err := evmDownloadTokenERC20(ctx, logger, source, round, tokenEthAddr)
	if err != nil {
		return nil, fmt.Errorf("download token ERC-20: %w", err)
	}
	if tokenData != nil {
		return tokenData, nil
	}

	// No applicable token discovered.
	return nil, nil
}

func EVMDownloadMutatedToken(ctx context.Context, logger *log.Logger, source storage.RuntimeSourceStorage, round uint64, tokenEthAddr []byte, tokenType EVMTokenType) (*EVMTokenMutableData, error) {
	switch tokenType {
	case EVMTokenTypeERC20:
		// Check ERC-20.
		mutable, err := evmDownloadTokenERC20Mutable(ctx, logger, source, round, tokenEthAddr)
		if err != nil {
			return nil, fmt.Errorf("download token ERC-20 mutable: %w", err)
		}
		return mutable, nil

	// todo: add suppport for other token types
	// see https://github.com/oasisprotocol/oasis-indexer/issues/225

	default:
		return nil, fmt.Errorf("download mutated token type %v not handled", tokenType)
	}
}
