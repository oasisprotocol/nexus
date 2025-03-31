package runtime

// This file analyzes raw runtime data as fetched from the node, and transforms
// into indexed structures that are suitable/convenient for data insertion into
// the DB.
//
// The main entrypoint is `ExtractRound()`.

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	ethCommon "github.com/ethereum/go-ethereum/common"
	"github.com/oasisprotocol/oasis-core/go/common/cbor"
	"github.com/oasisprotocol/oasis-core/go/common/quantity"
	sdkConfig "github.com/oasisprotocol/oasis-sdk/client-sdk/go/config"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/modules/accounts"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/modules/consensusaccounts"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/modules/core"
	sdkEVM "github.com/oasisprotocol/oasis-sdk/client-sdk/go/modules/evm"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/modules/rofl"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/modules/roflmarket"
	sdkTypes "github.com/oasisprotocol/oasis-sdk/client-sdk/go/types"

	"github.com/oasisprotocol/nexus/analyzer/evmabi"
	"github.com/oasisprotocol/nexus/analyzer/runtime/encryption"
	evm "github.com/oasisprotocol/nexus/analyzer/runtime/evm"
	uncategorized "github.com/oasisprotocol/nexus/analyzer/uncategorized"
	"github.com/oasisprotocol/nexus/analyzer/util/addresses"
	"github.com/oasisprotocol/nexus/analyzer/util/eth"
	apiTypes "github.com/oasisprotocol/nexus/api/v1/types"
	"github.com/oasisprotocol/nexus/common"
	"github.com/oasisprotocol/nexus/log"
	"github.com/oasisprotocol/nexus/storage"
	"github.com/oasisprotocol/nexus/storage/oasis/nodeapi"
)

const (
	TxRevertErrPrefix     = "reverted: "
	DefaultTxRevertErrMsg = "reverted without a message"
)

type BlockTransactionSignerData struct {
	Index   int
	Address apiTypes.Address
	Nonce   int
}

type BlockTransactionData struct {
	Index                   int
	Hash                    string
	EthHash                 *string
	GasUsed                 uint64
	Size                    int
	Raw                     []byte
	RawResult               []byte
	SignerData              []*BlockTransactionSignerData
	RelatedAccountAddresses map[apiTypes.Address]struct{}
	RelatedRoflAddresses    map[nodeapi.AppID]struct{}
	Fee                     common.BigInt
	FeeSymbol               string
	FeeProxyModule          *string
	FeeProxyID              *[]byte
	GasLimit                uint64
	OasisEncrypted          *encryption.EncryptedData
	Method                  string
	Body                    interface{}
	ContractCandidate       *apiTypes.Address // If non-nil, an address that was encountered in the tx and might be a contract.
	To                      *apiTypes.Address // Extracted from the body for convenience. Semantics vary by tx type.
	Amount                  *common.BigInt    // Extracted from the body for convenience. Semantics vary by tx type.
	AmountSymbol            *string           // Extracted from the body for convenience.
	EVMEncrypted            *encryption.EncryptedData
	EVMContract             *evm.EVMContractData
	Success                 *bool
	Error                   *TxError
	IsLikelyTokenTransfer   bool
}

type TxError struct {
	Code   uint32
	Module string
	// The raw error message returned by the node. Note that this may be null.
	// https://github.com/oasisprotocol/oasis-sdk/blob/fb741678585c04fdb413441f2bfba18aafbf98f3/client-sdk/go/types/transaction.go#L488-L492
	RawMessage *string
	// The human-readable error message parsed from RawMessage.
	Message *string
}

type EventBody interface{}

type EventData struct {
	TxIndex          *int    // nil for non-tx events
	TxHash           *string // nil for non-tx events
	TxEthHash        *string // nil for non-evm-tx events
	EventIdx         int     // Unique event index within the block.
	Type             apiTypes.RuntimeEventType
	Body             EventBody
	WithScope        ScopedSdkEvent
	EvmLogName       *string
	EvmLogSignature  *ethCommon.Hash
	EvmLogParams     []*apiTypes.EvmAbiParam
	RelatedAddresses map[apiTypes.Address]struct{}
}

// ScopedSdkEvent is a one-of container for SDK events.
type ScopedSdkEvent struct {
	Core              *core.Event
	Accounts          *accounts.Event
	ConsensusAccounts *consensusaccounts.Event
	EVM               *sdkEVM.Event
	Rofl              *rofl.Event
	RoflMarket        *roflmarket.Event
}

type TokenChangeKey struct {
	// TokenAddress is the Oasis address of the smart contract of the
	// compatible (e.g. ERC-20) token.
	TokenAddress apiTypes.Address
	// AccountAddress is the Oasis address of the owner of some amount of the
	// compatible (e.g. ERC-20) token.
	AccountAddress apiTypes.Address
}

type NFTKey struct {
	TokenAddress apiTypes.Address
	TokenID      *big.Int
}

type PossibleNFT struct {
	// NumTransfers is how many times we saw it transferred. If it's more than
	// zero, Burned or NewOwner will be set.
	NumTransfers int
	// Burned is true if NumTransfers is more than zero and the NFT instance
	// was burned.
	Burned bool
	// NewOwner has the latest owner if NumTransfers is more than zero.
	NewOwner apiTypes.Address
}

type SwapCreationKey struct {
	Factory apiTypes.Address
	Token0  apiTypes.Address
	Token1  apiTypes.Address
}

type PossibleSwapCreation struct {
	Pair apiTypes.Address
}

type PossibleSwapSync struct {
	Reserve0 *big.Int
	Reserve1 *big.Int
}

type BlockData struct {
	Header              nodeapi.RuntimeBlockHeader
	NumTransactions     int // Might be different from len(TransactionData) if some transactions are malformed.
	GasUsed             uint64
	Size                int
	TransactionData     []*BlockTransactionData
	EventData           []*EventData
	AddressPreimages    map[apiTypes.Address]*addresses.PreimageData
	TokenBalanceChanges map[TokenChangeKey]*big.Int
	PossibleTokens      map[apiTypes.Address]*evm.EVMPossibleToken // key is oasis bech32 address
	PossibleNFTs        map[NFTKey]*PossibleNFT
	SwapCreations       map[SwapCreationKey]*PossibleSwapCreation
	SwapSyncs           map[apiTypes.Address]*PossibleSwapSync
}

// Function naming conventions in this file:
// 'extract-' -> dataflow from parameters to return values, no side effects. suitable for processing pieces of data
//   that doesn't affect their siblings
// 'register-' -> dataflow from input parameters to output parameters, side effects. may have dataflow of something
//   useful to return values as well, to entice developers to use these functions instead of e.g. converting an address
//   manually and inadvertently leaving it out of a related address or address preimage map
// 'visit-' -> dataflow from generic parameter to specific callback, no side effects, although callbacks will have side
//   effects. suitable for processing smaller pieces of data that contribute to aggregated structures

func findPossibleNFT(possibleNFTs map[NFTKey]*PossibleNFT, contractAddr apiTypes.Address, tokenID *big.Int) *PossibleNFT {
	key := NFTKey{contractAddr, tokenID}
	possibleNFT, ok := possibleNFTs[key]
	if !ok {
		possibleNFT = &PossibleNFT{}
		possibleNFTs[key] = possibleNFT
	}
	return possibleNFT
}

func registerNFTExist(nftChanges map[NFTKey]*PossibleNFT, contractAddr apiTypes.Address, tokenID *big.Int) {
	findPossibleNFT(nftChanges, contractAddr, tokenID)
}

