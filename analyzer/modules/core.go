package modules

import (
	"context"
	"fmt"

	"github.com/oasislabs/oasis-indexer/storage"
)

// CoreHandler implements support for transforming and inserting data from the
// `core` module for a runtime into target storage.
type CoreHandler struct {
	source storage.RuntimeSourceStorage
}

// NewCoreHandler creates a new handler for `core` module data.
func NewCoreHandler(source storage.RuntimeSourceStorage) *CoreHandler {
	return &CoreHandler{source}
}

// PrepareCoreData prepares raw data from the `core` module for insertion
// into target storage.
func (a *CoreHandler) PrepareCoreData(ctx context.Context, round uint64, batch *storage.QueryBatch) error {
	data, err := a.source.CoreData(ctx, round)
	if err != nil {
		return err
	}

	for _, gasUsed := range data.GasUsed {
		fmt.Printf("gas used: %d\n", gasUsed.Amount)
	}
	return nil
}
