// Package api defines API handlers for the Oasis Indexer API.
package api

import (
	"net/http"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"

	v1 "github.com/oasislabs/oasis-block-indexer/go/api/v1"
	"github.com/oasislabs/oasis-block-indexer/go/log"
	"github.com/oasislabs/oasis-block-indexer/go/storage"
)

const (
	moduleName = "api"
)

// APIHandler is a handler that handles API requests.
type APIHandler interface {
	// RegisterRoutes registers routes for this API Handler
	RegisterRoutes(chi.Router)

	// Name returns the name of this API handler.
	Name() string
}

// IndexerAPI is an API for the Oasis Indexer.
type IndexerAPI struct {
	router   *chi.Mux
	handlers []APIHandler
	logger   *log.Logger
}

// NewIndexerAPI creates a new Indexer API.
func NewIndexerAPI(db storage.TargetStorage, l *log.Logger) *IndexerAPI {
	r := chi.NewRouter()

	v1Handler := v1.NewHandler(db, l)
	handlers := []APIHandler{
		v1Handler,
	}
	for _, handler := range handlers {
		handler.RegisterRoutes(r)
	}

	h := &IndexerAPI{
		router:   r,
		handlers: handlers,
		logger:   l.WithModule(moduleName),
	}
	r.Use(middleware.Recoverer)

	return h
}

// Router gets the router for this Handler.
func (a *IndexerAPI) Router() *chi.Mux {
	return a.router
}

// TODO is a default request handler that can be used for unimplemented endpoints.
func TODO(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "endpoint unimplemented. stay tuned!", http.StatusNotImplemented)
}
