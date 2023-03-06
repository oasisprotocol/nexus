package modules

import (
	"context"
	"fmt"
	"math/big"

	"github.com/oasisprotocol/oasis-indexer/analyzer"
	"github.com/oasisprotocol/oasis-indexer/analyzer/queries"
	"github.com/oasisprotocol/oasis-indexer/log"
	"github.com/oasisprotocol/oasis-indexer/storage"
)

const (
	accountsHandlerName = "accounts"
)

// AccountsHandler implements support for transforming and inserting data from the
// `accounts` module for a runtime into target storage.
type AccountsHandler struct {
	source  storage.RuntimeSourceStorage
	runtime analyzer.Runtime
	logger  *log.Logger
}

// NewAccountsHandler creates a new handler for `accounts` module data.
func NewAccountsHandler(source storage.RuntimeSourceStorage, runtime analyzer.Runtime, logger *log.Logger) *AccountsHandler {
	return &AccountsHandler{source, runtime, logger}
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
			queries.RuntimeMintInsert,
			h.runtime,
			data.Round,
			mint.Owner.String(),
			h.source.StringifyDenomination(mint.Amount.Denomination),
			mint.Amount.Amount.String(),
		)
		// Increase minter's balance.
		batch.Queue(
			queries.RuntimeNativeBalanceUpdate,
			h.runtime,
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
			queries.RuntimeBurnInsert,
			h.runtime,
			data.Round,
			burn.Owner.String(),
			h.source.StringifyDenomination(burn.Amount.Denomination),
			burn.Amount.Amount.String(),
		)
		// Decrease burner's balance.
		batch.Queue(
			queries.RuntimeNativeBalanceUpdate,
			h.runtime,
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
			queries.RuntimeTransferInsert,
			h.runtime,
			data.Round,
			transfer.From.String(),
			transfer.To.String(),
			h.source.StringifyDenomination(transfer.Amount.Denomination),
			transfer.Amount.Amount.String(),
		)
		// Increase receiver's balance.
		batch.Queue(
			queries.RuntimeNativeBalanceUpdate,
			h.runtime,
			transfer.To.String(),
			h.source.StringifyDenomination(transfer.Amount.Denomination),
			transfer.Amount.Amount.String(),
		)
		// Decrease sender's balance.
		batch.Queue(
			queries.RuntimeNativeBalanceUpdate,
			h.runtime,
			transfer.From.String(),
			h.source.StringifyDenomination(transfer.Amount.Denomination),
			(&big.Int{}).Neg(transfer.Amount.Amount.ToBigInt()).String(),
		)
	}

	return nil
}
