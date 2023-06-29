package evm

import (
	"context"
	"fmt"

	"github.com/oasisprotocol/oasis-core/go/common/errors"

	"github.com/oasisprotocol/nexus/analyzer/evmabi"
	"github.com/oasisprotocol/nexus/log"
	"github.com/oasisprotocol/nexus/storage/oasis/nodeapi"
)

const EVMTokenTypeERC721 EVMTokenType = 721

func evmDownloadTokenERC721Mutable(ctx context.Context, logger *log.Logger, source nodeapi.RuntimeApiLite, round uint64, tokenEthAddr []byte) (*EVMTokenMutableData, error) {
	var mutable EVMTokenMutableData
	supportsEnumerable, err := detectInterface(ctx, logger, source, round, tokenEthAddr, ERC721EnumerableInterfaceID)
	if err != nil {
		return nil, fmt.Errorf("checking ERC721Enumerable interface: %w", err)
	}
	if supportsEnumerable {
		if err1 := evmCallWithABI(ctx, source, round, tokenEthAddr, evmabi.ERC721Enumerable, &mutable.TotalSupply, "totalSupply"); err1 != nil {
			if !errors.Is(err, EVMDeterministicError{}) {
				return nil, fmt.Errorf("calling totalSupply: %w", err1)
			}
			logDeterministicError(logger, round, tokenEthAddr, "ERC721Enumerable", "totalSupply", err1)
		}
	}
	return &mutable, nil
}

func evmDownloadTokenERC721(ctx context.Context, logger *log.Logger, source nodeapi.RuntimeApiLite, round uint64, tokenEthAddr []byte) (*EVMTokenData, error) {
	tokenData := EVMTokenData{
		Type: EVMTokenTypeERC721,
	}
	supportsMetadata, err := detectInterface(ctx, logger, source, round, tokenEthAddr, ERC721MetadataInterfaceID)
	if err != nil {
		return nil, fmt.Errorf("checking ERC721Metadata interface: %w", err)
	}
	if supportsMetadata { //nolint:nestif
		if err1 := evmCallWithABI(ctx, source, round, tokenEthAddr, evmabi.ERC721Metadata, &tokenData.Name, "name"); err1 != nil {
			if !errors.Is(err, EVMDeterministicError{}) {
				return nil, fmt.Errorf("calling name: %w", err)
			}
			logDeterministicError(logger, round, tokenEthAddr, "ERC721Metadata", "name", err1)
		}
		if err1 := evmCallWithABI(ctx, source, round, tokenEthAddr, evmabi.ERC721Metadata, &tokenData.Symbol, "symbol"); err1 != nil {
			if !errors.Is(err, EVMDeterministicError{}) {
				return nil, fmt.Errorf("calling symbol: %w", err)
			}
			logDeterministicError(logger, round, tokenEthAddr, "ERC721Metadata", "symbol", err1)
		}
	}
	mutable, err := evmDownloadTokenERC721Mutable(ctx, logger, source, round, tokenEthAddr)
	if err != nil {
		return nil, err
	}
	tokenData.EVMTokenMutableData = mutable
	return &tokenData, nil
}
