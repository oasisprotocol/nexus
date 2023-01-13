// Package api implements the api sub-command.
package api

import (
	"encoding/json"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/spf13/cobra"

	v1 "github.com/oasisprotocol/oasis-indexer/api/v1"
	apiTypes "github.com/oasisprotocol/oasis-indexer/api/v1/types"
	"github.com/oasisprotocol/oasis-indexer/cmd/common"
	"github.com/oasisprotocol/oasis-indexer/config"
	"github.com/oasisprotocol/oasis-indexer/log"
	"github.com/oasisprotocol/oasis-indexer/metrics"
	storage "github.com/oasisprotocol/oasis-indexer/storage/client"
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
		log.NewDefaultLogger("init").Error("init failed",
			"error", err,
		)
		os.Exit(1)
	}

	// Initialize common environment.
	if err = common.Init(cfg); err != nil {
		log.NewDefaultLogger("init").Error("init failed",
			"error", err,
		)
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
	defer service.Shutdown()

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
	server  string
	chainID string
	target  *storage.StorageClient
	logger  *log.Logger
}

// NewService creates a new API service.
func NewService(cfg *config.ServerConfig) (*Service, error) {
	logger := common.Logger().WithModule(moduleName)

	// Initialize target storage.
	backing, err := common.NewClient(cfg.Storage, logger)
	if err != nil {
		return nil, err
	}
	client, err := storage.NewStorageClient(cfg.ChainID, backing, logger)
	if err != nil {
		return nil, err
	}

	return &Service{
		server:  cfg.Endpoint,
		chainID: cfg.ChainID,
		target:  client,
		logger:  logger,
	}, nil
}

// Start starts the API service.
func (s *Service) Start() {
	s.logger.Info("starting api service at " + s.server)

	// prepare middlewares for installing later
	middlewares := []apiTypes.MiddlewareFunc{
		v1.ChainMiddleware(s.chainID),
		v1.MetricsMiddleware(metrics.NewDefaultRequestMetrics(moduleName), *s.logger),
		v1.RuntimeFromURLMiddleware,
	}

	// prepare static routes
	r := chi.NewRouter()
	r.Route("/v1/spec", func(r chi.Router) {
		specServer := http.FileServer(http.Dir("api/spec"))
		r.Handle("/*", http.StripPrefix("/v1/spec", specServer))
	})

	strictHandler := apiTypes.NewStrictHandlerWithOptions(
		v1.NewStrictServerImpl(*s.target, *s.logger),
		[]apiTypes.StrictMiddlewareFunc{
			v1.FixDefaultsAndLimitsMiddleware,
			v1.ParseBigIntParamsMiddleware,
		},
		apiTypes.StrictHTTPServerOptions{
			// TODO: flesh these out
			RequestErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, err error) {
				msg := err.Error()
				jsonErr, _ := json.Marshal(apiTypes.HumanReadableError{Msg: msg})
				http.Error(w, string(jsonErr), http.StatusInternalServerError)
			},
			ResponseErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, err error) {
				msg := err.Error()
				jsonErr, _ := json.Marshal(apiTypes.HumanReadableError{Msg: msg})
				http.Error(w, string(jsonErr), http.StatusInternalServerError)
			},
		},
	)

	experimentalHandler := apiTypes.HandlerWithOptions(
		strictHandler, // v1.NewFoo(s.api.V1Handler.Client, s.logger, s.api.V1Handler.Metrics),
		apiTypes.ChiServerOptions{
			BaseURL:     "/v1",
			Middlewares: middlewares,
			BaseRouter:  r,
		})

	server := &http.Server{
		Addr:           s.server,
		Handler:        experimentalHandler, // s.api.Router(),
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}

	s.logger.Error("shutting down",
		"error", server.ListenAndServe(),
	)
}

// Shutdown gracefully shuts down the service.
func (s *Service) Shutdown() {
	s.target.Shutdown()
}

// Register registers the process sub-command.
func Register(parentCmd *cobra.Command) {
	apiCmd.Flags().StringVar(&configFile, "config", "./config/local.yml", "path to the config.yml file")
	parentCmd.AddCommand(apiCmd)
}
