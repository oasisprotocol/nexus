// Package api implements the api sub-command.
package api

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"

	sdkConfig "github.com/oasisprotocol/oasis-sdk/client-sdk/go/config"

	api "github.com/oasisprotocol/nexus/api"
	v1 "github.com/oasisprotocol/nexus/api/v1"
	apiTypes "github.com/oasisprotocol/nexus/api/v1/types"
	cmdCommon "github.com/oasisprotocol/nexus/cmd/common"
	"github.com/oasisprotocol/nexus/common"
	"github.com/oasisprotocol/nexus/config"
	"github.com/oasisprotocol/nexus/log"
	"github.com/oasisprotocol/nexus/metrics"
	storage "github.com/oasisprotocol/nexus/storage/client"
	source "github.com/oasisprotocol/nexus/storage/oasis"
	"github.com/oasisprotocol/nexus/storage/oasis/nodeapi"
)

const (
	moduleName = "api"
	// The path portion with which all v1 API endpoints start.
	v1BaseURL = "/v1"

	defaultRequestHandleTimeout = 20 * time.Second
)

// safeFileSystem is a wrapper around `http.FileServer` that only serves
// regular files (and not symlinks, directories, etc.).
type safeFileSystem struct {
	fs http.Dir
}

// Open implements `http.FileSystem`.
func (sfs safeFileSystem) Open(path string) (http.File, error) {
	// http.Dir already handles directory traversal.
	f, err := sfs.fs.Open(path)
	if err != nil {
		return nil, err
	}

	// Use `os.Lstat` since it doesn't follow symlinks.
	info, err := os.Lstat(filepath.Join(string(sfs.fs), path))
	if err != nil {
		_ = f.Close()
		return nil, err
	}
	if !info.Mode().IsRegular() {
		_ = f.Close()
		return nil, os.ErrPermission
	}

	return f, nil
}

// specFileServer is a wrapper around `http.FileServer` that
// serves files from `rootDir`, and also hardcodes the MIME type for
// YAML files to `application/x-yaml`. The latter is a hack to
// make the HTTP headers independent of the OS's MIME type database.
type specFileServer struct{ rootDir string }

func (srv specFileServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !utf8.ValidString(r.URL.Path) {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}
	if strings.ContainsRune(r.URL.Path, '\x00') {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}
	if len(r.URL.Path) > 10000 {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}
	if strings.HasSuffix(r.URL.Path, ".yaml") || strings.HasSuffix(r.URL.Path, ".yml") {
		w.Header().Set("Content-Type", "application/x-yaml")
	}
	// "api/spec" is the local path from which we serve the files.
	http.FileServer(safeFileSystem{fs: http.Dir(srv.rootDir)}).ServeHTTP(w, r)
}

// Init initializes the API service.
func Init(ctx context.Context, cfg *config.ServerConfig, logger *log.Logger) (*Service, error) {
	logger = logger.WithModule(moduleName)
	service, err := NewService(ctx, cfg, logger)
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

	requestTimeout time.Duration
}

// NewService creates a new API service.
func NewService(ctx context.Context, cfg *config.ServerConfig, logger *log.Logger) (*Service, error) {
	// Initialize target storage.
	backing, err := cmdCommon.NewClient(cfg.Storage, logger)
	if err != nil {
		return nil, err
	}

	// Runtime clients.
	runtimeClients := make(map[common.Runtime]nodeapi.RuntimeApiLite)
	var networkConfig *sdkConfig.Network
	referenceSwaps := cfg.Source.ReferenceSwaps()
	networkConfig = cfg.Source.SDKNetwork()
	apiRuntimes := []common.Runtime{common.RuntimeEmerald, common.RuntimeSapphire, common.RuntimePontusxTest, common.RuntimePontusxDev}
	for _, runtime := range apiRuntimes {
		client, err2 := source.NewRuntimeClient(ctx, cfg.Source, runtime)
		if err2 != nil {
			logger.Warn("unable to instantiate runtime client for api server", "runtime", runtime, "err", err2)
		}
		runtimeClients[runtime] = client
	}

	client, err := storage.NewStorageClient(
		cfg,
		backing,
		referenceSwaps,
		runtimeClients,
		networkConfig,
		logger,
	)
	if err != nil {
		return nil, err
	}

	timeout := defaultRequestHandleTimeout
	if cfg.RequestTimeout != nil {
		timeout = *cfg.RequestTimeout
	}

	return &Service{
		address:        cfg.Endpoint,
		target:         client,
		logger:         logger,
		requestTimeout: timeout,
	}, nil
}

// Start starts the API service.
func (s *Service) Run(ctx context.Context) error {
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
			RequestErrorHandlerFunc:  api.HumanReadableJsonErrorHandler(*s.logger),
			ResponseErrorHandlerFunc: api.HumanReadableJsonErrorHandler(*s.logger),
		},
	)

	// The top-level chi handler.
	handler := apiTypes.HandlerWithOptions(
		strictHandler,
		apiTypes.ChiServerOptions{
			BaseURL:          v1BaseURL,
			Middlewares:      []apiTypes.MiddlewareFunc{},
			BaseRouter:       baseRouter,
			ErrorHandlerFunc: api.HumanReadableJsonErrorHandler(*s.logger),
		})
	// By default, request context is not cancelled when write timeout is reached. The connection
	// is closed, but the handler continues to run. Ref: https://github.com/golang/go/issues/59602
	// We use `http.TimeoutHandler`, to cancel requests and return a 503 to the client when timeout is reached.
	// The handler also cancels the downstream request context on timeout.
	handler = http.TimeoutHandler(handler, s.requestTimeout, "request timed out")
	// Manually apply the CORS middleware; we want it to run always.
	// HandlerWithOptions() above does not apply it to some requests (404 URLs, requests with bad params, etc.).
	handler = api.CorsMiddleware(handler)
	// Manually apply the metrics middleware; we want it to run always, and at the outermost layer.
	// HandlerWithOptions() above does not apply it to some requests (404 URLs, requests with bad params, etc.).
	handler = api.MetricsMiddleware(metrics.NewDefaultRequestMetrics(moduleName), *s.logger)(handler)

	server := &http.Server{
		Addr:           s.address,
		Handler:        handler,
		ReadTimeout:    5 * time.Second,
		WriteTimeout:   s.requestTimeout + 5*time.Second, // Should be longer than the request handling timeout.
		MaxHeaderBytes: 1 << 20,                          // 1MB.
	}

	return cmdCommon.RunServer(ctx, server, s.logger)
}

// cleanup gracefully shuts down the service.
func (s *Service) cleanup() {
	s.target.Shutdown()
}
