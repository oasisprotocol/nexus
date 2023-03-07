package v1

import (
	"fmt"

	"github.com/oasisprotocol/oasis-core/go/common/cbor"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/modules/accounts"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/modules/consensusaccounts"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/modules/evm"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/types"

	uncategorized "github.com/oasisprotocol/oasis-indexer/analyzer/uncategorized"
	apiTypes "github.com/oasisprotocol/oasis-indexer/api/v1/types"
	"github.com/oasisprotocol/oasis-indexer/storage/client"
)

func renderRuntimeTransaction(storageTransaction client.RuntimeTransaction) (apiTypes.RuntimeTransaction, error) {
	var utx types.UnverifiedTransaction
	if err := cbor.Unmarshal(storageTransaction.Raw, &utx); err != nil {
		return apiTypes.RuntimeTransaction{}, fmt.Errorf("utx unmarshal: %w", err)
	}
	tx, err := uncategorized.OpenUtxNoVerify(&utx)
	if err != nil {
		return apiTypes.RuntimeTransaction{}, fmt.Errorf("utx open no verify: %w", err)
	}
	sender0Addr, err := uncategorized.StringifyAddressSpec(&tx.AuthInfo.SignerInfo[0].AddressSpec)
	if err != nil {
		return apiTypes.RuntimeTransaction{}, fmt.Errorf("signer 0: %w", err)
	}
	sender0 := string(sender0Addr)
	sender0Eth := uncategorized.EthAddrReference(storageTransaction.AddressPreimage[sender0])
	var cr types.CallResult
	if err = cbor.Unmarshal(storageTransaction.ResultRaw, &cr); err != nil {
		return apiTypes.RuntimeTransaction{}, fmt.Errorf("result unmarshal: %w", err)
	}
	var body map[string]interface{}
	if err = cbor.Unmarshal(tx.Call.Body, &body); err != nil {
		return apiTypes.RuntimeTransaction{}, fmt.Errorf("body unmarshal: %w", err)
	}
	apiTransaction := apiTypes.RuntimeTransaction{
		Round:      storageTransaction.Round,
		Index:      storageTransaction.Index,
		Hash:       storageTransaction.Hash,
		EthHash:    storageTransaction.EthHash,
		GasUsed:    storageTransaction.GasUsed,
		Size:       storageTransaction.Size,
		Timestamp:  storageTransaction.Timestamp,
		Sender0:    sender0,
		Sender0Eth: sender0Eth,
		Nonce0:     tx.AuthInfo.SignerInfo[0].Nonce,
		Fee:        tx.AuthInfo.Fee.Amount.Amount.String(),
		GasLimit:   tx.AuthInfo.Fee.Gas,
		Method:     tx.Call.Method,
		Body:       body,
		Success:    cr.IsSuccess(),
	}
	if !cr.IsSuccess() {
		code := int(cr.Failed.Code)
		apiTransaction.Code = &code
		apiTransaction.Module = &cr.Failed.Module
		apiTransaction.Message = &cr.Failed.Message
	}
	if err = uncategorized.VisitCall(&tx.Call, &cr, &uncategorized.CallHandler{
		AccountsTransfer: func(body *accounts.Transfer) error {
			toAddr, err2 := uncategorized.StringifySdkAddress(&body.To)
			if err2 != nil {
				return fmt.Errorf("to: %w", err2)
			}
			to := string(toAddr)
			apiTransaction.To = &to
			amount, err2 := uncategorized.StringifyNativeDenomination(&body.Amount)
			if err2 != nil {
				return fmt.Errorf("amount: %w", err2)
			}
			apiTransaction.Amount = &amount
			return nil
		},
		ConsensusAccountsDeposit: func(body *consensusaccounts.Deposit) error {
			if body.To != nil {
				toAddr, err2 := uncategorized.StringifySdkAddress(body.To)
				if err2 != nil {
					return fmt.Errorf("to: %w", err2)
				}
				to := string(toAddr)
				toEth := uncategorized.EthAddrReference(storageTransaction.AddressPreimage[to])
				apiTransaction.To = &to
				apiTransaction.ToEth = toEth
			} else {
				apiTransaction.To = &sender0
				apiTransaction.ToEth = sender0Eth
			}
			amount, err2 := uncategorized.StringifyNativeDenomination(&body.Amount)
			if err2 != nil {
				return fmt.Errorf("amount: %w", err2)
			}
			apiTransaction.Amount = &amount
			return nil
		},
		ConsensusAccountsWithdraw: func(body *consensusaccounts.Withdraw) error {
			if body.To != nil {
				toAddr, err2 := uncategorized.StringifySdkAddress(body.To)
				if err2 != nil {
					return fmt.Errorf("to: %w", err2)
				}
				to := string(toAddr)
				toEth := uncategorized.EthAddrReference(storageTransaction.AddressPreimage[to])
				// Beware, this is the address of an account in the consensus
				// layer, not an account in the runtime indicated in this API
				// request.
				apiTransaction.To = &to
				apiTransaction.ToEth = toEth
			} else {
				apiTransaction.To = &sender0
				apiTransaction.ToEth = sender0Eth
			}
			// todo: ensure native denomination?
			amount := body.Amount.Amount.String()
			apiTransaction.Amount = &amount
			return nil
		},
		EVMCreate: func(body *evm.Create, ok *[]byte) error {
			if !cr.IsUnknown() && cr.IsSuccess() && len(*ok) == 32 {
				// todo: is this rigorous enough?
				toAddr, err2 := uncategorized.StringifyEthAddress(uncategorized.SliceEthAddress(*ok))
				if err2 != nil {
					return fmt.Errorf("created contract: %w", err2)
				}
				to := string(toAddr)
				toEth := uncategorized.EthAddrReference(storageTransaction.AddressPreimage[to])
				apiTransaction.To = &to
				apiTransaction.ToEth = toEth
			}
			amount := uncategorized.StringifyBytes(body.Value)
			apiTransaction.Amount = &amount
			return nil
		},
		EVMCall: func(body *evm.Call, ok *[]byte) error {
			toAddr, err2 := uncategorized.StringifyEthAddress(body.Address)
			if err2 != nil {
				return fmt.Errorf("to: %w", err2)
			}
			to := string(toAddr)
			toEth := uncategorized.EthAddrReference(storageTransaction.AddressPreimage[to])
			apiTransaction.To = &to
			apiTransaction.ToEth = toEth
			amount := uncategorized.StringifyBytes(body.Value)
			apiTransaction.Amount = &amount
			return nil
		},
	}); err != nil {
		return apiTypes.RuntimeTransaction{}, err
	}
	return apiTransaction, nil
}
