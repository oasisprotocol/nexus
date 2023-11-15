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
	Type     common.TokenType
	Name     string
	Symbol   string
	Decimals uint8
	*EVMTokenMutableData
}

type EVMTokenMutableData struct {
	TotalSupply *big.Int
}

type EVMNFTData struct {
	MetadataURI      *string
	MetadataAccessed *time.Time
	Metadata         *string
	Name             *string
	Description      *string
	Image            *string
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
	// Unlike `ResultData` and `DataData`, this field is not encrypted data. Rather,
	// it is extracted during encrypted tx parsing and processed in extract.go.
	// If non-null, the tx is an older Sapphire tx and it failed, and `ResultNonce`
	// and `ResultData` will be empty.
	FailedCallResult *sdkTypes.FailedCallResult
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
// with the `Type` field set to `TokenTypeUnsupported`.
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
	logger.Info("new token did not meet any supported standards",
		"round", round,
		"token_eth_addr_hex", hex.EncodeToString(tokenEthAddr),
	)
	return &EVMTokenData{Type: common.TokenTypeUnsupported}, nil
}

// EVMDownloadMutatedToken tries to download the mutable data of a given
// token. If it transiently fails to download the data, it returns with a
// non-nil error. If it deterministically cannot download the data, it returns
// nil with nil error as well. Note that this latter case is not considered an
// error!
func EVMDownloadMutatedToken(ctx context.Context, logger *log.Logger, source nodeapi.RuntimeApiLite, round uint64, tokenEthAddr []byte, tokenType common.TokenType) (*EVMTokenMutableData, error) {
	switch tokenType {
	case common.TokenTypeERC20:
		mutable, err := evmDownloadTokenERC20Mutable(ctx, logger, source, round, tokenEthAddr)
		if err != nil {
			return nil, fmt.Errorf("download token ERC-20 mutable: %w", err)
		}
		return mutable, nil

	case common.TokenTypeERC721:
		mutable, err := evmDownloadTokenERC721Mutable(ctx, logger, source, round, tokenEthAddr)
		if err != nil {
			return nil, fmt.Errorf("download token ERC-721 mutable: %w", err)
		}
		return mutable, nil

	// todo: add support for other token types
	// see https://github.com/oasisprotocol/nexus/issues/225

	default:
		logger.Info("mutated token is not from a supported token type",
			"round", round,
			"token_eth_addr_hex", hex.EncodeToString(tokenEthAddr),
			"token_type", tokenType,
		)
		return nil, nil
	}
}

func EVMDownloadNewNFT(ctx context.Context, logger *log.Logger, source nodeapi.RuntimeApiLite, ipfsClient ipfsclient.Client, round uint64, tokenEthAddr []byte, tokenType common.TokenType, id *big.Int) (*EVMNFTData, error) {
	switch tokenType {
	case common.TokenTypeERC721:
		nftData, err := evmDownloadNFTERC721(ctx, logger, source, ipfsClient, round, tokenEthAddr, id)
		if err != nil {
			return nil, fmt.Errorf("download NFT ERC-721: %w", err)
		}
		return nftData, nil

	default:
		logger.Info("new NFT is not a supported token type",
			"round", round,
			"token_eth_addr_hex", hex.EncodeToString(tokenEthAddr),
			"id", id,
			"token_type", tokenType,
		)
		return &EVMNFTData{}, nil
	}
}

