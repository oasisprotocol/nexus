package runtime

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"math/big"
	"time"

	ethCommon "github.com/ethereum/go-ethereum/common"
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
	apiTypes "github.com/oasisprotocol/oasis-indexer/api/v1/types"
	"github.com/oasisprotocol/oasis-indexer/log"
	"github.com/oasisprotocol/oasis-indexer/storage"
)

// todo: erc721, erc1155

type BlockTransactionSignerData struct {
	Index   int
	Address apiTypes.Address
	Nonce   int
}

type BlockTransactionData struct {
	Index                   int
	Hash                    string
	EthHash                 *string
	Raw                     []byte
	RawResult               []byte
	SignerData              []*BlockTransactionSignerData
	RelatedAccountAddresses map[apiTypes.Address]bool
}

type EventBody interface{}

type EventData struct {
	TxIndex          int
	TxHash           string
	Type             apiTypes.RuntimeEventType
	Body             EventBody
	EvmLogName       string
	EvmLogParams     []*apiTypes.EvmEventParam
	RelatedAddresses map[apiTypes.Address]bool
}

type extractEventResult struct {
	ExtractedEvents   []*EventData
	FoundGasUsedEvent bool
	TxGasUsed         uint64
}

type AddressPreimageData struct {
	ContextIdentifier string
	ContextVersion    int
	Data              []byte
}

type TokenChangeKey struct {
	// TokenAddress is the Oasis address of the smart contract of the
	// compatible (e.g. ERC-20) token.
	TokenAddress apiTypes.Address
	// AccountAddress is the Oasis address of the owner of some amount of the
	// compatible (e.g. ERC-20) token.
	AccountAddress apiTypes.Address
}

