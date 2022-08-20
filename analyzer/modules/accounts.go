package modules

import (
	"context"
	"fmt"

	"github.com/oasislabs/oasis-indexer/log"
	"github.com/oasislabs/oasis-indexer/storage"
)

// AccountsHandler implements support for transforming and inserting data from the
// `accounts` module for a runtime into target storage.
type AccountsHandler struct {
	source storage.RuntimeSourceStorage
	logger *log.Logger
}

// NewAccountsHandler creates a new handler for `accounts` module data.
func NewAccountsHandler(source storage.RuntimeSourceStorage, logger *log.Logger) *AccountsHandler {
	return &AccountsHandler{source, logger}
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

	for _, transfer := range data.Transfers {
		h.logger.Error(fmt.Sprintf("transfer from %s to %s of %s\n", transfer.From.String(), transfer.To.String(), transfer.Amount.String()))
	}
	return nil
}
