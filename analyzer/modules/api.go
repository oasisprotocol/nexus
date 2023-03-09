package modules

import (
	"github.com/oasisprotocol/oasis-indexer/storage"
)

// ModuleHandler handles parsing rounds for a runtime module.
type ModuleHandler interface {
	// PrepareData prepares data from the specified module for insertion
	// into a relational database via the provided QueryBatch.
	PrepareData(batch *storage.QueryBatch, data *storage.RuntimeAllData) error

	// Name returns the name of this ModuleHandler.
	Name() string
}
