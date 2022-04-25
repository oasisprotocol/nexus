// Package api defines API handlers for the Oasis Indexer API.
package api

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
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

// Handler handles API requests.
type Handler struct {
	db     *stateClient
	router *mux.Router
}

// NewHandler creates a new API handler.
func NewHandler(db storage.TargetStorage) *Handler {
	baseRouter := mux.NewRouter()
	apiRouter := baseRouter.PathPrefix("/api").Subrouter()

	h := &Handler{
		db:     makeStateClient(db),
		router: apiRouter,
	}
	h.registerEndpoints()

	return h
}

// Router gets the router for this Handler.
func (h *Handler) Router() *mux.Router {
	return h.router
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

	r.HandleFunc("/consensus/blocks", h.GetBlocks).Methods("GET")
	r.HandleFunc("/consensus/blocks/{height}", h.GetBlock).Methods("GET")
	r.HandleFunc("/consensus/blocks/{height}/beacon", h.GetBeacon).Methods("GET")
	r.HandleFunc("/consensus/blocks/{height}/events", h.GetBlockEvents).Methods("GET")
	r.HandleFunc("/consensus/transactions", h.GetTransactions).Methods("GET")
	r.HandleFunc("/consensus/transactions/{tx_hash}", h.GetTransaction).Methods("GET")
	r.HandleFunc("/consensus/transactions/{tx_hash}/events", h.GetTransactionEvents).Methods("GET")
}

func (h *Handler) registerRegistryEndpoints() {
	r := h.router

	r.HandleFunc("/consensus/entities", h.GetEntities).Methods("GET")
	r.HandleFunc("/consensus/entities/{entity_id}", h.GetEntity).Methods("GET")
	r.HandleFunc("/consensus/entities/{entity_id}/nodes", h.GetEntityNodes).Methods("GET")
	r.HandleFunc("/consensus/entities/{entity_id}/nodes/{node_id}", h.GetEntityNode).Methods("GET")
}

func (h *Handler) registerStakingEndpoints() {
	r := h.router

	r.HandleFunc("/consensus/accounts", h.GetAccounts).Methods("GET")
	r.HandleFunc("/consensus/accounts/{address}", h.GetAccount).Methods("GET")
}

func (h *Handler) registerSchedulerEndpoints() {
	r := h.router

	r.HandleFunc("/consensus/epochs", h.GetEpochs).Methods("GET")
	r.HandleFunc("/consensus/epochs/{epoch}", h.GetEpoch).Methods("GET")
	r.HandleFunc("/consensus/epochs/{epoch}/validators", h.GetValidators).Methods("GET")
	r.HandleFunc("/consensus/epochs/{epoch}/committees/{runtime_id}", h.GetCommittees).Methods("GET")
}

func (h *Handler) registerGovernanceEndpoints() {
	r := h.router

	r.HandleFunc("/consensus/proposals", h.GetProposals).Methods("GET")
	r.HandleFunc("/consensus/proposals/{proposal_id}", h.GetProposal).Methods("GET")
}

// GetStatus gets the latest indexing status of the Oasis Indexer.
func (h *Handler) GetStatus(w http.ResponseWriter, r *http.Request) {
	status := APIStatus{
		CurrentHeight: 8048956,
	}

	var resp []byte
	resp, err := json.Marshal(status)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("content-type", "application/json")
	w.Write(resp)
}

// GetMetadata gets metadata for the Oasis Indexer API.
func (h *Handler) GetMetadata(w http.ResponseWriter, r *http.Request) {
	var resp []byte
	resp, err := json.Marshal(latestMetadata)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("content-type", "application/json")
	w.Write(resp)
}
