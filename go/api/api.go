// Package api defines API handlers for the Oasis Indexer API.
package api

import (
	"github.com/oasislabs/oasis-block-indexer/go/log"
	"github.com/oasislabs/oasis-block-indexer/go/oasis-indexer/cmd/common"
	"github.com/oasislabs/oasis-block-indexer/go/storage"
)

const (
	moduleName = "api.handler"
)

var (
	latestMetadata = APIMetadata{
		Major: 0,
		Minor: 1,
		Patch: 0,
	}
)

// APIHandler handles API requests.
type APIHandler struct {
	client storage.TargetStorage
	logger *log.Logger
}

// NewAPIHandler creates a new API handler.
func NewAPIHandler(c storage.TargetStorage) *APIHandler {
	logger := common.Logger().WithModule(moduleName)
	return &APIHandler{
		client: c,
		logger: logger,
	}
}
