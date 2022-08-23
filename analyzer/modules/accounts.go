package modules

import (
	"context"

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
	source storage.RuntimeSourceStorage
	qf     *analyzer.QueryFactory
	logger *log.Logger
}

// NewAccountsHandler creates a new handler for `accounts` module data.
func NewAccountsHandler(source storage.RuntimeSourceStorage, qf *analyzer.QueryFactory, logger *log.Logger) *AccountsHandler {
	return &AccountsHandler{source, qf, logger}
}

// PrepareAccountsData prepares raw data from the `accounts` module for insertion
// into target storage.
func (h *AccountsHandler) PrepareData(ctx context.Context, round uint64, batch *storage.QueryBatch) error {
	data, err := h.source.AccountsData(ctx, round)
	if err != nil {
		h.logger.Error("error retrieving accounts data",
			"error", err,
		)
		return err
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
		batch.Queue(
			h.qf.RuntimeMintInsertQuery(),
			data.Round,
			mint.Owner.String(),
			mint.Amount.String(),
		)
	}

	return nil
}

func (h *AccountsHandler) queueBurns(batch *storage.QueryBatch, data *storage.AccountsData) error {
	for _, burn := range data.Burns {
		batch.Queue(
			h.qf.RuntimeBurnInsertQuery(),
			data.Round,
			burn.Owner.String(),
			burn.Amount.String(),
		)
	}

	return nil
}

func (h *AccountsHandler) queueTransfers(batch *storage.QueryBatch, data *storage.AccountsData) error {
	for _, transfer := range data.Transfers {
		batch.Queue(
			h.qf.RuntimeTransferInsertQuery(),
			data.Round,
			transfer.From.String(),
			transfer.To.String(),
			transfer.Amount.String(),
		)
	}

	return nil
}
