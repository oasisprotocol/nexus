// Package api defines API handlers for the Oasis Indexer API.
package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	v1 "github.com/oasisprotocol/oasis-indexer/api/v1"
	"github.com/oasisprotocol/oasis-indexer/log"
	"github.com/oasisprotocol/oasis-indexer/storage"
)

const (
	moduleName = "api"
)

// Handler is a handler that handles API requests.
type Handler interface {
	// RegisterRoutes registers middlewares for this API handler.
	RegisterMiddlewares(chi.Router)

	// RegisterRoutes registers routes for this API handler.
	RegisterRoutes(chi.Router)

	// Name returns the name of this API handler.
	Name() string
}

// IndexerAPI is an API for the Oasis Indexer.
type IndexerAPI struct {
	router   *chi.Mux
	handlers []Handler
	logger   *log.Logger
}

// NewIndexerAPI creates a new Indexer API.
func NewIndexerAPI(chainID string, db storage.TargetStorage, l *log.Logger) *IndexerAPI {
	r := chi.NewRouter()

	// Register handlers.
	v1Handler := v1.NewHandler(chainID, db, l)
	handlers := []Handler{
		v1Handler,
	}
	for _, handler := range handlers {
		handler.RegisterMiddlewares(r)
	}
	r.Use(middleware.Recoverer)

	// Register routes.
	for _, handler := range handlers {
		handler.RegisterRoutes(r)
	}

	return &IndexerAPI{
		router:   r,
		handlers: handlers,
		logger:   l.WithModule(moduleName),
	}
}

// Router gets the router for this Handler.
func (a *IndexerAPI) Router() *chi.Mux {
	return a.router
}

// TODO is a default request handler that can be used for unimplemented endpoints.
func TODO(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "endpoint unimplemented. stay tuned!", http.StatusNotImplemented)
}
