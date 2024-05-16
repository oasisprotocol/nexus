package evm

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"time"

	ethCommon "github.com/ethereum/go-ethereum/common"
	"github.com/oasisprotocol/oasis-core/go/common/errors"

	"github.com/oasisprotocol/nexus/analyzer/evmabi"
	"github.com/oasisprotocol/nexus/analyzer/evmnfts/ipfsclient"
	"github.com/oasisprotocol/nexus/analyzer/evmnfts/multiproto"
	"github.com/oasisprotocol/nexus/common"
	"github.com/oasisprotocol/nexus/log"
	"github.com/oasisprotocol/nexus/storage/oasis/nodeapi"
)

const MaxMetadataBytes = 10 * 1024 * 1024

// ERC721AssetMetadata is asset metadata
// https://eips.ethereum.org/EIPS/eip-721
type ERC721AssetMetadata struct {
	// Name identifies the asset which this NFT represents
	Name *string `json:"name"`
	// Description describes the asset which this NFT represents
	Description *string `json:"description"`
	// Image is A URI pointing to a resource with mime type image/*
	// representing the asset which this NFT represents. (Additional
	// non-descriptive text from ERC-721 omitted.)
	Image *string `json:"image"`
}

func evmDownloadTokenERC721Mutable(ctx context.Context, logger *log.Logger, source nodeapi.RuntimeApiLite, round uint64, tokenEthAddr []byte) (*EVMTokenMutableData, error) {
	var mutable EVMTokenMutableData
	supportsEnumerable, err := detectInterface(ctx, logger, source, round, tokenEthAddr, ERC721EnumerableInterfaceID)
	if err != nil {
		return nil, fmt.Errorf("checking ERC721Enumerable interface: %w", err)
	}
	if supportsEnumerable {
		if err1 := evmCallWithABI(ctx, source, round, tokenEthAddr, evmabi.ERC721Enumerable, &mutable.TotalSupply, "totalSupply"); err1 != nil {
			if !errors.Is(err1, EVMDeterministicError{}) {
				return nil, fmt.Errorf("calling totalSupply: %w", err1)
			}
			logDeterministicError(logger, round, tokenEthAddr, "ERC721Enumerable", "totalSupply", err1)
		}
	}
	return &mutable, nil
}