// EVMDownloadTokenBalance tries to download the balance of a given account
// for a given token. If it transiently fails to download the balance, it
// returns with a non-nil error. If it deterministically cannot download the
// balance, it returns nil with nil error as well. Note that this latter case
// is not considered an error!
func EVMDownloadTokenBalance(ctx context.Context, logger *log.Logger, source nodeapi.RuntimeApiLite, round uint64, tokenEthAddr []byte, accountEthAddr []byte, tokenType common.TokenType) (*EVMTokenBalanceData, error) {
	switch tokenType {
	case common.TokenTypeERC20:
		balance, err := evmDownloadTokenBalanceERC20(ctx, logger, source, round, tokenEthAddr, accountEthAddr)
		if err != nil {
			return nil, fmt.Errorf("download token balance ERC-20: %w", err)
		}
		return balance, nil

	case common.TokenTypeERC721:
		balance, err := evmDownloadTokenBalanceERC721(ctx, logger, source, round, tokenEthAddr, accountEthAddr)
		if err != nil {
			return nil, fmt.Errorf("download token balance ERC-721: %w", err)
		}
		return balance, nil

	// todo: add support for other token types
	// see https://github.com/oasisprotocol/nexus/issues/225

	default:
		logger.Info("changed balance is not from a supported token type",
			"round", round,
			"token_eth_addr_hex", hex.EncodeToString(tokenEthAddr),
			"account_eth_addr_hex", hex.EncodeToString(accountEthAddr),
			"token_type", tokenType,
		)
		return nil, nil
	}
}

