package modules

import (
	"context"
	"fmt"

	"github.com/oasislabs/oasis-indexer/log"
	"github.com/oasislabs/oasis-indexer/storage"
)

// CoreHandler implements support for transforming and inserting data from the
// `core` module for a runtime into target storage.
type CoreHandler struct {
	source storage.RuntimeSourceStorage
	logger *log.Logger
}

// NewCoreHandler creates a new handler for `core` module data.
func NewCoreHandler(source storage.RuntimeSourceStorage, logger *log.Logger) *CoreHandler {
	return &CoreHandler{source, logger}
}

// PrepareCoreData prepares raw data from the `core` module for insertion
// into target storage.
func (h *CoreHandler) PrepareData(ctx context.Context, round uint64, batch *storage.QueryBatch) error {
	data, err := h.source.CoreData(ctx, round)
	if err != nil {
		h.logger.Error("error retrieving core data",
			"error", err,
		)
		return err
	}

	for _, gasUsed := range data.GasUsed {
		h.logger.Error(fmt.Sprintf("gas used: %d\n", gasUsed.Amount))
	}
	return nil
}