type BlockData struct {
	Hash                string
	Timestamp           time.Time
	NumTransactions     int
	GasUsed             uint64
	Size                int
	TransactionData     []*BlockTransactionData
	EventData           []*EventData
	AddressPreimages    map[apiTypes.Address]*AddressPreimageData
	TokenBalanceChanges map[TokenChangeKey]*big.Int
	// key is oasis bech32 address
	PossibleTokens map[apiTypes.Address]*modules.EVMPossibleToken
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

func registerAddressSpec(addressPreimages map[apiTypes.Address]*AddressPreimageData, as *sdkTypes.AddressSpec) (apiTypes.Address, error) {
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

func registerEthAddress(addressPreimages map[apiTypes.Address]*AddressPreimageData, ethAddr []byte) (apiTypes.Address, error) {
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

func registerRelatedSdkAddress(relatedAddresses map[apiTypes.Address]bool, sdkAddr *sdkTypes.Address) (apiTypes.Address, error) {
	addr, err := common.StringifySdkAddress(sdkAddr)
	if err != nil {
		return "", err
	}

	relatedAddresses[addr] = true

	return addr, nil
}

func registerRelatedAddressSpec(addressPreimages map[apiTypes.Address]*AddressPreimageData, relatedAddresses map[apiTypes.Address]bool, as *sdkTypes.AddressSpec) (apiTypes.Address, error) {
	addr, err := registerAddressSpec(addressPreimages, as)
	if err != nil {
		return "", err
	}

	relatedAddresses[addr] = true

	return addr, nil
}

func registerRelatedEthAddress(addressPreimages map[apiTypes.Address]*AddressPreimageData, relatedAddresses map[apiTypes.Address]bool, ethAddr []byte) (apiTypes.Address, error) {
	addr, err := registerEthAddress(addressPreimages, ethAddr)
	if err != nil {
		return "", err
	}

	relatedAddresses[addr] = true

	return addr, nil
}

func findTokenChange(tokenChanges map[TokenChangeKey]*big.Int, contractAddr apiTypes.Address, accountAddr apiTypes.Address) *big.Int {
	key := TokenChangeKey{contractAddr, accountAddr}
	change, ok := tokenChanges[key]
	if !ok {
		change = &big.Int{}
		tokenChanges[key] = change
	}
	return change
}

func registerTokenIncrease(tokenChanges map[TokenChangeKey]*big.Int, contractAddr apiTypes.Address, accountAddr apiTypes.Address, amount *big.Int) {
	change := findTokenChange(tokenChanges, contractAddr, accountAddr)
	change.Add(change, amount)
}

func registerTokenDecrease(tokenChanges map[TokenChangeKey]*big.Int, contractAddr apiTypes.Address, accountAddr apiTypes.Address, amount *big.Int) {
	change := findTokenChange(tokenChanges, contractAddr, accountAddr)
	change.Sub(change, amount)
}

func extractRound(b *block.Block, txrs []*sdkClient.TransactionWithResults, logger *log.Logger) (*BlockData, error) {
	var blockData BlockData
	blockData.Hash = b.Header.EncodedHash().String()
	blockData.Timestamp = time.Unix(int64(b.Header.Timestamp), 0 /* nanos */)
	blockData.NumTransactions = len(txrs)
	blockData.TransactionData = make([]*BlockTransactionData, 0, len(txrs))
	blockData.EventData = []*EventData{}
	blockData.AddressPreimages = map[apiTypes.Address]*AddressPreimageData{}
	blockData.TokenBalanceChanges = map[TokenChangeKey]*big.Int{}
	blockData.PossibleTokens = map[apiTypes.Address]*modules.EVMPossibleToken{}
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
		blockTransactionData.RelatedAccountAddresses = map[apiTypes.Address]bool{}
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
				EVMCreate: func(body *evm.Create, ok *[]byte) error {
					if !txr.Result.IsUnknown() && txr.Result.IsSuccess() && len(*ok) == 32 {
						// todo: is this rigorous enough?
						if _, err = registerRelatedEthAddress(blockData.AddressPreimages, blockTransactionData.RelatedAccountAddresses, common.SliceEthAddress(*ok)); err != nil {
							return fmt.Errorf("created contract: %w", err)
						}
					}
					return nil
				},
				EVMCall: func(body *evm.Call, ok *[]byte) error {
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
		res, err := extractEvents(&blockData, blockTransactionData.RelatedAccountAddresses, txr.Events)
		if err != nil {
			return nil, fmt.Errorf("tx %d: %w", txIndex, err)
		}
		// Populate eventData with tx-specific data.
		for _, eventData := range res.ExtractedEvents {
			eventData.TxIndex = txIndex
			eventData.TxHash = blockTransactionData.Hash
		}
		if !res.FoundGasUsedEvent {
			if (txr.Result.IsUnknown() || txr.Result.IsSuccess()) && tx != nil {
				// Treat as if it used all the gas.
				logger.Info("tx didn't emit a core.GasUsed event, assuming it used max allowed gas", "tx_hash", txr.Tx.Hash(), "assumed_gas_used", tx.AuthInfo.Fee.Gas)
				res.TxGasUsed = tx.AuthInfo.Fee.Gas
			} else { //nolint:staticcheck
				// Inaccurate: Treat as not using any gas.
			}
		}
		blockData.TransactionData = append(blockData.TransactionData, &blockTransactionData)
		blockData.EventData = append(blockData.EventData, res.ExtractedEvents...)
		// If this overflows, it will do so silently. However, supported
		// runtimes internally use u64 checked math to impose a batch gas,
		// which will prevent it from emitting blocks that use enough gas to
		// do that.
		blockData.GasUsed += res.TxGasUsed
		// Inaccurate: Re-serialize signed tx to estimate original size.
		txSize := len(cbor.Marshal(txr.Tx))
		blockData.Size += txSize
	}
	return &blockData, nil
}

func extractEvents(blockData *BlockData, relatedAccountAddresses map[apiTypes.Address]bool, eventsRaw []*sdkTypes.Event) (*extractEventResult, error) {
	extractedEvents := []*EventData{}
	foundGasUsedEvent := false
	var txGasUsed uint64
	if err := common.VisitSdkEvents(eventsRaw, &common.SdkEventHandler{
		Core: func(event *core.Event) error {
			if event.GasUsed != nil {
				if foundGasUsedEvent {
					return fmt.Errorf("multiple gas used events")
				}
				foundGasUsedEvent = true
				txGasUsed = event.GasUsed.Amount
				eventData := EventData{
					Type: apiTypes.RuntimeEventTypeCoreGasUsed,
					Body: event.GasUsed,
				}
				extractedEvents = append(extractedEvents, &eventData)
			}
			return nil
		},
		Accounts: func(event *accounts.Event) error {
			if event.Transfer != nil {
				fromAddr, err1 := registerRelatedSdkAddress(relatedAccountAddresses, &event.Transfer.From)
				if err1 != nil {
					return fmt.Errorf("from: %w", err1)
				}
				toAddr, err1 := registerRelatedSdkAddress(relatedAccountAddresses, &event.Transfer.To)
				if err1 != nil {
					return fmt.Errorf("to: %w", err1)
				}
				eventRelatedAddresses := map[apiTypes.Address]bool{}
				for _, addr := range []apiTypes.Address{fromAddr, toAddr} {
					eventRelatedAddresses[addr] = true
				}
				eventData := EventData{
					Type:             apiTypes.RuntimeEventTypeAccountsTransfer,
					Body:             event.Transfer,
					RelatedAddresses: eventRelatedAddresses,
				}
				extractedEvents = append(extractedEvents, &eventData)
			}
			if event.Burn != nil {
				ownerAddr, err1 := registerRelatedSdkAddress(relatedAccountAddresses, &event.Burn.Owner)
				if err1 != nil {
					return fmt.Errorf("owner: %w", err1)
				}
				eventData := EventData{
					Type:             apiTypes.RuntimeEventTypeAccountsBurn,
					Body:             event.Burn,
					RelatedAddresses: map[apiTypes.Address]bool{ownerAddr: true},
				}
				extractedEvents = append(extractedEvents, &eventData)
			}
			if event.Mint != nil {
				ownerAddr, err1 := registerRelatedSdkAddress(relatedAccountAddresses, &event.Mint.Owner)
				if err1 != nil {
					return fmt.Errorf("owner: %w", err1)
				}
				eventData := EventData{
					Type:             apiTypes.RuntimeEventTypeAccountsMint,
					Body:             event.Mint,
					RelatedAddresses: map[apiTypes.Address]bool{ownerAddr: true},
				}
				extractedEvents = append(extractedEvents, &eventData)
			}
			return nil
		},
		ConsensusAccounts: func(event *consensusaccounts.Event) error {
			if event.Deposit != nil {
				// .From is from another chain, so exclude?
				toAddr, err1 := registerRelatedSdkAddress(relatedAccountAddresses, &event.Deposit.To)
				if err1 != nil {
					return fmt.Errorf("from: %w", err1)
				}
				eventData := EventData{
					Type:             apiTypes.RuntimeEventTypeConsensusAccountsDeposit,
					Body:             event.Deposit,
					RelatedAddresses: map[apiTypes.Address]bool{toAddr: true},
				}
				extractedEvents = append(extractedEvents, &eventData)
			}
			if event.Withdraw != nil {
				fromAddr, err1 := registerRelatedSdkAddress(relatedAccountAddresses, &event.Withdraw.From)
				if err1 != nil {
					return fmt.Errorf("from: %w", err1)
				}
				eventData := EventData{
					Type:             apiTypes.RuntimeEventTypeConsensusAccountsWithdraw,
					Body:             event.Withdraw,
					RelatedAddresses: map[apiTypes.Address]bool{fromAddr: true},
				}
				extractedEvents = append(extractedEvents, &eventData)
				// .To is from another chain, so exclude?
			}
			return nil
		},
		EVM: func(event *evm.Event) error {
			eventAddr, err1 := registerRelatedEthAddress(blockData.AddressPreimages, relatedAccountAddresses, event.Address)
			if err1 != nil {
				return fmt.Errorf("event address: %w", err1)
			}
			eventRelatedAddresses := map[apiTypes.Address]bool{eventAddr: true}
			if err1 = common.VisitEVMEvent(event, &common.EVMEventHandler{
				ERC20Transfer: func(fromEthAddr []byte, toEthAddr []byte, amountU256 []byte) error {
					amount := &big.Int{}
					amount.SetBytes(amountU256)
					fromZero := bytes.Equal(fromEthAddr, common.ZeroEthAddr)
					toZero := bytes.Equal(toEthAddr, common.ZeroEthAddr)
					if !fromZero {
						fromAddr, err2 := registerRelatedEthAddress(blockData.AddressPreimages, relatedAccountAddresses, fromEthAddr)
						if err2 != nil {
							return fmt.Errorf("from: %w", err2)
						}
						eventRelatedAddresses[fromAddr] = true
						registerTokenDecrease(blockData.TokenBalanceChanges, eventAddr, fromAddr, amount)
					}
					if !toZero {
						toAddr, err2 := registerRelatedEthAddress(blockData.AddressPreimages, relatedAccountAddresses, toEthAddr)
						if err2 != nil {
							return fmt.Errorf("to: %w", err2)
						}
						eventRelatedAddresses[toAddr] = true
						registerTokenIncrease(blockData.TokenBalanceChanges, eventAddr, toAddr, amount)
					}
					if _, ok := blockData.PossibleTokens[eventAddr]; !ok {
						blockData.PossibleTokens[eventAddr] = &modules.EVMPossibleToken{}
					}
					// Mark as mutated if transfer is between zero address
					// and nonzero address (either direction) and nonzero
					// amount. These will change the total supply as mint/
					// burn.
					if fromZero != toZero && amount.Cmp(&big.Int{}) != 0 {
						blockData.PossibleTokens[eventAddr].Mutated = true
					}
					evmLogParams := []*apiTypes.EvmEventParam{
						{
							Name:    "from",
							EvmType: "address",
							Value:   ethCommon.BytesToAddress(fromEthAddr),
						},
						{
							Name:    "to",
							EvmType: "address",
							Value:   ethCommon.BytesToAddress(toEthAddr),
						},
						{
							Name:    "value",
							EvmType: "uint256",
							Value:   amount,
						},
					}
					eventData := EventData{
						Type:             apiTypes.RuntimeEventTypeEvmLog,
						Body:             event,
						EvmLogName:       apiTypes.Erc20Transfer,
						EvmLogParams:     evmLogParams,
						RelatedAddresses: eventRelatedAddresses,
					}
					extractedEvents = append(extractedEvents, &eventData)
					return nil
				},
				ERC20Approval: func(ownerEthAddr []byte, spenderEthAddr []byte, amountU256 []byte) error {
					if !bytes.Equal(ownerEthAddr, common.ZeroEthAddr) {
						ownerAddr, err2 := registerRelatedEthAddress(blockData.AddressPreimages, relatedAccountAddresses, ownerEthAddr)
						if err2 != nil {
							return fmt.Errorf("owner: %w", err2)
						}
						eventRelatedAddresses[ownerAddr] = true
					}
					if !bytes.Equal(spenderEthAddr, common.ZeroEthAddr) {
						spenderAddr, err2 := registerRelatedEthAddress(blockData.AddressPreimages, relatedAccountAddresses, spenderEthAddr)
						if err2 != nil {
							return fmt.Errorf("spender: %w", err2)
						}
						eventRelatedAddresses[spenderAddr] = true
					}
					if _, ok := blockData.PossibleTokens[eventAddr]; !ok {
						blockData.PossibleTokens[eventAddr] = &modules.EVMPossibleToken{}
					}
					amount := &big.Int{}
					amount.SetBytes(amountU256)
					evmLogParams := []*apiTypes.EvmEventParam{
						{
							Name:    "owner",
							EvmType: "address",
							Value:   ethCommon.BytesToAddress(ownerEthAddr),
						},
						{
							Name:    "spender",
							EvmType: "address",
							Value:   ethCommon.BytesToAddress(spenderEthAddr),
						},
						{
							Name:    "value",
							EvmType: "uint256",
							Value:   amount,
						},
					}
					eventData := EventData{
						Type:             apiTypes.RuntimeEventTypeEvmLog,
						Body:             event,
						EvmLogName:       apiTypes.Erc20Approval,
						EvmLogParams:     evmLogParams,
						RelatedAddresses: eventRelatedAddresses,
					}
					extractedEvents = append(extractedEvents, &eventData)
					return nil
				},
			}); err1 != nil {
				return err1
			}
			return nil
		},
	}); err != nil {
		return &extractEventResult{}, err
	}
	return &extractEventResult{
		ExtractedEvents:   extractedEvents,
		FoundGasUsedEvent: foundGasUsedEvent,
		TxGasUsed:         txGasUsed,
	}, nil
}

func emitRoundBatch(batch *storage.QueryBatch, qf *analyzer.QueryFactory, round uint64, blockData *BlockData) {
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
		batch.Queue(qf.RuntimeTransactionInsertQuery(), round, transactionData.Index, transactionData.Hash, transactionData.EthHash, blockData.Timestamp, transactionData.Raw, transactionData.RawResult)
	}
	for _, eventData := range blockData.EventData {
		eventRelatedAddresses := common.ExtractAddresses(eventData.RelatedAddresses)
		batch.Queue(
			qf.RuntimeEventInsertQuery(),
			round,
			eventData.TxIndex,
			eventData.TxHash,
			eventData.Type,
			eventData.Body,
			eventData.EvmLogName,
			eventData.EvmLogParams,
			eventRelatedAddresses,
		)
	}
	for addr, preimageData := range blockData.AddressPreimages {
		batch.Queue(qf.AddressPreimageInsertQuery(), addr, preimageData.ContextIdentifier, preimageData.ContextVersion, preimageData.Data)
	}
	for key, change := range blockData.TokenBalanceChanges {
		batch.Queue(qf.RuntimeEvmBalanceUpdateQuery(), key.TokenAddress, key.AccountAddress, change.String())
	}
	for addr, possibleToken := range blockData.PossibleTokens {
		if possibleToken.Mutated {
			batch.Queue(qf.RuntimeEVMTokenAnalysisMutateInsertQuery(), addr, round)
		} else {
			batch.Queue(qf.RuntimeEVMTokenAnalysisInsertQuery(), addr, round)
		}
	}
}
