package v1

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/oasisprotocol/oasis-indexer/log"
	"github.com/oasisprotocol/oasis-indexer/metrics"
	storage "github.com/oasisprotocol/oasis-indexer/storage/client"
)

const (
	moduleName = "api_v1"
)

// Handler is the Oasis Indexer V1 API handler.
type Handler struct {
	client  *storageClient
	logger  *log.Logger
	metrics metrics.RequestMetrics
}

// NewHandler creates a new V1 API handler.
func NewHandler(chainID string, s *storage.StorageClient, l *log.Logger) *Handler {
	return &Handler{
		client:  newStorageClient(chainID, s, l),
		logger:  l.WithModule(moduleName),
		metrics: metrics.NewDefaultRequestMetrics(moduleName),
	}
}

// RegisterRoutes implements the APIHandler interface.
func (h *Handler) RegisterMiddlewares(r chi.Router) {
	r.Use(h.metricsMiddleware)
	r.Use(h.chainMiddleware)
}

// RegisterRoutes implements the APIHandler interface.
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Route("/v1", func(r chi.Router) {
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
				r.Get("/{address}/delegations", h.GetDelegations)
				r.Get("/{address}/debonding_delegations", h.GetDebondingDelegations)
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

			// Validator Endpoints.
			r.Route("/validators", func(r chi.Router) {
				r.Get("/", h.ListValidators)
				r.Get("/{entity_id}", h.GetValidator)
			})

			// Aggregate Statistics.
			r.Route("/stats", func(r chi.Router) {
				r.Get("/tps", h.ListTransactionsPerSecond)
				r.Get("/daily_volume", h.ListDailyVolume)
			})
		})

		// ParaTime Endpoints.
		r.Route("/emerald", func(r chi.Router) {
			r.Use(h.runtimeMiddleware("emerald"))
			// Block Endpoints.
			r.Route("/blocks", func(r chi.Router) {
				r.Get("/", h.RuntimeListBlocks)
			})
			r.Route("/transactions", func(r chi.Router) {
				r.Get("/", h.RuntimeListTransactions)
			})
			r.Route("/tokens", func(r chi.Router) {
				r.Get("/", h.RuntimeListTokens)
			})
		})

		// API specs.
		r.Route("/spec", func(r chi.Router) {
			specServer := http.FileServer(http.Dir("api/spec"))
			r.Handle("/*", http.StripPrefix("/v1/spec", specServer))
		})
	})
}

// Name implements the APIHandler interface.
func (h *Handler) Name() string {
	return "v1"
}
