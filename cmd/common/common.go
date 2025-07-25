// Package common implements common nexus command options.
package common

import (
	"context"
	"errors"
	"fmt"
	"io"
	stdLog "log"
	"net/http"
	"os"
	"time"

	"github.com/akrylysov/pogreb"
	coreLogging "github.com/oasisprotocol/oasis-core/go/common/logging"

	"github.com/oasisprotocol/nexus/config"
	"github.com/oasisprotocol/nexus/log"
	"github.com/oasisprotocol/nexus/storage"
	"github.com/oasisprotocol/nexus/storage/postgres"
)

const serverShutdownTimeout = 10 * time.Second

var rootLogger = log.NewDefaultLogger("nexus")

// Init initializes the common environment.
func Init(cfg *config.Config) error {
	var w io.Writer = os.Stdout
	format := log.FmtJSON
	level := log.LevelDebug
	coreFormat := coreLogging.FmtJSON   // For oasis-core.
	coreLevel := coreLogging.LevelDebug // For oasis-core.

	// Initialize nexus logging.
	if cfg.Log != nil {
		var err error
		if w, err = getLoggingStream(cfg.Log); err != nil {
			return fmt.Errorf("opening log file: %w", err)
		}
		if err := format.Set(cfg.Log.Format); err != nil {
			return err
		}
		if err := level.Set(cfg.Log.Level); err != nil {
			return err
		}
	}
	logger, err := log.NewLogger("nexus", w, format, level)
	if err != nil {
		return err
	}
	rootLogger = logger

	// Initialize oasis-core logging. Useful for low-level gRPC issues.
	if err := coreLogging.Initialize(w, coreFormat, coreLevel, nil); err != nil {
		logger.Error("failed to initialize oasis-core logging", "err", err)
		return err
	}

	// Initialize pogreb logging.
	pogrebLogger := RootLogger().WithModule("pogreb").WithCallerUnwind(7)
	pogreb.SetLogger(stdLog.New(log.WriterIntoLogger(*pogrebLogger), "", 0))

	return nil
}

// RootLogger returns the logger defined by logging flags.
func RootLogger() *log.Logger {
	return rootLogger
}

func getLoggingStream(cfg *config.LogConfig) (io.Writer, error) {
	if cfg == nil || cfg.File == "" {
		return os.Stdout, nil
	}
	w, err := os.OpenFile(cfg.File, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, err
	}
	return w, nil
}

// NewClient creates a new client to target storage.
func NewClient(cfg *config.StorageConfig, logger *log.Logger) (storage.TargetStorage, error) {
	var backend config.StorageBackend
	if err := backend.Set(cfg.Backend); err != nil {
		return nil, err
	}

	var client storage.TargetStorage
	var err error
	switch backend {
	case config.BackendPostgres:
		client, err = postgres.NewClient(cfg.Endpoint, logger)
	default:
		panic(fmt.Sprintf("unsupported storage backend: %v", backend))
	}
	if err != nil {
		return nil, err
	}

	return client, nil
}

// RunServer is a context-aware wrapper around http.Server.ListenAndServe.
func RunServer(ctx context.Context, server *http.Server, logger *log.Logger) error {
	errCh := make(chan error, 1)
	go func() {
		logger.Info("starting server", "address", server.Addr)
		if err := server.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			errCh <- fmt.Errorf("server error: %w", err)
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		logger.Info("shutting down server", "timeout", serverShutdownTimeout)
		ctx, cancel := context.WithTimeout(context.Background(), serverShutdownTimeout)
		defer cancel()
		return server.Shutdown(ctx)
	}
}