// EVMMaybeUnmarshalEncryptedData breaks down a possibly encrypted data +
// result into their encryption envelope fields. If the data is not encrypted,
// it returns nil with no error.
//
// LESSON: Oasis paratime transaction structure.
//
// Oasis paratime transactions conform to the oasis format transaction payload:
// https://github.com/oasisprotocol/oasis-sdk/blob/04e8918a9b3eb90caffe115ea2d46b5742438714/client-sdk/go/types/transaction.go#L212
//
// Transaction payloads may be encrypted in oasis. To express that, the
// Transaction.Call field of the transaction would have a call format set, e.g.
//     Call.Format: 'encrypted/x25519-deoxysii'
//
// This gives rise to the concept of outer/inner calls for oasis-format
// transactions, where the inner call would be encrypted and stored in the
// outer Call.Body. However, when the runtime evm module converts ethereum
// format transactions to oasis format transaction payloads, they are never
// in this encrypted mode because there is no native ethereum way to express
// an encrypted transaction.
//
// Thus, for ethereum format transactions, the outer oasis format Call.Format
// is always 0/plain (or equivalently, unset). This outer Call directly has
// the method and body of the transaction. For example, the method may be
// "evm.Call" and the body would be the corresponding struct:
// https://github.com/oasisprotocol/oasis-sdk/blob/04e8918a9b3eb90caffe115ea2d46b5742438714/client-sdk/go/modules/evm/types.go#L13
//
// Here is where we may encounter sapphire-style encryption. In the first and
// most basic case, there is no sapphire-style encryption at all and the fields
// of the evm.Call struct are the exact bytes that would go into the evm.
// Note that Sapphire supports this mode.
//
// The result of this transaction is stored in the associated CallResult struct
// that is returned alongside the transaction in TransactionWithResult:
// https://github.com/oasisprotocol/oasis-sdk/blob/main/client-sdk/go/client/client.go#L146
//
// A transaction may also be in sapphire encrypted mode. In this case, the
// evm.Call.Data or evm.Create.Initcode field is itself a cbor-encoded oasis
// format runtime-sdk Call struct that we term the "sapphire outer call". This
// struct is the same oasis format Transaction.Call mentioned above:
// https://github.com/oasisprotocol/oasis-sdk/blob/04e8918a9b3eb90caffe115ea2d46b5742438714/client-sdk/go/types/transaction.go#L363
//
// In encrypted mode, the sapphire outer call has a Call.Format of
// 'encrypted/x25519-deoxysii', and the call.Body is a cbor-encoded encryption
// envelope, which can be decrypted and parsed into another oasis format Call
// struct that we term the "sapphire inner call". Note that Nexus never has
// the keys the decrypt this envelope to get the inner struct since the
// Sapphire transaction is encrypted. The sapphire inner call.Body has the evm
// input data, and the inner call.Format is always unset.
//
// The results of sapphire encrypted transactions are stored similarly. In the
// first case, we should note that sapphire encrypted transactions may have
// unencrypted results if the inputs were not encrypted. Unencrypted transaction
// results are stored in the same associated CallResult as the results of
// unencrypted transactions. We term this CallResult the "outer call result."
//
// Encrypted results can be recovered by cbor-unmarshalling the outer
// CallResult.Ok field into another CallResult struct that we term the
// "sapphire outer call result." Currently, the sapphire outer CallResult.Ok
// stores the result envelope of successful sapphire encrypted transactions.
// However, in older versions of Sapphire and for non-evm transactions, this
// result envelope is stored in the sapphire outer CallResult.Unknown.
//
// The result envelope contains the encrypted "sapphire inner call result",
// where the sapphire inner CallResult.Ok field contains the output data
// from the evm. Note that Nexus does not have the keys to decrypt and access
// this sapphire inner call result.
//
// Side note: The CallResult struct does not specify its own encryption format.
// Rather, it follows the format of its corresponding oasis format Call. The
// "outer call" Call.Format corresponds to the outer call result, and the
// "sapphire outer call" Call.Format corresponds to the sapphire outer call
// result.
//
// Currently, if an encrypted transaction fails, the error is bubbled up to
// the outer CallResult.Failed. However, in older versions of Sapphire the
// error was stored in the sapphire outer CallResult.Failed.
//
// Lastly, there's also a "not-encryption-but-sapphire-something" case where
// the sapphire outer Call.Format is set to "plain". In this case, there's no
// sapphire inner call and the sapphire outer Call.Body has the evm input data.
// Nexus does not extract the evm input data directly here like the way it does
// for unencrypted evm.Call transactions.
//
// The results of these "plain" transactions are returned in the outer
// CallResult.
//
// Outer oasis Call.Body
//   -> inner oasis Call
//   -> evm.Call
//        -> sapphire outer Call.Body
//             -> sapphire inner Call
//
// To summarize:
// * Non-Sapphire
//   * Tx Body: Stored in oasis outer Call.Body
//   * Success Result: Stored in outer oasis CallResult.Ok
//   * Failure Result: Stored in outer oasis CallResult.Failed
// * Sapphire Encrypted
//   * Tx Body: Encryption envelope stored in sapphire outer Call.Body
//   * Success Result: Encryption envelope stored in either sapphire outer
//                     CallResult.Ok (new) or CallResult.Unknown (old)
//   * Failure Result: Stored in either the sapphire outer CallResult.Failed (old)
//                     or the outer CallResult.Failed (new)
// * Sapphire Plain(0)
//   * Tx Body: Stored in the sapphire outer Call.Body
//   * Success Result: Stored in outer oasis CallResult.Ok
//   * Failure Result: Stored in outer oasis CallResult.Failed

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
	// Plain txs have no encrypted fields to extract.
	case sdkTypes.CallFormatPlain:
		return nil, nil
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
		switch call.Format {
		case sdkTypes.CallFormatEncryptedX25519DeoxysII:
			var resultEnvelope sdkTypes.ResultEnvelopeX25519DeoxysII
			switch {
			// The result is encrypted, in which case it is stored in callResult.Unknown
			case callResult.IsUnknown():
				if err := cbor.Unmarshal(callResult.Unknown, &resultEnvelope); err != nil {
					return nil, fmt.Errorf("outer call result unmarshal unknown: %w", err)
				}
			// The result may be unencrypted, in which case it is stored in callResult.Ok
			//
			// Note: IsUnknown() is already checked above, but we keep it here to make the
			// logic explicit in the event of future refactors. IsSuccess() returns true both
			// when the call succeeded and when it is unknown. However, we want this case
			// to run iff the call succeeded.
			case callResult.IsSuccess() && !callResult.IsUnknown():
				if err := cbor.Unmarshal(callResult.Ok, &resultEnvelope); err != nil {
					return nil, fmt.Errorf("outer call result unmarshal ok: %w", err)
				}
			// For non-evm txs as well as older Sapphire txs, the outer callResult may
			// be Unknown and the inner callResult Failed. In this case, we extract the
			// failed callResult.
			case callResult.Failed != nil:
				encryptedData.FailedCallResult = callResult.Failed
			default:
				return nil, fmt.Errorf("unknown inner callResult type")
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
	return &encryptedData, nil
}
