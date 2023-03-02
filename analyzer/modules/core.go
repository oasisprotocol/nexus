package modules

import (
	"context"
	"fmt"

	"github.com/oasisprotocol/oasis-indexer/analyzer"
	"github.com/oasisprotocol/oasis-indexer/log"
	"github.com/oasisprotocol/oasis-indexer/storage"
)

const (
	coreHandlerName = "core"
)

// CoreHandler implements support for transforming and inserting data from the
// `core` module for a runtime into target storage.
type CoreHandler struct {
	source      storage.RuntimeSourceStorage
	runtimeName string
	qf          *analyzer.QueryFactory
	logger      *log.Logger
}

// NewCoreHandler creates a new handler for `core` module data.
func NewCoreHandler(source storage.RuntimeSourceStorage, runtimeName string, qf *analyzer.QueryFactory, logger *log.Logger) *CoreHandler {
	return &CoreHandler{source, runtimeName, qf, logger}
}

// PrepareCoreData prepares raw data from the `core` module for insertion.
// into target storage.
func (h *CoreHandler) PrepareData(ctx context.Context, round uint64, batch *storage.QueryBatch) error {
	data, err := h.source.CoreData(ctx, round)
	if err != nil {
		return fmt.Errorf("error retrieving core data: %w", err)
	}

	for _, f := range []func(*storage.QueryBatch, *storage.CoreData) error{
		h.queueGasUsed,
	} {
		if err := f(batch, data); err != nil {
			return err
		}
	}

	return nil
}

// Name returns the name of the handler.
func (h *CoreHandler) Name() string {
	return coreHandlerName
}

func (h *CoreHandler) queueGasUsed(batch *storage.QueryBatch, data *storage.CoreData) error {
	for _, gasUsed := range data.GasUsed {
		batch.Queue(
			h.qf.RuntimeGasUsedInsertQuery(),
			h.runtimeName,
			data.Round,
			nil, // TODO: Get sender address from transaction data
			gasUsed.Amount,
		)
	}

	return nil
}
