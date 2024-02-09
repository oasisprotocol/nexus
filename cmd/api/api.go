// Package api implements the api sub-command.
package api

import (
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/spf13/cobra"

	api "github.com/oasisprotocol/nexus/api"
	v1 "github.com/oasisprotocol/nexus/api/v1"
	apiTypes "github.com/oasisprotocol/nexus/api/v1/types"
	"github.com/oasisprotocol/nexus/cmd/common"
	"github.com/oasisprotocol/nexus/config"
	"github.com/oasisprotocol/nexus/log"
	"github.com/oasisprotocol/nexus/metrics"
	storage "github.com/oasisprotocol/nexus/storage/client"
)

const (
	moduleName = "api"
	// The path portion with which all v1 API endpoints start.
	v1BaseURL = "/v1"
)

var (
	// Path to the configuration file.
	configFile string

	apiCmd = &cobra.Command{
		Use:   "serve",
		Short: "Serve Oasis Nexus API",
		Run:   runServer,
	}
)

// specFileServer is a wrapper around `http.FileServer` that
// serves files from `rootDir`, and also hardcodes the MIME type for
// YAML files to `application/x-yaml`. The latter is a hack to
// make the HTTP headers independent of the OS's MIME type database.
type specFileServer struct{ rootDir string }

func (srv specFileServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if strings.HasSuffix(r.URL.Path, ".yaml") || strings.HasSuffix(r.URL.Path, ".yml") {
		w.Header().Set("Content-Type", "application/x-yaml")
	}
	// "api/spec" is the local path from which we serve the files.
	http.FileServer(http.Dir(srv.rootDir)).ServeHTTP(w, r)
}

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
	logger := common.RootLogger()

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
	logger := common.RootLogger()

	service, err := NewService(cfg)
	if err != nil {
		logger.Error("service failed to start",
			"error", err,
		)
		return nil, err
	}
	return service, nil
}

// Service is Oasis Nexus's API service.
type Service struct {
	address string
	target  *storage.StorageClient
	logger  *log.Logger
}

// NewService creates a new API service.
func NewService(cfg *config.ServerConfig) (*Service, error) {
	logger := common.RootLogger().WithModule(moduleName)

	// Initialize target storage.
	backing, err := common.NewClient(cfg.Storage, logger)
	if err != nil {
		return nil, err
	}
	client, err := storage.NewStorageClient(cfg.ChainName, backing, logger)
	if err != nil {
		return nil, err
	}

	return &Service{
		address: cfg.Endpoint,
		target:  client,
		logger:  logger,
	}, nil
}

// HACK HACK HACK
// Temporarily in place for frontend to test the PontusX endpoints.
// Remove once pontusx support is properly implemented.
func rewritePontusxToSapphireMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/v1/pontusx/") {
			r.URL.Path = strings.Replace(r.URL.Path, "/v1/pontusx/", "/v1/sapphire/", 1)
			r.RequestURI = ""
		}
		// Call the next handler in the chain.
		next.ServeHTTP(w, r)
	})
}

// Start starts the API service.
func (s *Service) Start() {
	defer s.cleanup()
	s.logger.Info("starting api service at " + s.address)

	baseRouter := chi.NewRouter()
	// Do something useful at the root URL, rather than return 404.
	baseRouter.Get("/", http.RedirectHandler(v1BaseURL+"/", http.StatusMovedPermanently).ServeHTTP)
	// Routes to static files (openapi spec).
	baseRouter.Route("/v1/spec", func(r chi.Router) {
		r.Handle("/*", http.StripPrefix("/v1/spec", specFileServer{rootDir: "api/spec"}))
	})

	// A "strict handler" that handles the great majority of requests.
	// It is strict in the sense that it enforces input and output types
	// as defined in the OpenAPI spec.
	strictHandler := apiTypes.NewStrictHandlerWithOptions(
		// The "meat" of the API. The rest of `strictHandler` is autogenned code
		// that deals with validation and serialization.
		v1.NewStrictServerImpl(*s.target, *s.logger),
		// Middleware to apply to all requests. These operate on parsed parameters.
		[]apiTypes.StrictMiddlewareFunc{
			api.FixDefaultsAndLimitsMiddleware,
			api.ParseBigIntParamsMiddleware,
		},
		apiTypes.StrictHTTPServerOptions{
			RequestErrorHandlerFunc:  api.HumanReadableJsonErrorHandler,
			ResponseErrorHandlerFunc: api.HumanReadableJsonErrorHandler,
		},
	)

	// The top-level chi handler.
	handler := apiTypes.HandlerWithOptions(
		strictHandler,
		apiTypes.ChiServerOptions{
			BaseURL: v1BaseURL,
			Middlewares: []apiTypes.MiddlewareFunc{
				api.RuntimeFromURLMiddleware(v1BaseURL),
				api.CorsMiddleware,
			},
			BaseRouter:       baseRouter,
			ErrorHandlerFunc: api.HumanReadableJsonErrorHandler,
		})
	// Manually apply the metrics middleware; we want it to run always, and at the outermost layer.
	// HandlerWithOptions() above does not apply it to some requests (404 URLs, requests with bad params, etc.).
	handler = api.MetricsMiddleware(metrics.NewDefaultRequestMetrics(moduleName), *s.logger)(handler)
	handler = rewritePontusxToSapphireMiddleware(handler)

	server := &http.Server{
		Addr:           s.address,
		Handler:        handler,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}

	s.logger.Error("shutting down",
		"error", server.ListenAndServe(),
	)
}

// cleanup gracefully shuts down the service.
func (s *Service) cleanup() {
	s.target.Shutdown()
}

// Register registers the process sub-command.
func Register(parentCmd *cobra.Command) {
	apiCmd.Flags().StringVar(&configFile, "config", "./config/local.yml", "path to the config.yml file")
	parentCmd.AddCommand(apiCmd)
}
