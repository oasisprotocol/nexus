package emerald

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"math/big"

	"github.com/oasisprotocol/oasis-core/go/common/cbor"
	"github.com/oasisprotocol/oasis-core/go/common/crypto/address"
	"github.com/oasisprotocol/oasis-core/go/roothash/api/block"
	sdkClient "github.com/oasisprotocol/oasis-sdk/client-sdk/go/client"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/modules/accounts"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/modules/consensusaccounts"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/modules/core"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/modules/evm"
	sdkTypes "github.com/oasisprotocol/oasis-sdk/client-sdk/go/types"

	"github.com/oasisprotocol/oasis-indexer/analyzer"
	"github.com/oasisprotocol/oasis-indexer/analyzer/modules"
	common "github.com/oasisprotocol/oasis-indexer/analyzer/uncategorized"
	"github.com/oasisprotocol/oasis-indexer/log"
	"github.com/oasisprotocol/oasis-indexer/storage"
)

// todo: erc721, erc1155

type BlockTransactionSignerData struct {
	Index   int
	Address string
	Nonce   int
}

type BlockTransactionData struct {
	Index                   int
	Hash                    string
	EthHash                 *string
	Raw                     []byte
	RawResult               []byte
	SignerData              []*BlockTransactionSignerData
	RelatedAccountAddresses map[string]bool
}

type AddressPreimageData struct {
	ContextIdentifier string
	ContextVersion    int
	Data              []byte
}

type TokenChangeKey struct {
	// TokenAddress is the Oasis address of the smart contract of the
	// compatible (e.g. ERC-20) token.
	TokenAddress string
	// AccountAddress is the Oasis address of the owner of some amount of the
	// compatible (e.g. ERC-20) token.
	AccountAddress string
}

type BlockData struct {
	Hash                string
	NumTransactions     int
	GasUsed             uint64
	Size                int
	TransactionData     []*BlockTransactionData
	AddressPreimages    map[string]*AddressPreimageData
	TokenBalanceChanges map[TokenChangeKey]*big.Int
	// key is oasis bech32 address
	PossibleTokens map[string]*modules.EVMPossibleToken
}

// Function naming conventions in this file:
// 'extract-' -> dataflow from parameters to return values, no side effects. suitable for processing pieces of data
//   that doesn't affect their siblings
// 'register-' -> dataflow from input parameters to output parameters, side effects. may have dataflow of something
//   useful to return values as well, to entice developers to use these functions instead of e.g. converting an address
//   manually and inadvertently leaving it out of a related address or address preimage map
// 'visit-' -> dataflow from generic parameter to specific callback, no side effects, although callbacks will have side
//   effects. suitable for processing smaller pieces of data that contribute to aggregated structures

func extractAddressPreimage(as *sdkTypes.AddressSpec) (*AddressPreimageData, error) {
	// Adapted from oasis-sdk/client-sdk/go/types/transaction.go.
	var (
		ctx  address.Context
		data []byte
	)
	switch {
	case as.Signature != nil:
		spec := as.Signature
		switch {
		case spec.Ed25519 != nil:
			ctx = sdkTypes.AddressV0Ed25519Context
			data, _ = spec.Ed25519.MarshalBinary()
		case spec.Secp256k1Eth != nil:
			ctx = sdkTypes.AddressV0Secp256k1EthContext
			// Use a scheme such that we can compute Secp256k1 addresses from Ethereum
			// addresses as this makes things more interoperable.
			untaggedPk, _ := spec.Secp256k1Eth.MarshalBinaryUncompressedUntagged()
			data = common.SliceEthAddress(common.Keccak256(untaggedPk))
		case spec.Sr25519 != nil:
			ctx = sdkTypes.AddressV0Sr25519Context
			data, _ = spec.Sr25519.MarshalBinary()
		default:
			panic("address: unsupported public key type")
		}
	case as.Multisig != nil:
		config := as.Multisig
		ctx = sdkTypes.AddressV0MultisigContext
		data = cbor.Marshal(config)
	default:
		return nil, fmt.Errorf("malformed AddressSpec")
	}
	return &AddressPreimageData{
		ContextIdentifier: ctx.Identifier,
		ContextVersion:    int(ctx.Version),
		Data:              data,
	}, nil
}

func registerAddressSpec(addressPreimages map[string]*AddressPreimageData, as *sdkTypes.AddressSpec) (string, error) {
	addr, err := common.StringifyAddressSpec(as)
	if err != nil {
		return "", err
	}

	if _, ok := addressPreimages[addr]; !ok {
		preimageData, err1 := extractAddressPreimage(as)
		if err1 != nil {
			return "", fmt.Errorf("extract address preimage: %w", err1)
		}
		addressPreimages[addr] = preimageData
	}

	return addr, nil
}

