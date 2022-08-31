package modules

import (
	"context"

	"github.com/oasisprotocol/oasis-indexer/storage"
)

// ModuleHandler handles parsing rounds for a runtime module.
type ModuleHandler interface {
	// PrepareData prepares data at the specified from the module this ModuleHandler is for
	// insertion into a relational database via the provided QueryBatch.
	PrepareData(ctx context.Context, round uint64, batch *storage.QueryBatch) error

	// Name returns the name of this ModuleHandler.
	Name() string
}
