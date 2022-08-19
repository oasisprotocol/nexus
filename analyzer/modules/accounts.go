package modules

import (
	"context"
	"fmt"

	"github.com/oasislabs/oasis-indexer/storage"
)

// AccountsHandler implements support for transforming and inserting data from the
// `accounts` module for a runtime into target storage.
type AccountsHandler struct {
	source storage.RuntimeSourceStorage
}

// NewAccountsHandler creates a new handler for `accounts` module data.
func NewAccountsHandler(source storage.RuntimeSourceStorage) *AccountsHandler {
	return &AccountsHandler{source}
}

// PrepareAccountsData prepares raw data from the `accounts` module for insertion
// into target storage.
func (a *AccountsHandler) PrepareAccountsData(ctx context.Context, round uint64, batch *storage.QueryBatch) error {
	data, err := a.source.AccountsData(ctx, round)
	if err != nil {
		return err
	}

	for _, transfer := range data.Transfers {
		fmt.Printf("transfer from %s to %s of %s\n", transfer.From.String(), transfer.To.String(), transfer.Amount.String())
	}
	return nil
}
