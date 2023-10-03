package evm

import (
	"context"
	"encoding/hex"
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi"
	ethCommon "github.com/ethereum/go-ethereum/common"
	"github.com/oasisprotocol/oasis-core/go/common/cbor"
	sdkTypes "github.com/oasisprotocol/oasis-sdk/client-sdk/go/types"

	"github.com/oasisprotocol/nexus/analyzer/evmnfts/ipfsclient"
	apiTypes "github.com/oasisprotocol/nexus/api/v1/types"
	"github.com/oasisprotocol/nexus/common"
	"github.com/oasisprotocol/nexus/log"
	"github.com/oasisprotocol/nexus/storage/oasis/nodeapi"
)

// EVMTokenType is a small-ish number that we use to identify what type of
// token each row of the tokens table is. Values aren't consecutive like in an
// enum. Prefer to use the number of the ERC if applicable and available, e.g.
// 20 for ERC-20.
// "Historical reasons" style note: this has grown to include non-EVM token
// types as well.
type EVMTokenType int

const (
	EVMTokenTypeNative      EVMTokenType = -1 // A placeholder type to represent the runtime's native token. No contract should be assigned this type.
	EVMTokenTypeUnsupported EVMTokenType = 0  // A smart contract for which we're confident it's not a supported token kind.
)

// A fake address that is used to represent the native runtime token in contexts
// that are primarily intended for tracking EVM tokens (= contract-based tokens).
const NativeRuntimeTokenAddress = "oasis1runt1menat1vet0ken0000000000000000000000"