func registerNFTTransfer(nftChanges map[NFTKey]*PossibleNFT, contractAddr apiTypes.Address, tokenID *big.Int, burned bool, newOwner apiTypes.Address) {
	possibleNFT := findPossibleNFT(nftChanges, contractAddr, tokenID)
	possibleNFT.NumTransfers++
	possibleNFT.Burned = burned
	possibleNFT.NewOwner = newOwner
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

func registerPaymentAddress(paymentAddress *roflmarket.PaymentAddress, blockTransactionData *BlockTransactionData, blockData *BlockData) error {
	if paymentAddress.Native != nil {
		native, err := addresses.FromSdkAddress(paymentAddress.Native)
		if err != nil {
			return err
		}
		blockTransactionData.RelatedAccountAddresses[native] = struct{}{}
	}
	if paymentAddress.Eth != nil {
		eth, err := addresses.RegisterEthAddress(blockData.AddressPreimages, paymentAddress.Eth[:])
		if err != nil {
			return err
		}
		blockTransactionData.RelatedAccountAddresses[eth] = struct{}{}
	}
	return nil
}

func ExtractRound(blockHeader nodeapi.RuntimeBlockHeader, txrs []nodeapi.RuntimeTransactionWithResults, rawEvents []nodeapi.RuntimeEvent, sdkPT *sdkConfig.ParaTime, logger *log.Logger) (*BlockData, error) { //nolint:gocyclo
	blockData := BlockData{
		Header:              blockHeader,
		NumTransactions:     len(txrs),
		TransactionData:     make([]*BlockTransactionData, 0, len(txrs)),
		EventData:           []*EventData{},
		AddressPreimages:    map[apiTypes.Address]*addresses.PreimageData{},
		TokenBalanceChanges: map[TokenChangeKey]*big.Int{},
		PossibleTokens:      map[apiTypes.Address]*evm.EVMPossibleToken{},
		PossibleNFTs:        map[NFTKey]*PossibleNFT{},
		SwapCreations:       map[SwapCreationKey]*PossibleSwapCreation{},
		SwapSyncs:           map[apiTypes.Address]*PossibleSwapSync{},
	}

	extractedEvents, err := extractEvents(&blockData, rawEvents)
	if err != nil {
		return nil, fmt.Errorf("extract non-tx events: %w", err)
	}
	for _, event := range extractedEvents {
		if event.TxHash == nil {
			blockData.EventData = append(blockData.EventData, event)
		}
	}

	// Extract info from transactions.
	for txIndex, txr := range txrs {
		var blockTransactionData BlockTransactionData
		blockTransactionData.Index = txIndex
		blockTransactionData.Hash = txr.Tx.Hash().Hex()
		if len(txr.Tx.AuthProofs) == 1 && txr.Tx.AuthProofs[0].Module == "evm.ethereum.v0" {
			ethHash := hex.EncodeToString(eth.Keccak256(txr.Tx.Body))
			blockTransactionData.EthHash = &ethHash
		}
		blockTransactionData.Raw = cbor.Marshal(txr.Tx)
		// Inaccurate: Re-serialize signed tx to estimate original size.
		blockTransactionData.Size = len(blockTransactionData.Raw)
		blockTransactionData.RawResult = cbor.Marshal(txr.Result)
		blockTransactionData.RelatedAccountAddresses = map[apiTypes.Address]struct{}{}
		blockTransactionData.RelatedRoflAddresses = map[nodeapi.AppID]struct{}{}
		var isRoflCreate bool
		tx, err := uncategorized.OpenUtxNoVerify(&txr.Tx)
		if err != nil {
			logger.Error("error decoding tx, skipping tx-specific analysis",
				"round", blockHeader.Round,
				"tx_index", txIndex,
				"tx_hash", txr.Tx.Hash(),
				"tx_body_cbor", hex.EncodeToString(txr.Tx.Body),
				"err", err,
			)
			tx = nil
		}
		if tx != nil { //nolint:nestif
			blockTransactionData.SignerData = make([]*BlockTransactionSignerData, 0, len(tx.AuthInfo.SignerInfo))
			for j, si := range tx.AuthInfo.SignerInfo {
				si := si // we have no dangerous uses of &si, but capture the variable just in case (and to make the linter happy)
				var blockTransactionSignerData BlockTransactionSignerData
				blockTransactionSignerData.Index = j
				addr, err1 := addresses.RegisterAddressSpec(blockData.AddressPreimages, &si.AddressSpec)
				if err1 != nil {
					return nil, fmt.Errorf("tx %d signer %d visit address spec: %w", txIndex, j, err1)
				}
				blockTransactionData.RelatedAccountAddresses[addr] = struct{}{}
				blockTransactionSignerData.Address = addr
				blockTransactionSignerData.Nonce = int(si.Nonce)
				blockTransactionData.SignerData = append(blockTransactionData.SignerData, &blockTransactionSignerData)
			}
			blockTransactionData.Fee = common.BigIntFromQuantity(tx.AuthInfo.Fee.Amount.Amount)
			blockTransactionData.FeeSymbol = stringifyDenomination(sdkPT, tx.AuthInfo.Fee.Amount.Denomination)
			if tx.AuthInfo.Fee.Proxy != nil {
				blockTransactionData.FeeProxyModule = &tx.AuthInfo.Fee.Proxy.Module
				blockTransactionData.FeeProxyID = common.Ptr(tx.AuthInfo.Fee.Proxy.ID)
			}
			blockTransactionData.GasLimit = tx.AuthInfo.Fee.Gas

			// Parse the success/error status.
			if fail := txr.Result.Failed; fail != nil {
				txErr := extractTxError(*fail)
				blockTransactionData.Error = &txErr
				blockTransactionData.Success = common.Ptr(false)
			} else if txr.Result.Ok != nil {
				blockTransactionData.Success = common.Ptr(true)
			} else {
				blockTransactionData.Success = nil
			}

			if oasisEncrypted, err1 := OasisMaybeUnmarshalEncryptedData(&tx.Call, &txr.Result); err1 == nil {
				blockTransactionData.OasisEncrypted = oasisEncrypted
			} else {
				logger.Error("error unmarshalling encrypted transaction and result, omitting encrypted fields",
					"round", blockHeader.Round,
					"tx_index", txIndex,
					"tx_hash", txr.Tx.Hash(),
					"err", err1,
				)
			}

			blockTransactionData.Method = string(tx.Call.Method)
			var to apiTypes.Address
			var amount quantity.Quantity
			if err = VisitCall(&tx.Call, &txr.Result, &CallHandler{
				AccountsTransfer: func(body *accounts.Transfer) error {
					blockTransactionData.IsLikelyTokenTransfer = true
					blockTransactionData.Body = body
					amount = body.Amount.Amount
					blockTransactionData.AmountSymbol = common.Ptr(stringifyDenomination(sdkPT, body.Amount.Denomination))
					if to, err = addresses.FromSdkAddress(&body.To); err != nil {
						return fmt.Errorf("to: %w", err)
					}
					blockTransactionData.RelatedAccountAddresses[to] = struct{}{}
					return nil
				},
				ConsensusAccountsDeposit: func(body *consensusaccounts.Deposit) error {
					blockTransactionData.Body = body
					amount = body.Amount.Amount
					blockTransactionData.AmountSymbol = common.Ptr(stringifyDenomination(sdkPT, body.Amount.Denomination))
					if body.To != nil {
						if to, err = addresses.FromSdkAddress(body.To); err != nil {
							return fmt.Errorf("to: %w", err)
						}
					} else {
						// A missing `body.To` implies that deposited-to runtime address is the same as the sender, i.e. deposited-from address.
						// (The sender is technically also a runtime address because Deposit is a runtime tx, but the runtime verifies that the address also corresponds to a valid consensus account.)
						// Ref: https://github.com/oasisprotocol/oasis-sdk/blob/runtime-sdk/v0.8.4/runtime-sdk/src/modules/consensus_accounts/mod.rs#L418
						to = blockTransactionData.SignerData[0].Address
					}
					// Set the 'Success' field to 'Pending' for deposits. This is because the outcome of the Deposit tx is only known in the next block.
					blockTransactionData.Success = nil
					blockTransactionData.RelatedAccountAddresses[to] = struct{}{}
					return nil
				},
				ConsensusAccountsWithdraw: func(body *consensusaccounts.Withdraw) error {
					blockTransactionData.Body = body
					amount = body.Amount.Amount
					blockTransactionData.AmountSymbol = common.Ptr(stringifyDenomination(sdkPT, body.Amount.Denomination))
					if body.To != nil {
						// This is the address of an account in the consensus layer only; we do not register it as a preimage.
						if to, err = addresses.FromSdkAddress(body.To); err != nil {
							return fmt.Errorf("to: %w", err)
						}
					} else {
						// A missing `body.To` implies that the withdrawn-to consensus address is the same as the withdrawn-from runtime address.
						// Ref: https://github.com/oasisprotocol/oasis-sdk/blob/runtime-sdk/v0.8.4/runtime-sdk/src/modules/consensus_accounts/mod.rs#L462
						to = blockTransactionData.SignerData[0].Address
					}
					blockTransactionData.RelatedAccountAddresses[to] = struct{}{}
					// Set the 'Success' field to 'Pending' for withdrawals. This is because the outcome of the Withdraw tx is only known in the next block.
					blockTransactionData.Success = nil

					return nil
				},
				ConsensusAccountsDelegate: func(body *consensusaccounts.Delegate) error {
					// LESSON: What (un)delegations look like on the chain.
					//
					// Example from Sapphire Testnet:
					// Round 2378822:
					//   - tx Delegate(sender: oasis1...nz2f, to: oasis1...8tha)
					//       Runtime account wants to delegate some funds. Here, nz2f is a runtime address, 8tha is a validator's consensus address.
					//   - event Transfer(from: nz2f, to: q49r)
					//       q49r is the special `pending-delegation` system address in the runtime; each runtime has it.
					//       The reason for this temporary transfer is that delegations (and other consensus-related stuff) are async, which means that
					//       whether a delegation succeeded can only be known in the following round. So we need to prevent the user from moving the
					//       tokens after delegating them, which is why we lock the tokens by moving them into the pending delegation address until
					//       the result is known (in the next block). Then they are either returned (if delegation failed) or burned (if delegation succeeded).
					// Round 2378823 (= next round):
					//   - event Delegate(from: nz2f, to: 8tha)
					//   - event Burn(owner: q49r)
					//       The runtime has learned (via a Message, a consensus->runtime communication mechanism) that the delegation succeeded at the
					//       consensus layer, so it burns the tokens inside the runtime, as discussed.
					// Round 2379853 (triggered by user action):
					//   - event UndelegateStart(from: 8tha, to: nz2f)
					// Round 2534792 (= after debonding period):
					//   - event UndelegateDone(from: 8tha, to: nz2f)
					//   - event Mint(to: nz2f)
					blockTransactionData.Body = body
					amount = body.Amount.Amount
					blockTransactionData.AmountSymbol = common.Ptr(stringifyDenomination(sdkPT, body.Amount.Denomination))
					// This is the address of an account in the consensus layer only; we do not register it as a preimage.
					if to, err = addresses.FromSdkAddress(&body.To); err != nil {
						return fmt.Errorf("to: %w", err)
					}
					blockTransactionData.RelatedAccountAddresses[to] = struct{}{}
					// Set the 'Success' field to 'Pending' for delegations. This is because the outcome of the Delegate tx is only known in the next block.
					blockTransactionData.Success = nil
					return nil
				},
				ConsensusAccountsUndelegate: func(body *consensusaccounts.Undelegate) error {
					blockTransactionData.Body = body
					// NOTE: The `from` and `to` addresses have swapped semantics compared to most other txs:
					// Assume R is a runtime address and C is a consensus address (likely a validator). The inverse of Delegate(from=R, to=C) is Undelegate(from=C, to=R).
					// In Undelegate semantics, the inexistent `body.To` is implicitly the account that created this tx, i.e. the delegator R.
					// Ref: https://github.com/oasisprotocol/oasis-sdk/blob/eb97a8162f84ae81d11d805e6dceeeb016841c27/runtime-sdk/src/modules/consensus_accounts/mod.rs#L465-L465
					// However, we instead expose `body.From` as the DB/API `to` for consistency with `Delegate`, and because it is more useful: the delegator R is already indexed in the tx sender field.
					if to, err = addresses.FromSdkAddress(&body.From); err != nil {
						return fmt.Errorf("from: %w", err)
					}
					blockTransactionData.RelatedAccountAddresses[to] = struct{}{}
					// The `amount` (of tokens) is not contained in the body, only `shares` is. There isn't sufficient information
					// to convert `shares` to `amount` until the undelegation actually happens (= UndelegateDone event); in the meantime,
					// the validator's token pool might change, e.g. because of slashing.
					// Do not store `body.Shares` in DB's `amount` to avoid confusion. Clients can still look up the shares in the tx body if they really need it.

					// Set the 'Success' field to 'Pending' for undelegations. This is because the outcome of the Undelegate tx is only known in the next block.
					blockTransactionData.Success = nil
					return nil
				},
				EVMCreate: func(body *sdkEVM.Create, ok *[]byte) error {
					blockTransactionData.Body = body
					amount = uncategorized.QuantityFromBytes(body.Value)

					if !txr.Result.IsUnknown() && txr.Result.IsSuccess() && len(*ok) == 20 {
						// Decode address of newly-created contract
						// todo: is this rigorous enough?
						if to, err = addresses.RegisterEthAddress(blockData.AddressPreimages, *ok); err != nil {
							return fmt.Errorf("created contract: %w", err)
						}
						blockTransactionData.RelatedAccountAddresses[to] = struct{}{}
						blockTransactionData.EVMContract = &evm.EVMContractData{
							Address:          to,
							CreationBytecode: body.InitCode,
							CreationTx:       blockTransactionData.Hash,
						}

						// The `to` address is a contract; enqueue it for analysis.
						blockTransactionData.ContractCandidate = &to

						// Mark sender and contract accounts as having potentially stale balances.
						// EVMCreate can transfer funds from the sender to the contract.
						if to != "" {
							registerTokenIncrease(blockData.TokenBalanceChanges, evm.NativeRuntimeTokenAddress, to, big.NewInt(0))
						}
						for _, signer := range blockTransactionData.SignerData {
							registerTokenDecrease(blockData.TokenBalanceChanges, evm.NativeRuntimeTokenAddress, signer.Address, big.NewInt(0))
						}
					}

					// Handle encrypted txs.
					// We don't pass the tx result (`ok`) to EVMMaybeUnmarshalEncryptedData because it's the
					// (unencrypted) address of the created contract. The function expects a CBOR-encoded
					// encryption envelope as its second argument, so passing the unencrypted address
					// makes it incorrectly declare the whole tx unencrypted.
					// Note: The address of the created contract is tracked in blockTransactionData.To.
					if evmEncrypted, _, err2 := evm.EVMMaybeUnmarshalEncryptedData(body.InitCode, nil); err2 == nil {
						blockTransactionData.EVMEncrypted = evmEncrypted
					} else {
						logger.Error("error unmarshalling encrypted init code and result, omitting encrypted fields",
							"round", blockHeader.Round,
							"tx_index", txIndex,
							"tx_hash", txr.Tx.Hash(),
							"err", err2,
						)
					}
					return nil
				},
				EVMCall: func(body *sdkEVM.Call, ok *[]byte) error {
					blockTransactionData.IsLikelyTokenTransfer = len(body.Data) == 0 // Calls with empty body are likely native token transfers.
					blockTransactionData.Body = body
					amount = uncategorized.QuantityFromBytes(body.Value)
					if to, err = addresses.RegisterEthAddress(blockData.AddressPreimages, body.Address); err != nil {
						return fmt.Errorf("address: %w", err)
					}
					blockTransactionData.RelatedAccountAddresses[to] = struct{}{}
					if evmEncrypted, failedCallResult, err2 := evm.EVMMaybeUnmarshalEncryptedData(body.Data, ok); err2 == nil {
						blockTransactionData.EVMEncrypted = evmEncrypted
						// For non-evm txs as well as older Sapphire txs, the outer CallResult may
						// be unknown and the inner callResult Failed. In this case, we extract the
						// error fields.
						if failedCallResult != nil {
							txErr := extractTxError(*failedCallResult)
							blockTransactionData.Error = &txErr
							blockTransactionData.Success = common.Ptr(false)
						}
					} else {
						logger.Error("error unmarshalling encrypted data and result, omitting encrypted fields",
							"round", blockHeader.Round,
							"tx_index", txIndex,
							"tx_hash", txr.Tx.Hash(),
							"err", err2,
						)
					}

					// Any recipient of a call might be a contract.
					blockTransactionData.ContractCandidate = &to

					if txr.Result.Ok != nil {
						// Dead-reckon native token balances.
						// Native token transfers do not generate events. Theoretically, any call can change the balance of any account,
						// and we do not have a good way of tracking them; we just query them with the evm_token_balances analyzer.
						// But heuristically, a call is most likely to change the balances of the sender and the receiver, so we create
						// a (quite possibly incorrect) dead-reckoned change of 0 for those accounts, which will cause the evm_token_balances analyzer
						// to re-query their real balance.
						reckonedAmount := amount.ToBigInt() // Calls with an empty body represent a transfer of the native token.
						if len(body.Data) != 0 || len(blockTransactionData.SignerData) > 1 {
							// Calls with a non-empty body have no standard impact on native balance. Better to dead-reckon a 0 change (and keep stale balances)
							// than to reckon a wrong change (and have a "random" incorrect balance until it is re-queried).
							reckonedAmount = big.NewInt(0)
						}
						registerTokenIncrease(blockData.TokenBalanceChanges, evm.NativeRuntimeTokenAddress, to, reckonedAmount)
						for _, signer := range blockTransactionData.SignerData {
							registerTokenDecrease(blockData.TokenBalanceChanges, evm.NativeRuntimeTokenAddress, signer.Address, reckonedAmount)
						}
					}

					// TODO: maybe parse known token methods (ERC-20 etc)
					return nil
				},
				RoflCreate: func(body *rofl.Create) error {
					blockTransactionData.Body = body
					// blockTransactionData.RelatedRoflAddresses[] = struct{}{} // We don't have the ID yet here, we need to get it from the event.
					isRoflCreate = true
					return nil
				},
				RoflUpdate: func(body *rofl.Update) error {
					blockTransactionData.Body = body
					blockTransactionData.RelatedRoflAddresses[body.ID] = struct{}{}
					admin, err := addresses.FromSdkAddress(body.Admin)
					if err != nil {
						logger.Warn("failed to convert admin address to native address", "err", err)
						return nil
					}
					blockTransactionData.RelatedAccountAddresses[admin] = struct{}{}
					return nil
				},
				RoflRemove: func(body *rofl.Remove) error {
					blockTransactionData.Body = body
					blockTransactionData.RelatedRoflAddresses[body.ID] = struct{}{}
					return nil
				},
				RoflRegister: func(body *rofl.Register) error {
					blockTransactionData.Body = body
					blockTransactionData.RelatedRoflAddresses[body.App] = struct{}{}
					return nil
				},
				RoflMarketProviderCreate: func(body *roflmarket.ProviderCreate) error {
					blockTransactionData.Body = body
					blockTransactionData.RelatedRoflAddresses[body.SchedulerApp] = struct{}{}
					if err := registerPaymentAddress(&body.PaymentAddress, &blockTransactionData, &blockData); err != nil {
						logger.Warn("failed to register payment address", "err", err)
					}
					return nil
				},
				RoflMarketProviderUpdate: func(body *roflmarket.ProviderUpdate) error {
					blockTransactionData.Body = body
					blockTransactionData.RelatedRoflAddresses[body.SchedulerApp] = struct{}{}
					if err := registerPaymentAddress(&body.PaymentAddress, &blockTransactionData, &blockData); err != nil {
						logger.Warn("failed to register payment address", "err", err)
					}
					provider, err := addresses.FromSdkAddress(&body.Provider)
					if err != nil {
						logger.Warn("failed to convert provider address to native address", "err", err)
						return nil
					}
					blockTransactionData.RelatedAccountAddresses[provider] = struct{}{}
					return nil
				},
				RoflMarketProviderUpdateOffers: func(body *roflmarket.ProviderUpdateOffers) error {
					blockTransactionData.Body = body
					provider, err := addresses.FromSdkAddress(&body.Provider)
					if err != nil {
						logger.Warn("failed to convert provider address to native address", "err", err)
					}
					blockTransactionData.RelatedAccountAddresses[provider] = struct{}{}
					return nil
				},
				RoflMarketProviderRemove: func(body *roflmarket.ProviderRemove) error {
					blockTransactionData.Body = body
					provider, err := addresses.FromSdkAddress(&body.Provider)
					if err != nil {
						logger.Warn("failed to convert provider address to native address", "err", err)
					}
					blockTransactionData.RelatedAccountAddresses[provider] = struct{}{}
					return nil
				},
				RoflMarketInstanceCreate: func(body *roflmarket.InstanceCreate) error {
					blockTransactionData.Body = body
					provider, err := addresses.FromSdkAddress(&body.Provider)
					if err != nil {
						logger.Warn("failed to convert provider address to native address", "err", err)
					}
					blockTransactionData.RelatedAccountAddresses[provider] = struct{}{}

					if body.Admin != nil {
						admin, err := addresses.FromSdkAddress(body.Admin)
						if err != nil {
							logger.Warn("failed to convert admin address to native address", "err", err)
						} else {
							blockTransactionData.RelatedAccountAddresses[admin] = struct{}{}
						}
					}

					if body.Deployment != nil {
						blockTransactionData.RelatedRoflAddresses[body.Deployment.AppID] = struct{}{}
					}
					return nil
				},
				RoflMarketInstanceTopUp: func(body *roflmarket.InstanceTopUp) error {
					blockTransactionData.Body = body
					provider, err := addresses.FromSdkAddress(&body.Provider)
					if err != nil {
						logger.Warn("failed to convert provider address to native address", "err", err)
					}
					blockTransactionData.RelatedAccountAddresses[provider] = struct{}{}
					return nil
				},
				RoflMarketInstanceCancel: func(body *roflmarket.InstanceCancel) error {
					blockTransactionData.Body = body
					provider, err := addresses.FromSdkAddress(&body.Provider)
					if err != nil {
						logger.Warn("failed to convert provider address to native address", "err", err)
					}
					blockTransactionData.RelatedAccountAddresses[provider] = struct{}{}
					return nil
				},
				RoflMarketInstanceExecuteCmds: func(body *roflmarket.InstanceExecuteCmds) error {
					blockTransactionData.Body = body
					provider, err := addresses.FromSdkAddress(&body.Provider)
					if err != nil {
						logger.Warn("failed to convert provider address to native address", "err", err)
					}
					blockTransactionData.RelatedAccountAddresses[provider] = struct{}{}
					return nil
				},
				UnknownMethod: func(methodName string) error {
					logger.Warn("unknown tx method, skipping tx-specific analysis", "tx_method", methodName)
					return nil
				},
			}); err != nil {
				return nil, fmt.Errorf("tx %d: %w", txIndex, err)
			}
			if to != "" {
				blockTransactionData.To = &to
			}
			blockTransactionData.Amount = common.Ptr(common.BigIntFromQuantity(amount))
		}

		// Find extracted events for this tx.
		var extractedTxEvents []*EventData
		for _, event := range extractedEvents {
			if event.TxHash != nil && *event.TxHash == txr.Tx.Hash().String() {
				extractedTxEvents = append(extractedTxEvents, event)
				// Register related addresses found in the event for the transaction as well.
				for addr := range event.RelatedAddresses {
					blockTransactionData.RelatedAccountAddresses[addr] = struct{}{}
				}

				// For rofl.Create we need to get the App ID from the event so that we
				// can mark the transaction related ROFL address.
				if isRoflCreate {
					for _, event := range extractedEvents {
						if event.WithScope.Rofl != nil && event.WithScope.Rofl.AppCreated != nil {
							blockTransactionData.RelatedRoflAddresses[event.WithScope.Rofl.AppCreated.ID] = struct{}{}
						}
					}
				}
			}
		}
		txGasUsed, foundGasUsedEvent := sumGasUsed(extractedTxEvents)
		// Populate eventData with tx-specific data.
		for _, eventData := range extractedTxEvents {
			txIndex := txIndex // const local copy of loop variable
			eventData.TxIndex = &txIndex
			eventData.TxHash = &blockTransactionData.Hash
			eventData.TxEthHash = blockTransactionData.EthHash
		}
		if !foundGasUsedEvent {
			// Early versions of runtimes didn't emit a GasUsed event.
			if (txr.Result.IsUnknown() || txr.Result.IsSuccess()) && tx != nil {
				// Treat as if it used all the gas.
				logger.Debug("tx didn't emit a core.GasUsed event, assuming it used max allowed gas", "tx_hash", txr.Tx.Hash(), "assumed_gas_used", tx.AuthInfo.Fee.Gas)
				txGasUsed = tx.AuthInfo.Fee.Gas
			} else {
				// Very rough heuristic: Treat as not using any gas.
				//
				// It's probably closer to truth to guess that all gas was used, unless
				// there was an auth or insufficient-funds error, but a very simple
				// heuristic is nice in its own right; it's easy to explain.
				//
				// Beware that some failed txs have an enormous (e.g. MAX_INT64) gas
				// limit.
				logger.Debug("tx didn't emit a core.GasUsed event and failed, assuming it used no gas", "tx_hash", txr.Tx.Hash(), "assumed_gas_used", 0)
				txGasUsed = 0
			}
		}
		blockTransactionData.GasUsed = txGasUsed
		blockData.TransactionData = append(blockData.TransactionData, &blockTransactionData)
		blockData.EventData = append(blockData.EventData, extractedTxEvents...)
		// If this overflows, it will do so silently. However, supported
		// runtimes internally use u64 checked math to impose a batch gas,
		// which will prevent it from emitting blocks that use enough gas to
		// do that.
		blockData.GasUsed += txGasUsed
		blockData.Size += blockTransactionData.Size
	}

	return &blockData, nil
}

func sumGasUsed(events []*EventData) (sum uint64, foundGasUsedEvent bool) {
	foundGasUsedEvent = false
	for _, event := range events {
		if event.WithScope.Core != nil && event.WithScope.Core.GasUsed != nil {
			foundGasUsedEvent = true
			sum += event.WithScope.Core.GasUsed.Amount
		}
	}
	return
}

func extractTxError(fcr sdkTypes.FailedCallResult) TxError {
	txErr := &TxError{
		Code:   fcr.Code,
		Module: fcr.Module,
	}
	if len(fcr.Message) > 0 {
		// Store raw error message.
		sanitizedRawMsg := storage.SanitizeString(fcr.Message)
		txErr.RawMessage = &sanitizedRawMsg
		// Store parsed error message, if possible.
		txErr.Message = tryParseErrorMessage(fcr.Module, fcr.Code, fcr.Message)
	}

	return *txErr
}

// Attempts to extract the human-readable error message from
// the raw error returned by the node.
//
// Transactions can error for many reasons, and in most cases
// will return a plaintext error message such as
// - "execution failed: out of fund"
// - "withdraw: insufficient runtime balance"
//
// Transactions that were reverted by the EVM return a distinct class
// of transaction revert errors. In older Emerald and Sapphire
// versions (prior to Sapphire 0.6.3, roughly Q4 2023), transaction
// revert reasons were decoded by the runtime and returned as
// human-readable strings, e.g.
// - "reverted: Incorrect premium amount."
//
// More recent versions of Emerald and Sapphire no longer decode the
// revert reason for transaction revert errors and instead use the
// following error format:
// - "reverted: " || base64(abiEncode(error))
//
// We first check to see if the error message is a plaintext error string,
// in which case we simply sanitize and return the message.
// Otherwise, we optimistically try to decode the error as the prevailing
// Error(string) type and return the error message if successful.
//
// If no revert reason was provided, e.g. the raw error message is
// "reverted: ", we return a fixed default error message instead.
//
// Note that the error type can be any data type specified in the abi.
// We may not have the abi available now, so in those cases the
// abi analyzer will extract the error message once the contract
// abi is available.
func tryParseErrorMessage(errorModule string, errorCode uint32, msg string) *string {
	// Transaction revert errors specifically have an errorModule of `evm`
	// and errorCode of `8`. If the message is not from a transaction revert
	// error, the message must be plaintext, which we sanitize and return.
	if errorModule != sdkEVM.ModuleName || errorCode != 8 {
		sanitizedMsg := storage.SanitizeString(msg)
		return &sanitizedMsg
	}
	// Try to decode the revert reason as:
	// "reverted: " || base64(abiEncode(error))
	abiEncodedErr, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(msg, TxRevertErrPrefix))
	if err != nil {
		// An old-style plaintext error message.
		//
		// Note: This is an imperfect heuristic, some older error messages may
		// be valid b64 encodings and slip through. For newer errors, the runtime
		// guarantees that the message is base64-encoded.
		sanitizedMsg := storage.SanitizeString(msg)
		return &sanitizedMsg
	}
	// Return a default error message if no revert reason was provided.
	if len(abiEncodedErr) == 0 {
		return common.Ptr(DefaultTxRevertErrMsg)
	}
	// Try to abi decode as Error(string).
	stringAbi, _ := abi.NewType("string", "", nil)
	errAbi := abi.NewError("Error", abi.Arguments{{Type: stringAbi}})
	unpacked, err := errAbi.Unpack(abiEncodedErr)
	if err != nil {
		// Likely a custom error type that we need the abi to parse.
		return nil
	}
	errMsg := unpacked.([]interface{})[0].(string)
	sanitizedMsg := TxRevertErrPrefix + storage.SanitizeString(errMsg)
	return &sanitizedMsg
}