func registerEthAddress(addressPreimages map[string]*AddressPreimageData, ethAddr []byte) (string, error) {
	addr, err := common.StringifyEthAddress(ethAddr)
	if err != nil {
		return "", err
	}

	if _, ok := addressPreimages[addr]; !ok {
		addressPreimages[addr] = &AddressPreimageData{
			ContextIdentifier: sdkTypes.AddressV0Secp256k1EthContext.Identifier,
			ContextVersion:    int(sdkTypes.AddressV0Secp256k1EthContext.Version),
			Data:              ethAddr,
		}
	}

	return addr, nil
}

//nolint:unparam
func registerRelatedSdkAddress(relatedAddresses map[string]bool, sdkAddr *sdkTypes.Address) (string, error) {
	addr, err := common.StringifySdkAddress(sdkAddr)
	if err != nil {
		return "", err
	}

	relatedAddresses[addr] = true

	return addr, nil
}

func registerRelatedAddressSpec(addressPreimages map[string]*AddressPreimageData, relatedAddresses map[string]bool, as *sdkTypes.AddressSpec) (string, error) {
	addr, err := registerAddressSpec(addressPreimages, as)
	if err != nil {
		return "", err
	}

	relatedAddresses[addr] = true

	return addr, nil
}

func registerRelatedEthAddress(addressPreimages map[string]*AddressPreimageData, relatedAddresses map[string]bool, ethAddr []byte) (string, error) {
	addr, err := registerEthAddress(addressPreimages, ethAddr)
	if err != nil {
		return "", err
	}

	relatedAddresses[addr] = true

	return addr, nil
}

func findTokenChange(tokenChanges map[TokenChangeKey]*big.Int, contractAddr string, accountAddr string) *big.Int {
	key := TokenChangeKey{contractAddr, accountAddr}
	change, ok := tokenChanges[key]
	if !ok {
		change = &big.Int{}
		tokenChanges[key] = change
	}
	return change
}

func registerTokenIncrease(tokenChanges map[TokenChangeKey]*big.Int, contractAddr string, accountAddr string, amount *big.Int) {
	change := findTokenChange(tokenChanges, contractAddr, accountAddr)
	change.Add(change, amount)
}

func registerTokenDecrease(tokenChanges map[TokenChangeKey]*big.Int, contractAddr string, accountAddr string, amount *big.Int) {
	change := findTokenChange(tokenChanges, contractAddr, accountAddr)
	change.Sub(change, amount)
}

