// Package api implements the api sub-command.
package api

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/oasislabs/oasis-indexer/api"
	"github.com/oasislabs/oasis-indexer/cmd/common"
	"github.com/oasislabs/oasis-indexer/config"
	"github.com/oasislabs/oasis-indexer/log"
)

const (
	moduleName = "api"
)

var (
	// Path to the configuration file.
	configFile string

	apiCmd = &cobra.Command{
		Use:   "serve",
		Short: "Serve Oasis Indexer API",
		Run:   runServer,
	}
)

func runServer(cmd *cobra.Command, args []string) {
	// Initialize config.
	cfg, err := config.InitConfig(configFile)
	if err != nil {
		fmt.Printf("indexer-api: %s", err.Error())
		os.Exit(1)
	}

	// Initialize common environment.
	if err = common.Init(cfg); err != nil {
		os.Exit(1)
	}
	logger := common.Logger()

	if cfg.Server == nil {
		logger.Error("server config not provided")
		os.Exit(1)
	}

	service, err := Init(cfg.Server)
	if err != nil {
		os.Exit(1)
	}

	service.Start()
}

// Init initializes the API service.
func Init(cfg *config.ServerConfig) (*Service, error) {
	logger := common.Logger()

	service, err := NewService(cfg)
	if err != nil {
		logger.Error("service failed to start",
			"error", err,
		)
		return nil, err
	}
	return service, nil
}

// Service is the Oasis Indexer's API service.
type Service struct {
	server string
	api    *api.IndexerAPI
	logger *log.Logger
}

// NewService creates a new API service.
func NewService(cfg *config.ServerConfig) (*Service, error) {
	logger := common.Logger().WithModule(moduleName)

	// Initialize target storage.
	client, err := common.NewClient(cfg.Storage, logger)
	if err != nil {
		return nil, err
	}

	return &Service{
		server: cfg.Endpoint,
		api:    api.NewIndexerAPI(client, logger),
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
	apiCmd.Flags().StringVar(&configFile, "config", "./config/local.yml", "path to the config.yml file")
	parentCmd.AddCommand(apiCmd)
}
