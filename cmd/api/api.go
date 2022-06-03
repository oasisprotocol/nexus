// Package api implements the api sub-command.
package api

import (
	"net/http"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/oasislabs/oasis-indexer/api"
	"github.com/oasislabs/oasis-indexer/cmd/common"
	"github.com/oasislabs/oasis-indexer/log"
	target "github.com/oasislabs/oasis-indexer/storage/cockroach"
)

const (
	// CfgServiceEndpoint is the service endpoint at which the Oasis Indexer API
	// can be reached.
	CfgServiceEndpoint = "api.service_endpoint"

	// CfgStorageEndpoint is the flag for setting the connection string to
	// the backing storage.
	CfgStorageEndpoint = "api.storage_endpoint"

	moduleName = "api"
)

var (
	cfgServiceEndpoint string
	cfgStorageEndpoint string

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

	service, err := NewService()
	if err != nil {
		common.Logger().Error("service failed to start",
			"error", err,
		)
		os.Exit(1)
	}

	service.Start()
}

// Service is the Oasis Indexer's API service.
type Service struct {
	server string
	api    *api.IndexerAPI
	logger *log.Logger
}

// NewService creates a new API service.
func NewService() (*Service, error) {
	logger := common.Logger().WithModule(moduleName)

	cockroachClient, err := target.NewClient(cfgStorageEndpoint, logger)
	if err != nil {
		return nil, err
	}

	return &Service{
		server: cfgServiceEndpoint,
		api:    api.NewIndexerAPI(cockroachClient, logger),
		logger: logger,
	}, nil
}

// Start starts the API service.
func (s *Service) Start() {
	s.logger.Info("starting api service")

	server := &http.Server{
		Addr:           s.server,
		Handler:        s.api.Router(),
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
	apiCmd.Flags().StringVar(&cfgStorageEndpoint, CfgStorageEndpoint, "", "a postgresql-compliant connection url")
	apiCmd.Flags().StringVar(&cfgServiceEndpoint, CfgServiceEndpoint, "localhost:8008", "service endpoint from which to serve indexer api")
	parentCmd.AddCommand(apiCmd)
}