func evmDownloadTokenERC721(ctx context.Context, logger *log.Logger, source nodeapi.RuntimeApiLite, round uint64, tokenEthAddr []byte) (*EVMTokenData, error) {
	tokenData := EVMTokenData{
		Type: common.TokenTypeERC721,
	}
	supportsMetadata, err := detectInterface(ctx, logger, source, round, tokenEthAddr, ERC721MetadataInterfaceID)
	if err != nil {
		return nil, fmt.Errorf("checking ERC721Metadata interface: %w", err)
	}
	if supportsMetadata { //nolint:nestif
		if err1 := evmCallWithABI(ctx, source, round, tokenEthAddr, evmabi.ERC721Metadata, &tokenData.Name, "name"); err1 != nil {
			if !errors.Is(err1, EVMDeterministicError{}) {
				return nil, fmt.Errorf("calling name: %w", err1)
			}
			logDeterministicError(logger, round, tokenEthAddr, "ERC721Metadata", "name", err1)
		}
		if err1 := evmCallWithABI(ctx, source, round, tokenEthAddr, evmabi.ERC721Metadata, &tokenData.Symbol, "symbol"); err1 != nil {
			if !errors.Is(err1, EVMDeterministicError{}) {
				return nil, fmt.Errorf("calling symbol: %w", err1)
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

func evmDownloadNFTERC721Metadata(ctx context.Context, logger *log.Logger, source nodeapi.RuntimeApiLite, ipfsClient ipfsclient.Client, nftData *EVMNFTData, round uint64, tokenEthAddr []byte, id *big.Int) error {
	var metadataURI string
	if err := evmCallWithABI(ctx, source, round, tokenEthAddr, evmabi.ERC721Metadata, &metadataURI, "tokenURI", id); err != nil {
		if !errors.Is(err, EVMDeterministicError{}) {
			return fmt.Errorf("calling tokenURI: %w", err)
		}
		logDeterministicError(logger, round, tokenEthAddr, "ERC721Metadata", "tokenURI", err,
			"nft_id", id,
		)
		return nil
	}
	nftData.MetadataURI = &metadataURI
	logger.Info("downloading metadata",
		"token_eth_addr", hex.EncodeToString(tokenEthAddr),
		"token_id", id,
		"uri", metadataURI,
	)
	nftData.MetadataAccessed = common.Ptr(time.Now())
	rc, err := multiproto.Get(ctx, ipfsClient, metadataURI)
	if err != nil {
		// TODO: Retry on some errors? See #532.
		logger.Info("error downloading token metadata",
			"uri", metadataURI,
			"err", err,
		)
		return nil
	}
	// 1. Read metadata into interface{} to allow any syntactically correct
	//    JSON value.
	var metadataAny interface{}
	limitedReader := io.LimitReader(rc, MaxMetadataBytes)
	if err = json.NewDecoder(limitedReader).Decode(&metadataAny); err != nil {
		logger.Info("error decoding token metadata as any",
			"uri", metadataURI,
			"err", err,
		)
		if err = rc.Close(); err != nil {
			return fmt.Errorf("closing metadata reader: %w", err)
		}
		return nil
	}
	if err = rc.Close(); err != nil {
		return fmt.Errorf("closing metadata reader: %w", err)
	}
	// 2. Re-serialize into JSON for database. This is to normalize the
	//    syntax, in case Go and our database have slightly different opinions
	//    on what's valid JSON.
	metadataNormalJSONBuf, err := json.Marshal(metadataAny)
	if err != nil {
		logger.Info("error re-encoding token metadata",
			"uri", metadataURI,
			"err", err,
		)
		return nil
	}
	nftData.Metadata = common.Ptr(string(metadataNormalJSONBuf))
	// 3. Parse it into the ERC-721 asset metadata schema. There's no function
	//    to convert these map[string]interface{} things into structs, so
	//    we'll just parse it again.
	var metadata ERC721AssetMetadata
	if err = json.Unmarshal(metadataNormalJSONBuf, &metadata); err != nil {
		logger.Info("error decoding token metadata as asset metadata",
			"uri", metadataURI,
			"err", err,
		)
		return nil
	}
	nftData.Name = metadata.Name
	nftData.Description = metadata.Description
	nftData.Image = metadata.Image
	return nil
}

func evmDownloadNFTERC721(ctx context.Context, logger *log.Logger, source nodeapi.RuntimeApiLite, ipfsClient ipfsclient.Client, round uint64, tokenEthAddr []byte, id *big.Int) (*EVMNFTData, error) {
	var nftData EVMNFTData
	supportsMetadata, err := detectInterface(ctx, logger, source, round, tokenEthAddr, ERC721MetadataInterfaceID)
	if err != nil {
		return nil, fmt.Errorf("checking ERC721Metadata interface: %w", err)
	}
	if supportsMetadata {
		if err = evmDownloadNFTERC721Metadata(ctx, logger, source, ipfsClient, &nftData, round, tokenEthAddr, id); err != nil {
			return nil, err
		}
	}
	return &nftData, nil
}

func evmDownloadTokenBalanceERC721(ctx context.Context, logger *log.Logger, source nodeapi.RuntimeApiLite, round uint64, tokenEthAddr []byte, accountEthAddr []byte) (*EVMTokenBalanceData, error) {
	var balanceData EVMTokenBalanceData
	accountECAddr := ethCommon.BytesToAddress(accountEthAddr)
	if err := evmCallWithABI(ctx, source, round, tokenEthAddr, evmabi.ERC721, &balanceData.Balance, "balanceOf", accountECAddr); err != nil {
		if !errors.Is(err, EVMDeterministicError{}) {
			return nil, fmt.Errorf("calling balanceOf: %w", err)
		}
		logDeterministicError(logger, round, tokenEthAddr, "ERC721", "balanceOf", err,
			"account_eth_addr_hex", hex.EncodeToString(accountEthAddr),
		)
		return nil, nil
	}
	return &balanceData, nil
}
