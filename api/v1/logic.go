package v1

import (
	"fmt"

	"github.com/oasisprotocol/oasis-core/go/common/cbor"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/modules/accounts"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/modules/consensusaccounts"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/modules/evm"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/types"

	uncategorized "github.com/oasisprotocol/oasis-indexer/analyzer/uncategorized"
	"github.com/oasisprotocol/oasis-indexer/storage/client"
)

func renderRuntimeTransaction(storageTransaction client.RuntimeTransaction) (RuntimeTransaction, error) {
	var utx types.UnverifiedTransaction
	if err := cbor.Unmarshal(storageTransaction.Raw, &utx); err != nil {
		return RuntimeTransaction{}, fmt.Errorf("utx unmarshal: %w", err)
	}
	tx, err := uncategorized.OpenUtxNoVerify(&utx)
	if err != nil {
		return RuntimeTransaction{}, fmt.Errorf("utx open no verify: %w", err)
	}
	sender0, err := uncategorized.StringifyAddressSpec(&tx.AuthInfo.SignerInfo[0].AddressSpec)
	if err != nil {
		return RuntimeTransaction{}, fmt.Errorf("signer 0: %w", err)
	}
	var cr types.CallResult
	if err = cbor.Unmarshal(storageTransaction.ResultRaw, &cr); err != nil {
		return RuntimeTransaction{}, fmt.Errorf("result unmarshal: %w", err)
	}
	apiTransaction := RuntimeTransaction{
		Round:   storageTransaction.Round,
		Index:   storageTransaction.Index,
		Hash:    storageTransaction.Hash,
		EthHash: storageTransaction.EthHash,
		// TODO: Get timestamp from that round's block
		Sender0:  sender0,
		Nonce0:   tx.AuthInfo.SignerInfo[0].Nonce,
		Fee:      tx.AuthInfo.Fee.Amount.Amount.String(),
		GasLimit: tx.AuthInfo.Fee.Gas,
		Method:   tx.Call.Method,
		Body:     tx.Call.Body,
		Success:  cr.IsSuccess(),
	}
	if err = uncategorized.VisitCall(&tx.Call, &cr, &uncategorized.CallHandler{
		AccountsTransfer: func(body *accounts.Transfer) error {
			to, err2 := uncategorized.StringifySdkAddress(&body.To)
			if err2 != nil {
				return fmt.Errorf("to: %w", err2)
			}
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
				to, err2 := uncategorized.StringifySdkAddress(body.To)
				if err2 != nil {
					return fmt.Errorf("to: %w", err2)
				}
				apiTransaction.To = &to
			} else {
				apiTransaction.To = &sender0
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
				to, err2 := uncategorized.StringifySdkAddress(body.To)
				if err2 != nil {
					return fmt.Errorf("to: %w", err2)
				}
				// Beware, this is the address of an account in the consensus
				// layer, not an account in the runtime indicated in this API
				// request.
				apiTransaction.To = &to
			} else {
				apiTransaction.To = &sender0
			}
			// todo: ensure native denomination?
			amount := body.Amount.Amount.String()
			apiTransaction.Amount = &amount
			return nil
		},
		EvmCreate: func(body *evm.Create, ok *[]byte) error {
			if !cr.IsUnknown() && cr.IsSuccess() && len(*ok) == 32 {
				// todo: is this rigorous enough?
				to, err2 := uncategorized.StringifyEthAddress(uncategorized.SliceEthAddress(*ok))
				if err2 != nil {
					return fmt.Errorf("created contract: %w", err2)
				}
				apiTransaction.To = &to
			}
			amount := uncategorized.StringifyBytes(body.Value)
			apiTransaction.Amount = &amount
			return nil
		},
		EvmCall: func(body *evm.Call, ok *[]byte) error {
			to, err2 := uncategorized.StringifyEthAddress(body.Address)
			if err2 != nil {
				return fmt.Errorf("to: %w", err2)
			}
			apiTransaction.To = &to
			amount := uncategorized.StringifyBytes(body.Value)
			apiTransaction.Amount = &amount
			return nil
		},
	}); err != nil {
		return RuntimeTransaction{}, err
	}
	return apiTransaction, nil
}
