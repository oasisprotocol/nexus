package modules

import (
	"context"

	"github.com/oasislabs/oasis-indexer/storage"
)

// ModuleHandler handles parsing rounds for a runtime module.
type ModuleHandler interface {
	PrepareData(ctx context.Context, round uint64, batch *storage.QueryBatch) error
}
