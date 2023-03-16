package runtime

import (
	"github.com/oasisprotocol/oasis-indexer/analyzer"
	"github.com/oasisprotocol/oasis-indexer/analyzer/queries"
	"github.com/oasisprotocol/oasis-indexer/log"
	"github.com/oasisprotocol/oasis-indexer/storage"
)

const (
	coreHandlerName = "core"
)

// CoreHandler implements support for transforming and inserting data from the
// `core` module for a runtime into target storage.
type CoreHandler struct {
	source  storage.RuntimeSourceStorage
	runtime analyzer.Runtime
	logger  *log.Logger
}

// NewCoreHandler creates a new handler for `core` module data.
func NewCoreHandler(source storage.RuntimeSourceStorage, runtime analyzer.Runtime, logger *log.Logger) *CoreHandler {
	return &CoreHandler{source, runtime, logger}
}

// PrepareCoreData prepares raw data from the `core` module for insertion.
// into target storage.
func (h *CoreHandler) PrepareData(batch *storage.QueryBatch, data *storage.RuntimeAllData) error {
	for _, f := range []func(*storage.QueryBatch, *storage.CoreData) error{
		h.queueGasUsed,
	} {
		if err := f(batch, data.CoreData); err != nil {
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
			queries.RuntimeGasUsedInsert,
			h.runtime,
			data.Round,
			nil, // TODO: Get sender address from transaction data
			gasUsed.Amount,
		)
	}

	return nil
}