//nolint:gocyclo
func extractRound(b *block.Block, txrs []*sdkClient.TransactionWithResults, logger *log.Logger) (*BlockData, error) {
	var blockData BlockData
	blockData.Hash = b.Header.EncodedHash().String()
	blockData.NumTransactions = len(txrs)
	blockData.TransactionData = make([]*BlockTransactionData, 0, len(txrs))
	blockData.AddressPreimages = map[string]*AddressPreimageData{}
	blockData.TokenBalanceChanges = map[TokenChangeKey]*big.Int{}
	blockData.PossibleTokens = map[string]*modules.EVMPossibleToken{}
	for txIndex, txr := range txrs {
		var blockTransactionData BlockTransactionData
		blockTransactionData.Index = txIndex
		blockTransactionData.Hash = txr.Tx.Hash().Hex()
		if len(txr.Tx.AuthProofs) == 1 && txr.Tx.AuthProofs[0].Module == "evm.ethereum.v0" {
			ethHash := hex.EncodeToString(common.Keccak256(txr.Tx.Body))
			blockTransactionData.EthHash = &ethHash
		}
		blockTransactionData.Raw = cbor.Marshal(txr.Tx)
		blockTransactionData.RawResult = cbor.Marshal(txr.Result)
		blockTransactionData.RelatedAccountAddresses = map[string]bool{}
		tx, err := common.OpenUtxNoVerify(&txr.Tx)
		if err != nil {
			logger.Error("error decoding tx, skipping tx-specific analysis",
				"round", b.Header.Round,
				"tx_index", txIndex,
				"tx_hash", txr.Tx.Hash(),
				"err", err,
			)
			tx = nil
		}
		if tx != nil {
			blockTransactionData.SignerData = make([]*BlockTransactionSignerData, 0, len(tx.AuthInfo.SignerInfo))
			for j, si := range tx.AuthInfo.SignerInfo {
				var blockTransactionSignerData BlockTransactionSignerData
				blockTransactionSignerData.Index = j
				addr, err1 := registerRelatedAddressSpec(blockData.AddressPreimages, blockTransactionData.RelatedAccountAddresses, &si.AddressSpec)
				if err1 != nil {
					return nil, fmt.Errorf("tx %d signer %d visit address spec: %w", txIndex, j, err1)
				}
				blockTransactionSignerData.Address = addr
				blockTransactionSignerData.Nonce = int(si.Nonce)
				blockTransactionData.SignerData = append(blockTransactionData.SignerData, &blockTransactionSignerData)
			}
			if err = common.VisitCall(&tx.Call, &txr.Result, &common.CallHandler{
				AccountsTransfer: func(body *accounts.Transfer) error {
					if _, err = registerRelatedSdkAddress(blockTransactionData.RelatedAccountAddresses, &body.To); err != nil {
						return fmt.Errorf("to: %w", err)
					}
					return nil
				},
				ConsensusAccountsDeposit: func(body *consensusaccounts.Deposit) error {
					if body.To != nil {
						if _, err = registerRelatedSdkAddress(blockTransactionData.RelatedAccountAddresses, body.To); err != nil {
							return fmt.Errorf("to: %w", err)
						}
					}
					return nil
				},
				ConsensusAccountsWithdraw: func(body *consensusaccounts.Withdraw) error {
					// .To is from another chain, so exclude?
					return nil
				},
				EvmCreate: func(body *evm.Create, ok *[]byte) error {
					if !txr.Result.IsUnknown() && txr.Result.IsSuccess() && len(*ok) == 32 {
						// todo: is this rigorous enough?
						if _, err = registerRelatedEthAddress(blockData.AddressPreimages, blockTransactionData.RelatedAccountAddresses, common.SliceEthAddress(*ok)); err != nil {
							return fmt.Errorf("created contract: %w", err)
						}
					}
					return nil
				},
				EvmCall: func(body *evm.Call, ok *[]byte) error {
					if _, err = registerRelatedEthAddress(blockData.AddressPreimages, blockTransactionData.RelatedAccountAddresses, body.Address); err != nil {
						return fmt.Errorf("address: %w", err)
					}
					// todo: maybe parse known token methods
					return nil
				},
			}); err != nil {
				return nil, fmt.Errorf("tx %d: %w", txIndex, err)
			}
		}
		var txGasUsed uint64
		foundGasUsedEvent := false
		if err = common.VisitSdkEvents(txr.Events, &common.SdkEventHandler{
			Core: func(event *core.Event) error {
				if event.GasUsed != nil {
					if foundGasUsedEvent {
						return fmt.Errorf("multiple gas used events")
					}
					foundGasUsedEvent = true
					txGasUsed = event.GasUsed.Amount
				}
				return nil
			},
			Accounts: func(event *accounts.Event) error {
				if event.Transfer != nil {
					if _, err1 := registerRelatedSdkAddress(blockTransactionData.RelatedAccountAddresses, &event.Transfer.From); err1 != nil {
						return fmt.Errorf("from: %w", err1)
					}
					if _, err1 := registerRelatedSdkAddress(blockTransactionData.RelatedAccountAddresses, &event.Transfer.To); err1 != nil {
						return fmt.Errorf("to: %w", err1)
					}
				}
				if event.Burn != nil {
					if _, err1 := registerRelatedSdkAddress(blockTransactionData.RelatedAccountAddresses, &event.Burn.Owner); err1 != nil {
						return fmt.Errorf("owner: %w", err1)
					}
				}
				if event.Mint != nil {
					if _, err1 := registerRelatedSdkAddress(blockTransactionData.RelatedAccountAddresses, &event.Mint.Owner); err1 != nil {
						return fmt.Errorf("owner: %w", err1)
					}
				}
				return nil
			},
			ConsensusAccounts: func(event *consensusaccounts.Event) error {
				if event.Deposit != nil {
					// .From is from another chain, so exclude?
					if _, err1 := registerRelatedSdkAddress(blockTransactionData.RelatedAccountAddresses, &event.Deposit.To); err1 != nil {
						return fmt.Errorf("from: %w", err1)
					}
				}
				if event.Withdraw != nil {
					if _, err1 := registerRelatedSdkAddress(blockTransactionData.RelatedAccountAddresses, &event.Withdraw.From); err1 != nil {
						return fmt.Errorf("from: %w", err1)
					}
					// .To is from another chain, so exclude?
				}
				return nil
			},
			Evm: func(event *evm.Event) error {
				eventAddr, err1 := registerRelatedEthAddress(blockData.AddressPreimages, blockTransactionData.RelatedAccountAddresses, event.Address)
				if err1 != nil {
					return fmt.Errorf("event address: %w", err1)
				}
				if err1 = common.VisitEvmEvent(event, &common.EvmEventHandler{
					Erc20Transfer: func(fromEthAddr []byte, toEthAddr []byte, amountU256 []byte) error {
						amount := &big.Int{}
						amount.SetBytes(amountU256)
						if !bytes.Equal(fromEthAddr, common.ZeroEthAddr) {
							fromAddr, err2 := registerRelatedEthAddress(blockData.AddressPreimages, blockTransactionData.RelatedAccountAddresses, fromEthAddr)
							if err2 != nil {
								return fmt.Errorf("from: %w", err2)
							}
							registerTokenDecrease(blockData.TokenBalanceChanges, eventAddr, fromAddr, amount)
						}
						if !bytes.Equal(toEthAddr, common.ZeroEthAddr) {
							toAddr, err2 := registerRelatedEthAddress(blockData.AddressPreimages, blockTransactionData.RelatedAccountAddresses, toEthAddr)
							if err2 != nil {
								return fmt.Errorf("to: %w", err2)
							}
							registerTokenIncrease(blockData.TokenBalanceChanges, eventAddr, toAddr, amount)
						}
						blockData.PossibleTokens[eventAddr] = &modules.EVMPossibleToken{
							PossibleTypes: []modules.EVMTokenType{modules.EVMTokenTypeERC20},
							EthAddr:       event.Address,
						}
						return nil
					},
					Erc20Approval: func(ownerEthAddr []byte, spenderEthAddr []byte, amountU256 []byte) error {
						if !bytes.Equal(ownerEthAddr, common.ZeroEthAddr) {
							_, err2 := registerRelatedEthAddress(blockData.AddressPreimages, blockTransactionData.RelatedAccountAddresses, ownerEthAddr)
							if err2 != nil {
								return fmt.Errorf("owner: %w", err2)
							}
						}
						if !bytes.Equal(spenderEthAddr, common.ZeroEthAddr) {
							_, err2 := registerRelatedEthAddress(blockData.AddressPreimages, blockTransactionData.RelatedAccountAddresses, spenderEthAddr)
							if err2 != nil {
								return fmt.Errorf("spender: %w", err2)
							}
						}
						blockData.PossibleTokens[eventAddr] = &modules.EVMPossibleToken{
							PossibleTypes: []modules.EVMTokenType{modules.EVMTokenTypeERC20},
							EthAddr:       event.Address,
						}
						return nil
					},
				}); err1 != nil {
					return err1
				}
				return nil
			},
		}); err != nil {
			return nil, fmt.Errorf("tx %d: %w", txIndex, err)
		}
		if !foundGasUsedEvent {
			if (txr.Result.IsUnknown() || txr.Result.IsSuccess()) && tx != nil {
				// Treat as if it used all the gas.
				txGasUsed = tx.AuthInfo.Fee.Gas
			} else { //nolint:staticcheck
				// Inaccurate: Treat as not using any gas.
			}
		}
		blockData.TransactionData = append(blockData.TransactionData, &blockTransactionData)
		// If this overflows, it will do so silently. However, supported
		// runtimes internally use u64 checked math to impose a batch gas,
		// which will prevent it from emitting blocks that use enough gas to
		// do that.
		blockData.GasUsed += txGasUsed
		// Inaccurate: Re-serialize signed tx to estimate original size.
		txSize := len(cbor.Marshal(txr.Tx))
		blockData.Size += txSize
	}
	return &blockData, nil
}