func extractEvents(blockData *BlockData, eventsRaw []nodeapi.RuntimeEvent) ([]*EventData, error) { //nolint:gocyclo
	extractedEvents := []*EventData{}
	if err := VisitSdkEvents(eventsRaw, &SdkEventHandler{
		Core: func(event *core.Event, eventTxHash *string, eventIdx int) error {
			if event.GasUsed != nil {
				eventData := EventData{
					EventIdx:  eventIdx,
					TxHash:    eventTxHash,
					Type:      apiTypes.RuntimeEventTypeCoreGasUsed,
					Body:      event.GasUsed,
					WithScope: ScopedSdkEvent{Core: event},
				}
				extractedEvents = append(extractedEvents, &eventData)
			}
			return nil
		},
		Accounts: func(event *accounts.Event, eventTxHash *string, eventIdx int) error {
			if event.Transfer != nil {
				fromAddr, err1 := addresses.FromSdkAddress(&event.Transfer.From)
				if err1 != nil {
					return fmt.Errorf("from: %w", err1)
				}
				toAddr, err1 := addresses.FromSdkAddress(&event.Transfer.To)
				if err1 != nil {
					return fmt.Errorf("to: %w", err1)
				}
				eventData := EventData{
					EventIdx:         eventIdx,
					TxHash:           eventTxHash,
					Type:             apiTypes.RuntimeEventTypeAccountsTransfer,
					Body:             event.Transfer,
					WithScope:        ScopedSdkEvent{Accounts: event},
					RelatedAddresses: map[apiTypes.Address]struct{}{fromAddr: {}, toAddr: {}},
				}
				extractedEvents = append(extractedEvents, &eventData)
			}
			if event.Burn != nil {
				ownerAddr, err1 := addresses.FromSdkAddress(&event.Burn.Owner)
				if err1 != nil {
					return fmt.Errorf("owner: %w", err1)
				}
				eventData := EventData{
					EventIdx:         eventIdx,
					TxHash:           eventTxHash,
					Type:             apiTypes.RuntimeEventTypeAccountsBurn,
					Body:             event.Burn,
					WithScope:        ScopedSdkEvent{Accounts: event},
					RelatedAddresses: map[apiTypes.Address]struct{}{ownerAddr: {}},
				}
				extractedEvents = append(extractedEvents, &eventData)
			}
			if event.Mint != nil {
				ownerAddr, err1 := addresses.FromSdkAddress(&event.Mint.Owner)
				if err1 != nil {
					return fmt.Errorf("owner: %w", err1)
				}
				eventData := EventData{
					EventIdx:         eventIdx,
					TxHash:           eventTxHash,
					Type:             apiTypes.RuntimeEventTypeAccountsMint,
					Body:             event.Mint,
					WithScope:        ScopedSdkEvent{Accounts: event},
					RelatedAddresses: map[apiTypes.Address]struct{}{ownerAddr: {}},
				}
				extractedEvents = append(extractedEvents, &eventData)
			}
			return nil
		},
		ConsensusAccounts: func(event *consensusaccounts.Event, eventTxHash *string, eventIdx int) error {
			if event.Deposit != nil {
				// NOTE: .From is a _consensus_ addr (not runtime). It's still related though.
				fromAddr, err1 := addresses.FromSdkAddress(&event.Deposit.From)
				if err1 != nil {
					return fmt.Errorf("from: %w", err1)
				}
				toAddr, err1 := addresses.FromSdkAddress(&event.Deposit.To)
				if err1 != nil {
					return fmt.Errorf("to: %w", err1)
				}
				eventData := EventData{
					EventIdx:         eventIdx,
					TxHash:           eventTxHash,
					Type:             apiTypes.RuntimeEventTypeConsensusAccountsDeposit,
					Body:             event.Deposit,
					WithScope:        ScopedSdkEvent{ConsensusAccounts: event},
					RelatedAddresses: map[apiTypes.Address]struct{}{fromAddr: {}, toAddr: {}},
				}
				extractedEvents = append(extractedEvents, &eventData)
			}
			if event.Withdraw != nil {
				fromAddr, err1 := addresses.FromSdkAddress(&event.Withdraw.From)
				if err1 != nil {
					return fmt.Errorf("from: %w", err1)
				}
				// NOTE: .To is a _consensus_ addr (not runtime). It's still related though.
				toAddr, err1 := addresses.FromSdkAddress(&event.Withdraw.To)
				if err1 != nil {
					return fmt.Errorf("to: %w", err1)
				}
				eventData := EventData{
					EventIdx:         eventIdx,
					TxHash:           eventTxHash,
					Type:             apiTypes.RuntimeEventTypeConsensusAccountsWithdraw,
					Body:             event.Withdraw,
					WithScope:        ScopedSdkEvent{ConsensusAccounts: event},
					RelatedAddresses: map[apiTypes.Address]struct{}{fromAddr: {}, toAddr: {}},
				}
				extractedEvents = append(extractedEvents, &eventData)
			}
			if event.Delegate != nil {
				// No dead reckoning needed; balance changes are signalled by other, co-emitted events.
				// See "LESSON" comment in the code that handles the Delegate tx.
				fromAddr, err1 := addresses.FromSdkAddress(&event.Delegate.From)
				if err1 != nil {
					return fmt.Errorf("from: %w", err1)
				}
				toAddr, err1 := addresses.FromSdkAddress(&event.Delegate.To)
				if err1 != nil {
					return fmt.Errorf("to: %w", err1)
				}
				eventData := EventData{
					EventIdx:         eventIdx,
					TxHash:           eventTxHash,
					Type:             apiTypes.RuntimeEventTypeConsensusAccountsDelegate,
					Body:             event.Delegate,
					WithScope:        ScopedSdkEvent{ConsensusAccounts: event},
					RelatedAddresses: map[apiTypes.Address]struct{}{fromAddr: {}, toAddr: {}},
				}
				extractedEvents = append(extractedEvents, &eventData)
			}
			if event.UndelegateStart != nil {
				fromAddr, err1 := addresses.FromSdkAddress(&event.UndelegateStart.From)
				if err1 != nil {
					return fmt.Errorf("from: %w", err1)
				}
				toAddr, err1 := addresses.FromSdkAddress(&event.UndelegateStart.To)
				if err1 != nil {
					return fmt.Errorf("to: %w", err1)
				}
				eventData := EventData{
					EventIdx:         eventIdx,
					TxHash:           eventTxHash,
					Type:             apiTypes.RuntimeEventTypeConsensusAccountsUndelegateStart,
					Body:             event.UndelegateStart,
					WithScope:        ScopedSdkEvent{ConsensusAccounts: event},
					RelatedAddresses: map[apiTypes.Address]struct{}{fromAddr: {}, toAddr: {}},
					// We cannot set EvmLogSignature here because topics[0] is not the log signature for anonymous events.
				}
				extractedEvents = append(extractedEvents, &eventData)
			}
			if event.UndelegateDone != nil {
				fromAddr, err1 := addresses.FromSdkAddress(&event.UndelegateDone.From)
				if err1 != nil {
					return fmt.Errorf("from: %w", err1)
				}
				toAddr, err1 := addresses.FromSdkAddress(&event.UndelegateDone.To)
				if err1 != nil {
					return fmt.Errorf("to: %w", err1)
				}
				eventData := EventData{
					EventIdx:         eventIdx,
					TxHash:           eventTxHash,
					Type:             apiTypes.RuntimeEventTypeConsensusAccountsUndelegateDone,
					Body:             event.UndelegateDone,
					WithScope:        ScopedSdkEvent{ConsensusAccounts: event},
					RelatedAddresses: map[apiTypes.Address]struct{}{fromAddr: {}, toAddr: {}},
				}
				extractedEvents = append(extractedEvents, &eventData)
			}
			return nil
		},
		EVM: func(event *sdkEVM.Event, eventTxHash *string, eventIdx int) error {
			eventAddr, err1 := addresses.RegisterEthAddress(blockData.AddressPreimages, event.Address)
			if err1 != nil {
				return fmt.Errorf("event address: %w", err1)
			}
			eventData := EventData{
				EventIdx:         eventIdx,
				TxHash:           eventTxHash,
				Type:             apiTypes.RuntimeEventTypeEvmLog,
				Body:             event,
				WithScope:        ScopedSdkEvent{EVM: event},
				RelatedAddresses: map[apiTypes.Address]struct{}{eventAddr: {}},
			}
			if err1 = VisitEVMEvent(event, &EVMEventHandler{
				ERC20Transfer: func(fromECAddr ethCommon.Address, toECAddr ethCommon.Address, value *big.Int) error {
					fromZero := bytes.Equal(fromECAddr.Bytes(), eth.ZeroEthAddr)
					toZero := bytes.Equal(toECAddr.Bytes(), eth.ZeroEthAddr)
					if !fromZero {
						fromAddr, err2 := addresses.RegisterEthAddress(blockData.AddressPreimages, fromECAddr.Bytes())
						if err2 != nil {
							return fmt.Errorf("from: %w", err2)
						}
						eventData.RelatedAddresses[fromAddr] = struct{}{}
						registerTokenDecrease(blockData.TokenBalanceChanges, eventAddr, fromAddr, value)
					}
					if !toZero {
						toAddr, err2 := addresses.RegisterEthAddress(blockData.AddressPreimages, toECAddr.Bytes())
						if err2 != nil {
							return fmt.Errorf("to: %w", err2)
						}
						eventData.RelatedAddresses[toAddr] = struct{}{}
						registerTokenIncrease(blockData.TokenBalanceChanges, eventAddr, toAddr, value)
					}
					if _, ok := blockData.PossibleTokens[eventAddr]; !ok {
						blockData.PossibleTokens[eventAddr] = &evm.EVMPossibleToken{}
					}
					// Mints, burns, and zero-value transfers all count as transfers.
					blockData.PossibleTokens[eventAddr].NumTransfersChange++
					// Mark as mutated if transfer is between zero address
					// and nonzero address (either direction) and nonzero
					// amount. These will change the total supply as mint/
					// burn.
					if fromZero != toZero && value.Cmp(&big.Int{}) != 0 {
						blockData.PossibleTokens[eventAddr].Mutated = true
					}
					eventData.EvmLogName = common.Ptr(apiTypes.Erc20Transfer)
					eventData.EvmLogSignature = common.Ptr(ethCommon.BytesToHash(event.Topics[0]))
					eventData.EvmLogParams = []*apiTypes.EvmAbiParam{
						{
							Name:    "from",
							EvmType: "address",
							Value:   fromECAddr,
						},
						{
							Name:    "to",
							EvmType: "address",
							Value:   toECAddr,
						},
						{
							Name:    "value",
							EvmType: "uint256",
							// JSON supports encoding big integers, but many clients (javascript, jq, etc.)
							// will incorrectly parse them as floats. So we encode uint256 as a string instead.
							Value: value.String(),
						},
					}
					return nil
				},
				ERC20Approval: func(ownerECAddr ethCommon.Address, spenderECAddr ethCommon.Address, value *big.Int) error {
					if !bytes.Equal(ownerECAddr.Bytes(), eth.ZeroEthAddr) {
						ownerAddr, err2 := addresses.RegisterEthAddress(blockData.AddressPreimages, ownerECAddr.Bytes())
						if err2 != nil {
							return fmt.Errorf("owner: %w", err2)
						}
						eventData.RelatedAddresses[ownerAddr] = struct{}{}
					}
					if !bytes.Equal(spenderECAddr.Bytes(), eth.ZeroEthAddr) {
						spenderAddr, err2 := addresses.RegisterEthAddress(blockData.AddressPreimages, spenderECAddr.Bytes())
						if err2 != nil {
							return fmt.Errorf("spender: %w", err2)
						}
						eventData.RelatedAddresses[spenderAddr] = struct{}{}
					}
					if _, ok := blockData.PossibleTokens[eventAddr]; !ok {
						blockData.PossibleTokens[eventAddr] = &evm.EVMPossibleToken{}
					}
					eventData.EvmLogName = common.Ptr(apiTypes.Erc20Approval)
					eventData.EvmLogSignature = common.Ptr(ethCommon.BytesToHash(event.Topics[0]))
					eventData.EvmLogParams = []*apiTypes.EvmAbiParam{
						{
							Name:    "owner",
							EvmType: "address",
							Value:   ownerECAddr,
						},
						{
							Name:    "spender",
							EvmType: "address",
							Value:   spenderECAddr,
						},
						{
							Name:    "value",
							EvmType: "uint256",
							// JSON supports encoding big integers, but many clients (javascript, jq, etc.)
							// will incorrectly parse them as floats. So we encode uint256 as a string instead.
							Value: value.String(),
						},
					}
					return nil
				},
				ERC721Transfer: func(fromECAddr ethCommon.Address, toECAddr ethCommon.Address, tokenID *big.Int) error {
					fromZero := bytes.Equal(fromECAddr.Bytes(), eth.ZeroEthAddr)
					toZero := bytes.Equal(toECAddr.Bytes(), eth.ZeroEthAddr)
					var fromAddr, toAddr apiTypes.Address
					if !fromZero {
						var err2 error
						fromAddr, err2 = addresses.RegisterEthAddress(blockData.AddressPreimages, fromECAddr.Bytes())
						if err2 != nil {
							return fmt.Errorf("from: %w", err2)
						}
						eventData.RelatedAddresses[fromAddr] = struct{}{}
						registerTokenDecrease(blockData.TokenBalanceChanges, eventAddr, fromAddr, big.NewInt(1))
					}
					if !toZero {
						var err2 error
						toAddr, err2 = addresses.RegisterEthAddress(blockData.AddressPreimages, toECAddr.Bytes())
						if err2 != nil {
							return fmt.Errorf("to: %w", err2)
						}
						eventData.RelatedAddresses[toAddr] = struct{}{}
						registerTokenIncrease(blockData.TokenBalanceChanges, eventAddr, toAddr, big.NewInt(1))
					}
					if _, ok := blockData.PossibleTokens[eventAddr]; !ok {
						blockData.PossibleTokens[eventAddr] = &evm.EVMPossibleToken{}
					}
					// Mints, burns, and zero-value transfers all count as transfers.
					blockData.PossibleTokens[eventAddr].NumTransfersChange++
					// Mark as mutated if transfer is between zero address
					// and nonzero address (either direction) and nonzero
					// amount. These will change the total supply as mint/
					// burn.
					if fromZero && !toZero {
						pt := blockData.PossibleTokens[eventAddr]
						pt.TotalSupplyChange.Add(&pt.TotalSupplyChange, big.NewInt(1))
						pt.Mutated = true
					}
					if !fromZero && toZero {
						pt := blockData.PossibleTokens[eventAddr]
						pt.TotalSupplyChange.Sub(&pt.TotalSupplyChange, big.NewInt(1))
						pt.Mutated = true
					}
					registerNFTExist(blockData.PossibleNFTs, eventAddr, tokenID)
					// Mints, burns, and zero-value transfers all count as transfers.
					registerNFTTransfer(blockData.PossibleNFTs, eventAddr, tokenID, toZero, toAddr)
					eventData.EvmLogName = common.Ptr(evmabi.ERC721.Events["Transfer"].Name)
					eventData.EvmLogSignature = common.Ptr(ethCommon.BytesToHash(event.Topics[0]))
					eventData.EvmLogParams = []*apiTypes.EvmAbiParam{
						{
							Name:    "from",
							EvmType: "address",
							Value:   fromECAddr,
						},
						{
							Name:    "to",
							EvmType: "address",
							Value:   toECAddr,
						},
						{
							Name:    "tokenID",
							EvmType: "uint256",
							// JSON supports encoding big integers, but many clients (javascript, jq, etc.)
							// will incorrectly parse them as floats. So we encode uint256 as a string instead.
							Value: tokenID.String(),
						},
					}
					return nil
				},
				ERC721Approval: func(ownerECAddr ethCommon.Address, approvedECAddr ethCommon.Address, tokenID *big.Int) error {
					if !bytes.Equal(ownerECAddr.Bytes(), eth.ZeroEthAddr) {
						ownerAddr, err2 := addresses.RegisterEthAddress(blockData.AddressPreimages, ownerECAddr.Bytes())
						if err2 != nil {
							return fmt.Errorf("owner: %w", err2)
						}
						eventData.RelatedAddresses[ownerAddr] = struct{}{}
					}
					if !bytes.Equal(approvedECAddr.Bytes(), eth.ZeroEthAddr) {
						approvedAddr, err2 := addresses.RegisterEthAddress(blockData.AddressPreimages, approvedECAddr.Bytes())
						if err2 != nil {
							return fmt.Errorf("approved: %w", err2)
						}
						eventData.RelatedAddresses[approvedAddr] = struct{}{}
					}
					if _, ok := blockData.PossibleTokens[eventAddr]; !ok {
						blockData.PossibleTokens[eventAddr] = &evm.EVMPossibleToken{}
					}
					registerNFTExist(blockData.PossibleNFTs, eventAddr, tokenID)
					eventData.EvmLogName = common.Ptr(evmabi.ERC721.Events["Approval"].Name)
					eventData.EvmLogSignature = common.Ptr(ethCommon.BytesToHash(event.Topics[0]))
					eventData.EvmLogParams = []*apiTypes.EvmAbiParam{
						{
							Name:    "owner",
							EvmType: "address",
							Value:   ownerECAddr,
						},
						{
							Name:    "approved",
							EvmType: "address",
							Value:   approvedECAddr,
						},
						{
							Name:    "tokenID",
							EvmType: "uint256",
							// JSON supports encoding big integers, but many clients (javascript, jq, etc.)
							// will incorrectly parse them as floats. So we encode uint256 as a string instead.
							Value: tokenID.String(),
						},
					}
					return nil
				},
				ERC721ApprovalForAll: func(ownerECAddr ethCommon.Address, operatorECAddr ethCommon.Address, approved bool) error {
					if !bytes.Equal(ownerECAddr.Bytes(), eth.ZeroEthAddr) {
						ownerAddr, err2 := addresses.RegisterEthAddress(blockData.AddressPreimages, ownerECAddr.Bytes())
						if err2 != nil {
							return fmt.Errorf("owner: %w", err2)
						}
						eventData.RelatedAddresses[ownerAddr] = struct{}{}
					}
					if !bytes.Equal(operatorECAddr.Bytes(), eth.ZeroEthAddr) {
						operatorAddr, err2 := addresses.RegisterEthAddress(blockData.AddressPreimages, operatorECAddr.Bytes())
						if err2 != nil {
							return fmt.Errorf("operator: %w", err2)
						}
						eventData.RelatedAddresses[operatorAddr] = struct{}{}
					}
					if _, ok := blockData.PossibleTokens[eventAddr]; !ok {
						blockData.PossibleTokens[eventAddr] = &evm.EVMPossibleToken{}
					}
					eventData.EvmLogName = common.Ptr(evmabi.ERC721.Events["ApprovalForAll"].Name)
					eventData.EvmLogSignature = common.Ptr(ethCommon.BytesToHash(event.Topics[0]))
					eventData.EvmLogParams = []*apiTypes.EvmAbiParam{
						{
							Name:    "owner",
							EvmType: "address",
							Value:   ownerECAddr,
						},
						{
							Name:    "operator",
							EvmType: "address",
							Value:   operatorECAddr,
						},
						{
							Name:    "approved",
							EvmType: "bool",
							Value:   approved,
						},
					}
					return nil
				},
				IUniswapV2FactoryPairCreated: func(token0ECAddr ethCommon.Address, token1ECAddr ethCommon.Address, pairECAddr ethCommon.Address, allPairsLength *big.Int) error {
					token0Addr, err := addresses.RegisterEthAddress(blockData.AddressPreimages, token0ECAddr.Bytes())
					if err != nil {
						return fmt.Errorf("token0: %w", err)
					}
					eventData.RelatedAddresses[token0Addr] = struct{}{}
					token1Addr, err := addresses.RegisterEthAddress(blockData.AddressPreimages, token1ECAddr.Bytes())
					if err != nil {
						return fmt.Errorf("token1: %w", err)
					}
					eventData.RelatedAddresses[token1Addr] = struct{}{}
					pairAddr, err := addresses.RegisterEthAddress(blockData.AddressPreimages, pairECAddr.Bytes())
					if err != nil {
						return fmt.Errorf("pair: %w", err)
					}
					eventData.RelatedAddresses[pairAddr] = struct{}{}
					blockData.SwapCreations[SwapCreationKey{
						Factory: eventAddr,
						Token0:  token0Addr,
						Token1:  token1Addr,
					}] = &PossibleSwapCreation{
						Pair: pairAddr,
					}
					eventData.EvmLogName = common.Ptr(evmabi.IUniswapV2Factory.Events["PairCreated"].Name)
					eventData.EvmLogSignature = common.Ptr(ethCommon.BytesToHash(event.Topics[0]))
					eventData.EvmLogParams = []*apiTypes.EvmAbiParam{
						{
							Name:    "token0",
							EvmType: "address",
							Value:   token0ECAddr,
						},
						{
							Name:    "token1",
							EvmType: "address",
							Value:   token1ECAddr,
						},
						{
							Name:    "pair",
							EvmType: "address",
							Value:   pairECAddr,
						},
						{
							Name:    "",
							EvmType: "uint256",
							// JSON supports encoding big integers, but many clients (javascript, jq, etc.)
							// will incorrectly parse them as floats. So we encode uint256 as a string instead.
							Value: allPairsLength.String(),
						},
					}
					return nil
				},
				IUniswapV2PairMint: func(senderECAddr ethCommon.Address, amount0 *big.Int, amount1 *big.Int) error {
					senderAddr, err := addresses.RegisterEthAddress(blockData.AddressPreimages, senderECAddr.Bytes())
					if err != nil {
						return fmt.Errorf("sender: %w", err)
					}
					eventData.RelatedAddresses[senderAddr] = struct{}{}
					eventData.EvmLogName = common.Ptr(evmabi.IUniswapV2Pair.Events["Mint"].Name)
					eventData.EvmLogSignature = common.Ptr(ethCommon.BytesToHash(event.Topics[0]))
					eventData.EvmLogParams = []*apiTypes.EvmAbiParam{
						{
							Name:    "sender",
							EvmType: "address",
							Value:   senderECAddr,
						},
						{
							Name:    "amount0",
							EvmType: "uint256",
							// JSON supports encoding big integers, but many clients (javascript, jq, etc.)
							// will incorrectly parse them as floats. So we encode uint256 as a string instead.
							Value: amount0.String(),
						},
						{
							Name:    "amount1",
							EvmType: "uint256",
							// JSON supports encoding big integers, but many clients (javascript, jq, etc.)
							// will incorrectly parse them as floats. So we encode uint256 as a string instead.
							Value: amount1.String(),
						},
					}
					return nil
				},
				IUniswapV2PairBurn: func(senderECAddr ethCommon.Address, amount0 *big.Int, amount1 *big.Int, toECAddr ethCommon.Address) error {
					senderAddr, err := addresses.RegisterEthAddress(blockData.AddressPreimages, senderECAddr.Bytes())
					if err != nil {
						return fmt.Errorf("sender: %w", err)
					}
					eventData.RelatedAddresses[senderAddr] = struct{}{}
					toAddr, err := addresses.RegisterEthAddress(blockData.AddressPreimages, toECAddr.Bytes())
					if err != nil {
						return fmt.Errorf("to: %w", err)
					}
					eventData.RelatedAddresses[toAddr] = struct{}{}
					eventData.EvmLogName = common.Ptr(evmabi.IUniswapV2Pair.Events["Burn"].Name)
					eventData.EvmLogSignature = common.Ptr(ethCommon.BytesToHash(event.Topics[0]))
					eventData.EvmLogParams = []*apiTypes.EvmAbiParam{
						{
							Name:    "sender",
							EvmType: "address",
							Value:   senderECAddr,
						},
						{
							Name:    "amount0",
							EvmType: "uint256",
							// JSON supports encoding big integers, but many clients (javascript, jq, etc.)
							// will incorrectly parse them as floats. So we encode uint256 as a string instead.
							Value: amount0.String(),
						},
						{
							Name:    "amount1",
							EvmType: "uint256",
							// JSON supports encoding big integers, but many clients (javascript, jq, etc.)
							// will incorrectly parse them as floats. So we encode uint256 as a string instead.
							Value: amount1.String(),
						},
						{
							Name:    "to",
							EvmType: "address",
							Value:   toECAddr,
						},
					}
					return nil
				},
				IUniswapV2PairSwap: func(senderECAddr ethCommon.Address, amount0In *big.Int, amount1In *big.Int, amount0Out *big.Int, amount1Out *big.Int, toECAddr ethCommon.Address) error {
					senderAddr, err := addresses.RegisterEthAddress(blockData.AddressPreimages, senderECAddr.Bytes())
					if err != nil {
						return fmt.Errorf("sender: %w", err)
					}
					eventData.RelatedAddresses[senderAddr] = struct{}{}
					toAddr, err := addresses.RegisterEthAddress(blockData.AddressPreimages, toECAddr.Bytes())
					if err != nil {
						return fmt.Errorf("to: %w", err)
					}
					eventData.RelatedAddresses[toAddr] = struct{}{}
					eventData.EvmLogName = common.Ptr(evmabi.IUniswapV2Pair.Events["Swap"].Name)
					eventData.EvmLogSignature = common.Ptr(ethCommon.BytesToHash(event.Topics[0]))
					eventData.EvmLogParams = []*apiTypes.EvmAbiParam{
						{
							Name:    "sender",
							EvmType: "address",
							Value:   senderECAddr,
						},
						{
							Name:    "amount0In",
							EvmType: "uint256",
							// JSON supports encoding big integers, but many clients (javascript, jq, etc.)
							// will incorrectly parse them as floats. So we encode uint256 as a string instead.
							Value: amount0In.String(),
						},
						{
							Name:    "amount1In",
							EvmType: "uint256",
							// JSON supports encoding big integers, but many clients (javascript, jq, etc.)
							// will incorrectly parse them as floats. So we encode uint256 as a string instead.
							Value: amount1In.String(),
						},
						{
							Name:    "amount0Out",
							EvmType: "uint256",
							// JSON supports encoding big integers, but many clients (javascript, jq, etc.)
							// will incorrectly parse them as floats. So we encode uint256 as a string instead.
							Value: amount0Out.String(),
						},
						{
							Name:    "amount1Out",
							EvmType: "uint256",
							// JSON supports encoding big integers, but many clients (javascript, jq, etc.)
							// will incorrectly parse them as floats. So we encode uint256 as a string instead.
							Value: amount1Out.String(),
						},
						{
							Name:    "to",
							EvmType: "address",
							Value:   toECAddr,
						},
					}
					return nil
				},
				IUniswapV2PairSync: func(reserve0 *big.Int, reserve1 *big.Int) error {
					blockData.SwapSyncs[eventAddr] = &PossibleSwapSync{
						Reserve0: reserve0,
						Reserve1: reserve1,
					}
					eventData.EvmLogName = common.Ptr(evmabi.IUniswapV2Pair.Events["Sync"].Name)
					eventData.EvmLogSignature = common.Ptr(ethCommon.BytesToHash(event.Topics[0]))
					eventData.EvmLogParams = []*apiTypes.EvmAbiParam{
						{
							Name:    "reserve0",
							EvmType: "uint112",
							// JSON supports encoding big integers, but many clients (javascript, jq, etc.)
							// will incorrectly parse them as floats. So we encode uint256 as a string instead.
							Value: reserve0.String(),
						},
						{
							Name:    "reserve1",
							EvmType: "uint112",
							// JSON supports encoding big integers, but many clients (javascript, jq, etc.)
							// will incorrectly parse them as floats. So we encode uint256 as a string instead.
							Value: reserve1.String(),
						},
					}
					return nil
				},
				WROSEDeposit: func(ownerECAddr ethCommon.Address, amount *big.Int) error {
					wrapperAddr := eventAddr // the WROSE wrapper contract is implicitly the address that emitted the contract

					ownerAddr, err2 := addresses.RegisterEthAddress(blockData.AddressPreimages, ownerECAddr.Bytes())
					if err2 != nil {
						return fmt.Errorf("owner: %w", err2)
					}
					eventData.RelatedAddresses[ownerAddr] = struct{}{}
					registerTokenIncrease(blockData.TokenBalanceChanges, wrapperAddr, ownerAddr, amount)
					registerTokenIncrease(blockData.TokenBalanceChanges, evm.NativeRuntimeTokenAddress, wrapperAddr, amount)
					registerTokenDecrease(blockData.TokenBalanceChanges, evm.NativeRuntimeTokenAddress, ownerAddr, amount)

					// ^ The above includes dead-reckoning for the native token because no events are emitted for native token transfers.
					//
					// Example: in mainnet Emerald block 7847845, account A withdrew 1 WROSE from the WROSE contract,
					//   i.e. unwrapped 1 WROSE into ROSE. The effect is that A's WROSE balance decreases by 1,
					//   and the WROSE contract transfers 1 ROSE to A.
					//   However, that block shows a single event: evm.log Withdrawal(from=A, value=1000000000000000000)
					//   Similarly, in block 7847770, A deposited 1 ROSE into the WROSE contract.
					//   No ROSE events were emitted, only evm.log Deposit(to=A, value=1000000000000000000)
					//
					// Details for the above example:
					//  A = 0x2435ff763095d7c8ABfc1F05d1C4031B44013914
					//  WROSE = 0x21C718C22D52d0F3a789b752D4c2fD5908a8A733

					if _, ok := blockData.PossibleTokens[eventAddr]; !ok {
						blockData.PossibleTokens[eventAddr] = &evm.EVMPossibleToken{}
					}

					eventData.EvmLogName = common.Ptr(evmabi.WROSE.Events["Deposit"].Name)
					eventData.EvmLogSignature = common.Ptr(ethCommon.BytesToHash(event.Topics[0]))
					eventData.EvmLogParams = []*apiTypes.EvmAbiParam{
						{
							Name:    "dst",
							EvmType: "address",
							Value:   ownerECAddr,
						},
						{
							Name:    "wad",
							EvmType: "uint256",
							// JSON supports encoding big integers, but many clients (javascript, jq, etc.)
							// will incorrectly parse them as floats. So we encode uint256 as a string instead.
							Value: amount.String(),
						},
					}
					return nil
				},
				WROSEWithdrawal: func(ownerECAddr ethCommon.Address, amount *big.Int) error {
					wrapperAddr := eventAddr // the WROSE wrapper contract is implicitly the address that emitted the contract

					ownerAddr, err2 := addresses.RegisterEthAddress(blockData.AddressPreimages, ownerECAddr.Bytes())
					if err2 != nil {
						return fmt.Errorf("owner: %w", err2)
					}
					eventData.RelatedAddresses[ownerAddr] = struct{}{}
					registerTokenDecrease(blockData.TokenBalanceChanges, wrapperAddr, ownerAddr, amount)
					registerTokenIncrease(blockData.TokenBalanceChanges, evm.NativeRuntimeTokenAddress, ownerAddr, amount)
					registerTokenDecrease(blockData.TokenBalanceChanges, evm.NativeRuntimeTokenAddress, wrapperAddr, amount)

					if _, ok := blockData.PossibleTokens[eventAddr]; !ok {
						blockData.PossibleTokens[eventAddr] = &evm.EVMPossibleToken{}
					}

					eventData.EvmLogName = common.Ptr(evmabi.WROSE.Events["Withdrawal"].Name)
					eventData.EvmLogSignature = common.Ptr(ethCommon.BytesToHash(event.Topics[0]))
					eventData.EvmLogParams = []*apiTypes.EvmAbiParam{
						{
							Name:    "src",
							EvmType: "address",
							Value:   ownerECAddr,
						},
						{
							Name:    "wad",
							EvmType: "uint256",
							// JSON supports encoding big integers, but many clients (javascript, jq, etc.)
							// will incorrectly parse them as floats. So we encode uint256 as a string instead.
							Value: amount.String(),
						},
					}
					return nil
				},
			}); err1 != nil {
				return err1
			}
			extractedEvents = append(extractedEvents, &eventData)
			return nil
		},
		Rofl: func(event *rofl.Event, eventTxHash *string, eventIdx int) error {
			if event.AppCreated != nil {
				eventData := EventData{
					EventIdx:  eventIdx,
					TxHash:    eventTxHash,
					Type:      apiTypes.RuntimeEventTypeRoflAppCreated,
					Body:      event.AppCreated,
					WithScope: ScopedSdkEvent{Rofl: event},
				}
				extractedEvents = append(extractedEvents, &eventData)
			}
			if event.AppRemoved != nil {
				eventData := EventData{
					EventIdx:  eventIdx,
					TxHash:    eventTxHash,
					Type:      apiTypes.RuntimeEventTypeRoflAppRemoved,
					Body:      event.AppRemoved,
					WithScope: ScopedSdkEvent{Rofl: event},
				}
				extractedEvents = append(extractedEvents, &eventData)
			}
			if event.AppUpdated != nil {
				eventData := EventData{
					EventIdx:  eventIdx,
					TxHash:    eventTxHash,
					Type:      apiTypes.RuntimeEventTypeRoflAppUpdated,
					Body:      event.AppUpdated,
					WithScope: ScopedSdkEvent{Rofl: event},
				}
				extractedEvents = append(extractedEvents, &eventData)
			}
			if event.InstanceRegistered != nil {
				eventData := EventData{
					EventIdx:  eventIdx,
					TxHash:    eventTxHash,
					Type:      apiTypes.RuntimeEventTypeRoflInstanceRegistered,
					Body:      event.InstanceRegistered,
					WithScope: ScopedSdkEvent{Rofl: event},
				}
				extractedEvents = append(extractedEvents, &eventData)
			}
			return nil
		},
		RoflMarket: func(event *roflmarket.Event, eventTxHash *string, eventIdx int) error {
			if event.ProviderCreated != nil {
				eventData := EventData{
					EventIdx:  eventIdx,
					TxHash:    eventTxHash,
					Type:      apiTypes.RuntimeEventTypeRoflmarketProviderCreated,
					Body:      event.ProviderCreated,
					WithScope: ScopedSdkEvent{RoflMarket: event},
				}
				address, err1 := addresses.FromSdkAddress(&event.ProviderCreated.Address)
				if err1 != nil {
					return fmt.Errorf("provider created address: %w", err1)
				}
				eventData.RelatedAddresses[address] = struct{}{}

				extractedEvents = append(extractedEvents, &eventData)
			}
			if event.ProviderUpdated != nil {
				eventData := EventData{
					EventIdx:  eventIdx,
					TxHash:    eventTxHash,
					Type:      apiTypes.RuntimeEventTypeRoflmarketProviderUpdated,
					Body:      event.ProviderUpdated,
					WithScope: ScopedSdkEvent{RoflMarket: event},
				}
				address, err1 := addresses.FromSdkAddress(&event.ProviderUpdated.Address)
				if err1 != nil {
					return fmt.Errorf("provider updated address: %w", err1)
				}
				eventData.RelatedAddresses[address] = struct{}{}

				extractedEvents = append(extractedEvents, &eventData)
			}
			if event.ProviderRemoved != nil {
				eventData := EventData{
					EventIdx:  eventIdx,
					TxHash:    eventTxHash,
					Type:      apiTypes.RuntimeEventTypeRoflmarketProviderRemoved,
					Body:      event.ProviderRemoved,
					WithScope: ScopedSdkEvent{RoflMarket: event},
				}
				address, err1 := addresses.FromSdkAddress(&event.ProviderRemoved.Address)
				if err1 != nil {
					return fmt.Errorf("provider removed address: %w", err1)
				}
				eventData.RelatedAddresses[address] = struct{}{}

				extractedEvents = append(extractedEvents, &eventData)
			}
			if event.InstanceCreated != nil {
				eventData := EventData{
					EventIdx:  eventIdx,
					TxHash:    eventTxHash,
					Type:      apiTypes.RuntimeEventTypeRoflmarketInstanceCreated,
					Body:      event.InstanceCreated,
					WithScope: ScopedSdkEvent{RoflMarket: event},
				}
				address, err1 := addresses.FromSdkAddress(&event.InstanceCreated.Provider)
				if err1 != nil {
					return fmt.Errorf("instance created provider address: %w", err1)
				}
				eventData.RelatedAddresses[address] = struct{}{}

				extractedEvents = append(extractedEvents, &eventData)
			}
			if event.InstanceUpdated != nil {
				eventData := EventData{
					EventIdx:  eventIdx,
					TxHash:    eventTxHash,
					Type:      apiTypes.RuntimeEventTypeRoflmarketInstanceUpdated,
					Body:      event.InstanceUpdated,
					WithScope: ScopedSdkEvent{RoflMarket: event},
				}
				address, err1 := addresses.FromSdkAddress(&event.InstanceUpdated.Provider)
				if err1 != nil {
					return fmt.Errorf("instance updated provider address: %w", err1)
				}
				eventData.RelatedAddresses[address] = struct{}{}

				extractedEvents = append(extractedEvents, &eventData)
			}
			if event.InstanceAccepted != nil {
				eventData := EventData{
					EventIdx:  eventIdx,
					TxHash:    eventTxHash,
					Type:      apiTypes.RuntimeEventTypeRoflmarketInstanceAccepted,
					Body:      event.InstanceAccepted,
					WithScope: ScopedSdkEvent{RoflMarket: event},
				}
				address, err1 := addresses.FromSdkAddress(&event.InstanceAccepted.Provider)
				if err1 != nil {
					return fmt.Errorf("instance accepted provider address: %w", err1)
				}
				eventData.RelatedAddresses[address] = struct{}{}

				extractedEvents = append(extractedEvents, &eventData)
			}
			if event.InstanceCancelled != nil {
				eventData := EventData{
					EventIdx:  eventIdx,
					TxHash:    eventTxHash,
					Type:      apiTypes.RuntimeEventTypeRoflmarketInstanceCancelled,
					Body:      event.InstanceCancelled,
					WithScope: ScopedSdkEvent{RoflMarket: event},
				}
				address, err1 := addresses.FromSdkAddress(&event.InstanceCancelled.Provider)
				if err1 != nil {
					return fmt.Errorf("instance cancelled provider address: %w", err1)
				}
				eventData.RelatedAddresses[address] = struct{}{}

				extractedEvents = append(extractedEvents, &eventData)
			}
			if event.InstanceRemoved != nil {
				eventData := EventData{
					EventIdx:  eventIdx,
					TxHash:    eventTxHash,
					Type:      apiTypes.RuntimeEventTypeRoflmarketInstanceRemoved,
					Body:      event.InstanceRemoved,
					WithScope: ScopedSdkEvent{RoflMarket: event},
				}
				address, err1 := addresses.FromSdkAddress(&event.InstanceRemoved.Provider)
				if err1 != nil {
					return fmt.Errorf("instance removed provider address: %w", err1)
				}
				eventData.RelatedAddresses[address] = struct{}{}

				extractedEvents = append(extractedEvents, &eventData)
			}
			if event.InstanceCommandQueued != nil {
				eventData := EventData{
					EventIdx:  eventIdx,
					TxHash:    eventTxHash,
					Type:      apiTypes.RuntimeEventTypeRoflmarketInstanceCommandQueued,
					Body:      event.InstanceCommandQueued,
					WithScope: ScopedSdkEvent{RoflMarket: event},
				}
				address, err1 := addresses.FromSdkAddress(&event.InstanceCommandQueued.Provider)
				if err1 != nil {
					return fmt.Errorf("instance command queued provider address: %w", err1)
				}
				eventData.RelatedAddresses[address] = struct{}{}

				extractedEvents = append(extractedEvents, &eventData)
			}
			return nil
		},
	}); err != nil {
		return nil, err
	}
	return extractedEvents, nil
}
