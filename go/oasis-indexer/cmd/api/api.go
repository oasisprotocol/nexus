// Package api implements the api sub-command.
package api

import (
	"context"
	"errors"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/mux"
	"github.com/spf13/cobra"

	"github.com/oasislabs/oasis-block-indexer/go/api"
	consensusApi "github.com/oasislabs/oasis-block-indexer/go/api/consensus"
	"github.com/oasislabs/oasis-block-indexer/go/log"
	"github.com/oasislabs/oasis-block-indexer/go/oasis-indexer/cmd/common"
)

const (
	// CfgServiceEndpoint is the service endpoint at which the Oasis Indexer API
	// can be reached.
	CfgServiceEndpoint = "api.service_endpoint"

	moduleName = "api"
)

var (
	cfgServiceEndpoint string

	apiCmd = &cobra.Command{
		Use:   "serve",
		Short: "Serve Oasis Indexer API",
		Run:   runServer,
	}
)

func runServer(cmd *cobra.Command, args []string) {
	if err := common.Init(); err != nil {
		os.Exit(1)
	}

	service, err := NewAPIService()
	switch {
	case err == nil:
		service.Start()
	case errors.Is(err, context.Canceled):
		// Shutdown requested during startup.
		return
	default:
		common.Logger().Error("service failed to start",
			"error", err,
		)
		os.Exit(1)
	}
}

// APIService is the Oasis Indexer's API service.
type APIService struct {
	server string
	router *mux.Router
	logger *log.Logger
}

// NewAPIService creates a new API service.
func NewAPIService() (*APIService, error) {
	logger := common.Logger().WithModule(moduleName)

	baseRouter := mux.NewRouter()
	apiRouter := baseRouter.PathPrefix("/api").Subrouter()

	apiService := &APIService{
		server: cfgServiceEndpoint,
		router: apiRouter,
		logger: logger,
	}

	// Register endpoints.
	for _, f := range []func(*mux.Router){
		apiService.registerBlockEndpoints,
		apiService.registerRegistryEndpoints,
		apiService.registerStakingEndpoints,
		apiService.registerSchedulerEndpoints,
		apiService.registerGovernanceEndpoints,
	} {
		f(apiRouter)
	}
	apiRouter.HandleFunc("/", api.GetMetadata).Methods("GET")
	apiRouter.HandleFunc("/status", consensusApi.GetStatus).Methods("GET")

	return apiService, nil
}

func (s *APIService) registerBlockEndpoints(r *mux.Router) {
	r.HandleFunc("/consensus/blocks", consensusApi.GetBlocks).Methods("GET")
	r.HandleFunc("/consensus/blocks/{height}", consensusApi.GetBlock).Methods("GET")
	r.HandleFunc("/consensus/blocks/{height}/beacon", consensusApi.GetBeacon).Methods("GET")
	r.HandleFunc("/consensus/blocks/{height}/events", consensusApi.GetBlockEvents).Methods("GET")
	r.HandleFunc("/consensus/transactions", consensusApi.GetTransactions).Methods("GET")
	r.HandleFunc("/consensus/transactions/{tx_hash}", consensusApi.GetTransaction).Methods("GET")
	r.HandleFunc("/consensus/transactions/{tx_hash}/events", consensusApi.GetTransactionEvents).Methods("GET")
}

func (s *APIService) registerRegistryEndpoints(r *mux.Router) {
	r.HandleFunc("/consensus/entities", consensusApi.GetEntities).Methods("GET")
	r.HandleFunc("/consensus/entities/{entity_id}", consensusApi.GetEntity).Methods("GET")
	r.HandleFunc("/consensus/entities/{entity_id}/nodes", consensusApi.GetEntityNodes).Methods("GET")
	r.HandleFunc("/consensus/entities/{entity_id}/nodes/{node_id}", consensusApi.GetEntityNode).Methods("GET")
}

func (s *APIService) registerStakingEndpoints(r *mux.Router) {
	r.HandleFunc("/consensus/accounts", consensusApi.GetAccounts).Methods("GET")
	r.HandleFunc("/consensus/accounts/{address}", consensusApi.GetAccount).Methods("GET")
}

func (s *APIService) registerSchedulerEndpoints(r *mux.Router) {
	r.HandleFunc("/consensus/epochs", consensusApi.GetEpochs).Methods("GET")
	r.HandleFunc("/consensus/epochs/{epoch}", consensusApi.GetEpoch).Methods("GET")
	r.HandleFunc("/consensus/epochs/{epoch}/validators", consensusApi.GetValidators).Methods("GET")
	r.HandleFunc("/consensus/epochs/{epoch}/committees/{runtime_id}", consensusApi.GetCommittees).Methods("GET")
}

func (s *APIService) registerGovernanceEndpoints(r *mux.Router) {
	r.HandleFunc("/consensus/proposals", consensusApi.GetProposals).Methods("GET")
	r.HandleFunc("/consensus/proposals/{proposal_id}", consensusApi.GetProposal).Methods("GET")
}

// Start starts the API service.
func (s *APIService) Start() {
	s.logger.Info("starting api service")

	server := &http.Server{
		Addr:           s.server,
		Handler:        s.router,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}

	s.logger.Error("shutting down",
		"error", server.ListenAndServe(),
	)
}

// Register registers the process sub-command.
func Register(parentCmd *cobra.Command) {
	apiCmd.Flags().StringVar(&cfgServiceEndpoint, CfgServiceEndpoint, "localhost:8008", "service endpoint from which to serve indexer api")
	parentCmd.AddCommand(apiCmd)
}
