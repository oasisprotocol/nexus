// Package api defines API handlers for the Oasis Indexer API.
package api

import (
	"net/http"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"

	"github.com/oasislabs/oasis-block-indexer/go/storage"
)

const (
	LatestChainID = "oasis-3"

	moduleName = "api.handler"
)

// Handler handles API requests.
type Handler struct {
	db     storage.TargetStorage
	router *chi.Mux
}

// NewHandler creates a new API handler.
func NewHandler(db storage.TargetStorage) *Handler {
	r := chi.NewRouter()

	// TODO: Replace with one of our structured loggers.
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(chainMiddleware)

	h := &Handler{
		db:     db,
		router: r,
	}
	h.registerEndpoints()

	return h
}

func (h *Handler) registerEndpoints() {
	r := h.router

	// Status endpoints.
	r.Get("/", h.GetStatus)

	// Consensus Endpoints.
	r.Route("/consensus", func(r chi.Router) {

		// Block Endpoints.
		r.Route("/blocks", func(r chi.Router) {
			r.Get("/", h.ListBlocks)
			r.Get("/{height}", h.GetBlock)
		})
		r.Route("/transactions", func(r chi.Router) {
			r.Get("/", h.ListTransactions)
			r.Get("/{tx_hash}", h.GetTransaction)
		})

		// Registry Endpoints.
		r.Route("/entities", func(r chi.Router) {
			r.Get("/", h.ListEntities)
			r.Get("/{entity_id}", h.GetEntity)
			r.Get("/{entity_id}/nodes", h.ListEntityNodes)
			r.Get("/{entity_id}/nodes/{node_id}", h.GetEntityNode)
		})

		// Staking Endpoints.
		r.Route("/accounts", func(r chi.Router) {
			r.Get("/", h.ListAccounts)
			r.Get("/{address}", h.GetAccount)
		})

		// Scheduler Endpoints.
		r.Route("/epochs", func(r chi.Router) {
			r.Get("/", h.TODO)
			r.Get("/{epoch}", h.TODO)
		})

		// Governance Endpoints.
		r.Route("/proposals", func(r chi.Router) {
			r.Get("/", h.ListProposals)
			r.Get("/{proposal_id}", h.GetProposal)
			r.Get("/{proposal_id}/votes", h.GetProposalVotes)
		})
	})

	// ... ParaTime Endpoint Registration.
}

// TODO is a default request handler that can be used for unimplemented endpoints.
func (h *Handler) TODO(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "endpoint unimplemented. stay tuned!", http.StatusNotImplemented)
}

// Router gets the router for this Handler.
func (h *Handler) Router() *chi.Mux {
	return h.router
}