func emitRoundBatch(batch *storage.QueryBatch, qf *analyzer.QueryFactory, round uint64, blockData *BlockData, blockTokenData *modules.EVMBlockTokenData) {
	for _, transactionData := range blockData.TransactionData {
		for _, signerData := range transactionData.SignerData {
			batch.Queue(
				qf.RuntimeTransactionSignerInsertQuery(),
				round,
				transactionData.Index,
				signerData.Index,
				signerData.Address,
				signerData.Nonce,
			)
		}
		for addr := range transactionData.RelatedAccountAddresses {
			batch.Queue(qf.RuntimeRelatedTransactionInsertQuery(), addr, round, transactionData.Index)
		}
		batch.Queue(qf.RuntimeTransactionInsertQuery(), round, transactionData.Index, transactionData.Hash, transactionData.EthHash, transactionData.Raw, transactionData.RawResult)
	}
	for addr, preimageData := range blockData.AddressPreimages {
		batch.Queue(qf.AddressPreimageInsertQuery(), addr, preimageData.ContextIdentifier, preimageData.ContextVersion, preimageData.Data)
	}
	for key, change := range blockData.TokenBalanceChanges {
		batch.Queue(qf.RuntimeTokenBalanceUpdateQuery(), key.TokenAddress, key.AccountAddress, change.String())
	}
	for tokenAddr, tokenData := range blockTokenData.TokenData {
		batch.Queue(
			qf.RuntimeTokenUpdateQuery(),
			tokenAddr,
			tokenData.Name,
			tokenData.Symbol,
			tokenData.Decimals,
			tokenData.TotalSupply.String(),
		)
	}
}
