package modules

import (
	"context"
	"fmt"
	"math/big"

	"github.com/oasisprotocol/oasis-indexer/analyzer"
	"github.com/oasisprotocol/oasis-indexer/log"
	"github.com/oasisprotocol/oasis-indexer/storage"
)

const (
	accountsHandlerName = "accounts"
)

// AccountsHandler implements support for transforming and inserting data from the
// `accounts` module for a runtime into target storage.
type AccountsHandler struct {
	source      storage.RuntimeSourceStorage
	runtimeName string
	qf          *analyzer.QueryFactory
	logger      *log.Logger
}

// NewAccountsHandler creates a new handler for `accounts` module data.
func NewAccountsHandler(source storage.RuntimeSourceStorage, runtimeName string, qf *analyzer.QueryFactory, logger *log.Logger) *AccountsHandler {
	return &AccountsHandler{source, runtimeName, qf, logger}
}

// PrepareAccountsData prepares raw data from the `accounts` module for insertion.
// into target storage.
func (h *AccountsHandler) PrepareData(ctx context.Context, round uint64, batch *storage.QueryBatch) error {
	data, err := h.source.AccountsData(ctx, round)
	if err != nil {
		return fmt.Errorf("error retrieving accounts data: %w", err)
	}

	for _, f := range []func(*storage.QueryBatch, *storage.AccountsData) error{
		h.queueMints,
		h.queueBurns,
		h.queueTransfers,
	} {
		if err := f(batch, data); err != nil {
			return err
		}
	}

	return nil
}

// Name returns the name of the handler.
func (h *AccountsHandler) Name() string {
	return accountsHandlerName
}

func (h *AccountsHandler) queueMints(batch *storage.QueryBatch, data *storage.AccountsData) error {
	for _, mint := range data.Mints {
		// Record the event.
		batch.Queue(
			h.qf.RuntimeMintInsertQuery(),
			h.runtimeName,
			data.Round,
			mint.Owner.String(),
			h.source.StringifyDenomination(mint.Amount.Denomination),
			mint.Amount.Amount.String(),
		)
		// Increase minter's balance.
		batch.Queue(
			h.qf.RuntimeNativeBalanceUpdateQuery(),
			h.runtimeName,
			mint.Owner.String(),
			h.source.StringifyDenomination(mint.Amount.Denomination),
			mint.Amount.Amount.String(),
		)
	}

	return nil
}

func (h *AccountsHandler) queueBurns(batch *storage.QueryBatch, data *storage.AccountsData) error {
	for _, burn := range data.Burns {
		// Record the event.
		batch.Queue(
			h.qf.RuntimeBurnInsertQuery(),
			h.runtimeName,
			data.Round,
			burn.Owner.String(),
			h.source.StringifyDenomination(burn.Amount.Denomination),
			burn.Amount.Amount.String(),
		)
		// Decrease burner's balance.
		batch.Queue(
			h.qf.RuntimeNativeBalanceUpdateQuery(),
			h.runtimeName,
			burn.Owner.String(),
			h.source.StringifyDenomination(burn.Amount.Denomination),
			(&big.Int{}).Neg(burn.Amount.Amount.ToBigInt()).String(),
		)
	}

	return nil
}

func (h *AccountsHandler) queueTransfers(batch *storage.QueryBatch, data *storage.AccountsData) error {
	for _, transfer := range data.Transfers {
		// Record the event.
		batch.Queue(
			h.qf.RuntimeTransferInsertQuery(),
			h.runtimeName,
			data.Round,
			transfer.From.String(),
			transfer.To.String(),
			h.source.StringifyDenomination(transfer.Amount.Denomination),
			transfer.Amount.Amount.String(),
		)
		// Increase receiver's balance.
		batch.Queue(
			h.qf.RuntimeNativeBalanceUpdateQuery(),
			h.runtimeName,
			transfer.To.String(),
			h.source.StringifyDenomination(transfer.Amount.Denomination),
			transfer.Amount.Amount.String(),
		)
		// Decrease sender's balance.
		batch.Queue(
			h.qf.RuntimeNativeBalanceUpdateQuery(),
			h.runtimeName,
			transfer.From.String(),
			h.source.StringifyDenomination(transfer.Amount.Denomination),
			(&big.Int{}).Neg(transfer.Amount.Amount.ToBigInt()).String(),
		)
	}

	return nil
}
