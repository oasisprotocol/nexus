package v1

import (
	"github.com/go-chi/chi"

	"github.com/oasislabs/oasis-indexer/go/log"
	"github.com/oasislabs/oasis-indexer/go/storage"
)

const (
	LatestChainID = "oasis-3"

	moduleName = "api.v1"
)

// Handler is the Oasis Indexer V1 API handler.
type Handler struct {
	client *storageClient
	logger *log.Logger
}

// NewHandler creates a new V1 API handler.
func NewHandler(db storage.TargetStorage, l *log.Logger) *Handler {
	return &Handler{
		client: newStorageClient(db, l),
		logger: l.WithModule(moduleName),
	}
}

// RegisterRoutes implements the APIHandler interface.
func (h *Handler) RegisterMiddlewares(r chi.Router) {
	r.Use(h.loggerMiddleware)
	r.Use(h.chainMiddleware)
}

// RegisterRoutes implements the APIHandler interface.
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Route("/v1", func(r chi.Router) {

		// Status endpoints.
		r.Get("/", h.GetStatus)

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
				r.Get("/", h.ListEpochs)
				r.Get("/{epoch}", h.GetEpoch)
			})

			// Governance Endpoints.
			r.Route("/proposals", func(r chi.Router) {
				r.Get("/", h.ListProposals)
				r.Get("/{proposal_id}", h.GetProposal)
				r.Get("/{proposal_id}/votes", h.GetProposalVotes)
			})
		})
	})

	// ... ParaTime Endpoint Registration.
}

// Name implements the APIHandler interface.
func (h *Handler) Name() string {
	return "v1"
}
