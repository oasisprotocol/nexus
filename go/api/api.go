// Package api defines API handlers for the Oasis Indexer API.
package api

import (
	"net/http"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"

	"github.com/oasislabs/oasis-block-indexer/go/storage"
)

const (
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

	h := &Handler{
		db:     db,
		router: r,
	}
	h.registerEndpoints()

	return h
}

func (h *Handler) registerEndpoints() {
	for _, f := range []func(){
		h.registerBlockEndpoints,
		h.registerRegistryEndpoints,
		h.registerStakingEndpoints,
		h.registerSchedulerEndpoints,
		h.registerGovernanceEndpoints,
	} {
		f()
	}
}

func (h *Handler) registerBlockEndpoints() {
	r := h.router

	r.Get("/consensus/blocks", h.TODO)
	r.Get("/consensus/blocks/{height}", h.TODO)
	r.Get("/consensus/transactions", h.TODO)
	r.Get("/consensus/transactions/{tx_hash}", h.TODO)
}

func (h *Handler) registerRegistryEndpoints() {
	r := h.router

	r.Get("/consensus/entities", h.ListEntities)
	r.Get("/consensus/entities/{entity_id}", h.GetEntity)
	r.Get("/consensus/entities/{entity_id}/nodes", h.GetEntityNodes)
	r.Get("/consensus/entities/{entity_id}/nodes/{node_id}", h.GetEntityNode)
}

func (h *Handler) registerStakingEndpoints() {
	r := h.router

	r.Get("/consensus/accounts", h.ListAccounts)
	r.Get("/consensus/accounts/{address}", h.GetAccount)
}

func (h *Handler) registerSchedulerEndpoints() {
	r := h.router

	r.Get("/consensus/epochs", h.TODO)
	r.Get("/consensus/epochs/{epoch}", h.TODO)
}

func (h *Handler) registerGovernanceEndpoints() {
	r := h.router

	r.Get("/consensus/proposals", h.ListProposals)
	r.Get("/consensus/proposals/{proposal_id}", h.GetProposal)
	r.Get("/consensus/proposals/{proposal_id}/votes", h.GetProposalVotes)
}

// TODO is a default request handler that can be used for unimplemented endpoints.
func (h *Handler) TODO(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "endpoint unimplemented. stay tuned!", http.StatusNotImplemented)
}

// Router gets the router for this Handler.
func (h *Handler) Router() *chi.Mux {
	return h.router
}