// A contract that "looks like" a token contract, e.g. because it emitted a Transfer event.
type EVMPossibleToken struct {
	// True if a mutable property of the token (e.g. total_supply) has changed and this
	// module wants to indicate that the contract should be queried to get the new value
	// of the mutable property (or ideally just verify it if we'll also dead-reckon it).
	Mutated            bool
	TotalSupplyChange  big.Int
	NumTransfersChange uint64
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

type EVMNFTData struct {
	MetadataURI      string
	MetadataAccessed time.Time
	Name             string
	Description      string
	Image            string
}

type EVMTokenBalanceData struct {
	// Balance... if you're here to ask about why there's a "balance" struct
	// with a Balance field, it's because the struct is really a little
	// document that the EVMDownloadTokenBalance function can optionally give
	// you about an account. (And we didn't name the struct "account" because
	// the only thing inside it is the balance.) We let that function return a
	// *EVMTokenBalanceData so that it can return nil if it can determine that
	// the contract is not supported. Plus, Go's idea of an arbitrary size
	// integer is *big.Int, and we don't want anyone fainting if they see a
	// ** in the codebase.
	Balance *big.Int
}

type EVMEncryptedData struct {
	Format      common.CallFormat
	PublicKey   []byte
	DataNonce   []byte
	DataData    []byte
	ResultNonce []byte
	ResultData  []byte
}

type EVMContractData struct {
	Address          apiTypes.Address
	CreationBytecode []byte
	CreationTx       string
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

func evmCallWithABICustom(
	ctx context.Context,
	source nodeapi.RuntimeApiLite,
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
	res, err := source.EVMSimulateCall(ctx, round, gasPrice, gasLimit, caller, contractEthAddr, value, inPacked)
	if err != nil {
		return fmt.Errorf("runtime client evm simulate call: %w", err)
	} else if res.DeterministicErr != nil {
		return EVMDeterministicError{res.DeterministicErr}
	}
	outPacked := res.Ok
	if err = contractABI.UnpackIntoInterface(result, method, outPacked); err != nil {
		err = fmt.Errorf("unpacking evm simulate call output: %w", err)
		err = EVMDeterministicError{err}
		return err
	}
	return nil
}

var (
	// https://github.com/oasisprotocol/oasis-web3-gateway/blob/v3.0.0/rpc/eth/api.go#L403-L408
	DefaultGasPrice        = []byte{1}
	DefaultGasLimit uint64 = 30_000_000
	DefaultCaller          = ethCommon.Address{1}.Bytes()
	DefaultValue           = []byte{0}
)

// evmCallWithABI: Given a runtime `source` and `round`, and given an EVM
// smart contract (at `contractEthAddr`, with `contractABI`) deployed in that
// runtime, invokes `method(params...)` in that smart contract. The method
// output is unpacked into `result`, so its type must match the output type of
// `method`.
func evmCallWithABI(
	ctx context.Context,
	source nodeapi.RuntimeApiLite,
	round uint64,
	contractEthAddr []byte,
	contractABI *abi.ABI,
	result interface{},
	method string,
	params ...interface{},
) error {
	return evmCallWithABICustom(ctx, source, round, DefaultGasPrice, DefaultGasLimit, DefaultCaller, contractEthAddr, DefaultValue, contractABI, result, method, params...)
}

// logDeterministicError is for if we know how to handle a deterministic
// error--in those cases you can use this to make a note of the error. Just in
// case someone wasn't expecting it, you know?
func logDeterministicError(logger *log.Logger, round uint64, contractEthAddr []byte, interfaceName string, method string, err error, keyvals ...interface{}) {
	keyvals = append([]interface{}{
		"round", round,
		"contract_eth_addr_hex", hex.EncodeToString(contractEthAddr),
		"interface_name", interfaceName,
		"method", method,
		"err", err,
	}, keyvals...)
	logger.Info("call failed deterministically", keyvals...)
}

// EVMDownloadNewToken tries to download the data of a given token. If it
// transiently fails to download the data, it returns with a non-nil error. If
// it deterministically cannot download the data, it returns a struct
// with the `Type` field set to `EVMTokenTypeUnsupported`.
func EVMDownloadNewToken(ctx context.Context, logger *log.Logger, source nodeapi.RuntimeApiLite, round uint64, tokenEthAddr []byte) (*EVMTokenData, error) {
	supportsERC165, err := detectERC165(ctx, logger, source, round, tokenEthAddr)
	if err != nil {
		return nil, fmt.Errorf("detect ERC165: %w", err)
	}
	if supportsERC165 {
		// Note: Per spec, every ERC-721 token has to support ERC-165.
		supportsERC721, err1 := detectInterface(ctx, logger, source, round, tokenEthAddr, ERC721InterfaceID)
		if err1 != nil {
			return nil, fmt.Errorf("checking ERC721 interface: %w", err1)
		}
		if supportsERC721 {
			tokenData, err2 := evmDownloadTokenERC721(ctx, logger, source, round, tokenEthAddr)
			if err2 != nil {
				return nil, fmt.Errorf("download token ERC-721: %w", err2)
			}
			return tokenData, nil
		}

		// todo: add support for other token types
		// see https://github.com/oasisprotocol/nexus/issues/225
	}

	// Check ERC-20. These tokens typically do not implement ERC-165, so do not bother checking for
	// explicit declaration of support, detect ERC-20 support with heuristics instead.
	tokenData, err := evmDownloadTokenERC20(ctx, logger, source, round, tokenEthAddr)
	if err != nil {
		return nil, fmt.Errorf("download token ERC-20: %w", err)
	}
	if tokenData != nil {
		return tokenData, nil
	}

	// No applicable token discovered.
	return &EVMTokenData{Type: EVMTokenTypeUnsupported}, nil
}

// EVMDownloadMutatedToken tries to download the mutable data of a given
// token. If it transiently fails to download the data, it returns with a
// non-nil error. If it deterministically cannot download the data, it returns
// nil with nil error as well. Note that this latter case is not considered an
// error!
func EVMDownloadMutatedToken(ctx context.Context, logger *log.Logger, source nodeapi.RuntimeApiLite, round uint64, tokenEthAddr []byte, tokenType EVMTokenType) (*EVMTokenMutableData, error) {
	switch tokenType {
	case EVMTokenTypeERC20:
		mutable, err := evmDownloadTokenERC20Mutable(ctx, logger, source, round, tokenEthAddr)
		if err != nil {
			return nil, fmt.Errorf("download token ERC-20 mutable: %w", err)
		}
		return mutable, nil

	case EVMTokenTypeERC721:
		mutable, err := evmDownloadTokenERC721Mutable(ctx, logger, source, round, tokenEthAddr)
		if err != nil {
			return nil, fmt.Errorf("download token ERC-721 mutable: %w", err)
		}
		return mutable, nil

	// todo: add support for other token types
	// see https://github.com/oasisprotocol/nexus/issues/225

	default:
		return nil, fmt.Errorf("download mutated token type %v not handled", tokenType)
	}
}

func EVMDownloadNewNFT(ctx context.Context, logger *log.Logger, source nodeapi.RuntimeApiLite, ipfsClient ipfsclient.Client, round uint64, tokenEthAddr []byte, tokenType EVMTokenType, id *big.Int) (*EVMNFTData, error) {
	switch tokenType {
	case EVMTokenTypeERC721:
		nftData, err := evmDownloadNFTERC721(ctx, logger, source, ipfsClient, round, tokenEthAddr, id)
		if err != nil {
			return nil, fmt.Errorf("download NFT ERC-721: %w", err)
		}
		return nftData, nil

	default:
		return nil, fmt.Errorf("download stale nft type %v not handled", tokenType)
	}
}

// EVMDownloadTokenBalance tries to download the balance of a given account
// for a given token. If it transiently fails to download the balance, it
// returns with a non-nil error. If it deterministically cannot download the
// balance, it returns nil with nil error as well. Note that this latter case
// is not considered an error!
func EVMDownloadTokenBalance(ctx context.Context, logger *log.Logger, source nodeapi.RuntimeApiLite, round uint64, tokenEthAddr []byte, accountEthAddr []byte, tokenType EVMTokenType) (*EVMTokenBalanceData, error) {
	switch tokenType {
	case EVMTokenTypeERC20:
		balance, err := evmDownloadTokenBalanceERC20(ctx, logger, source, round, tokenEthAddr, accountEthAddr)
		if err != nil {
			return nil, fmt.Errorf("download token balance ERC-20: %w", err)
		}
		return balance, nil

	case EVMTokenTypeERC721:
		balance, err := evmDownloadTokenBalanceERC721(ctx, logger, source, round, tokenEthAddr, accountEthAddr)
		if err != nil {
			return nil, fmt.Errorf("download token balance ERC-721: %w", err)
		}
		return balance, nil

	// todo: add support for other token types
	// see https://github.com/oasisprotocol/nexus/issues/225

	default:
		return nil, fmt.Errorf("download stale token balance type %v not handled", tokenType)
	}
}

// EVMMaybeUnmarshalEncryptedData breaks down a possibly encrypted data +
// result into their encryption envelope fields. If the data is not encrypted,
// it returns nil with no error.
func EVMMaybeUnmarshalEncryptedData(data []byte, result *[]byte) (*EVMEncryptedData, error) {
	var encryptedData EVMEncryptedData
	var call sdkTypes.Call
	if cbor.Unmarshal(data, &call) != nil {
		// Invalid CBOR means it's bare Ethereum format data. This is normal.
		// https://github.com/oasisprotocol/oasis-sdk/blob/runtime-sdk/v0.3.0/runtime-sdk/modules/evm/src/lib.rs#L626
		return nil, nil
	}
	encryptedData.Format = common.CallFormat(call.Format.String())
	switch call.Format {
	case sdkTypes.CallFormatEncryptedX25519DeoxysII:
		var callEnvelope sdkTypes.CallEnvelopeX25519DeoxysII
		if err := cbor.Unmarshal(call.Body, &callEnvelope); err != nil {
			return nil, fmt.Errorf("outer call format %s unmarshal body: %w", call.Format, err)
		}
		encryptedData.PublicKey = callEnvelope.Pk[:]
		encryptedData.DataNonce = callEnvelope.Nonce[:]
		encryptedData.DataData = callEnvelope.Data
	// If you are adding new call formats, remember to add them to the
	// database call_format enum too.
	default:
		return nil, fmt.Errorf("outer call format %s (%d) not supported", call.Format, call.Format)
	}
	var callResult sdkTypes.CallResult
	if result != nil {
		if err := cbor.Unmarshal(*result, &callResult); err != nil {
			return nil, fmt.Errorf("unmarshal outer call result: %w", err)
		}
		if callResult.IsUnknown() {
			switch call.Format {
			case sdkTypes.CallFormatEncryptedX25519DeoxysII:
				var resultEnvelope sdkTypes.ResultEnvelopeX25519DeoxysII
				if err := cbor.Unmarshal(callResult.Unknown, &resultEnvelope); err != nil {
					return nil, fmt.Errorf("outer call result unmarshal unknown: %w", err)
				}
				encryptedData.ResultNonce = resultEnvelope.Nonce[:]
				encryptedData.ResultData = resultEnvelope.Data
			default:
				// We have already checked when decoding the call envelope,
				// but I'm keeping this default case here so we don't forget
				// if this code gets restructured.
				return nil, fmt.Errorf("outer call format %s (%d) not supported", call.Format, call.Format)
			}
		}
	}
	return &encryptedData, nil
}
