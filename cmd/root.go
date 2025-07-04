// Package cmd implements commands for the processor executable.
package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/oasisprotocol/nexus/cmd/analyzer"
	"github.com/oasisprotocol/nexus/cmd/api"
	"github.com/oasisprotocol/nexus/cmd/common"
	"github.com/oasisprotocol/nexus/config"
	"github.com/oasisprotocol/nexus/log"
	"github.com/oasisprotocol/nexus/metrics"
)

const (
	analyzeCmdName = "analyze"
	serveCmdName   = "serve"

	// Very long shutdown timeout because consensus pogreb datastore sometimes takes a long time to
	// close. Reduce this in future.
	// https://github.com/oasisprotocol/nexus/issues/1034
	shutdownTimeout = 30 * time.Minute
)

var (
	// Path to the configuration file.
	configFile string

	rootCmd = &cobra.Command{
		Use:   "nexus",
		Short: "Oasis Nexus",
		Run:   rootMain,
	}

	analyzerCmd = &cobra.Command{
		Use:   analyzeCmdName,
		Short: "Run Nexus in analyzer-only mode",
		Run:   rootMain,
	}

	apiCmd = &cobra.Command{
		Use:   serveCmdName,
		Short: "Run Nexus in API-only mode",
		Run:   rootMain,
	}
)

// Service is a service run by Nexus.
type Service interface {
	// Run runs the service.
	Run(ctx context.Context) error
}

func rootMain(cmd *cobra.Command, args []string) {
	ctx, cancel := context.WithCancel(context.Background())

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

	switch cmd.Name() {
	case analyzeCmdName:
		// For backward compatibility: ignore server config even if present.
		// In the future, this should be unified under a single command where
		// the config file defines which services to run.
		cfg.Server = nil
	case serveCmdName:
		// For backward compatibility: ignore analysis config even if present.
		// In the future, this should be unified under a single command where
		// the config file defines which services to run.
		cfg.Analysis = nil
	default:
	}

	// Initialize services.
	errCh := make(chan error, 4)
	var wg sync.WaitGroup

	if cfg.Analysis != nil {
		analysisService, err := analyzer.Init(ctx, cfg.Analysis, logger)
		if err != nil {
			logger.Error("failed to initialize analysis service", "err", err)
			os.Exit(1)
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			// If no server is running, stop the process when the analyzer finishes.
			if cfg.Server == nil {
				defer cancel()
			}

			if err := analysisService.Run(ctx); err != nil {
				errCh <- fmt.Errorf("analysis: %w", err)
			}
		}()
	}

	if cfg.Server != nil {
		apiService, err := api.Init(ctx, cfg.Server, logger)
		if err != nil {
			logger.Error("failed to initialize api service", "err", err)
			os.Exit(1)
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := apiService.Run(ctx); err != nil {
				errCh <- fmt.Errorf("api: %w", err)
			}
		}()
	}

	if cfg.Metrics != nil {
		// Initialize Prometheus service.
		promServer, err := metrics.NewPullService(cfg.Metrics.PullEndpoint, logger)
		if err != nil {
			logger.Error("failed to initialize metrics", "err", err)
			os.Exit(1)
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := promServer.Run(ctx); err != nil {
				errCh <- fmt.Errorf("metrics: %w", err)
			}
		}()
	}

	if cfg.Metrics != nil && cfg.Metrics.PprofEndpoint != nil {
		// Initialize pprof endpoint.
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := common.PprofRun(ctx, *cfg.Metrics.PprofEndpoint, logger); err != nil {
				errCh <- fmt.Errorf("pprof: %w", err)
			}
		}()
	}

	logger.Info("started all services")

	// Ensure clean shutdown.
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(stop)
	select {
	case err := <-errCh:
		logger.Error("service stopped", "err", err)
	case <-stop:
		logger.Info("received signal to stop")
	case <-ctx.Done():
		logger.Info("context done")
	}
	cancel()

	logger.Info("waiting for services to shutdown")

	// Wait for services to shutdown.
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		logger.Info("services shut down")
	case <-time.After(shutdownTimeout):
		logger.Error("services did not shutdown in time, forcing exit")
		os.Exit(1)
	}
}

// Execute spawns the main entry point after handing the config file.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&configFile, "config", "./conf/server.yml", "path to the config.yml file")

	rootCmd.AddCommand(analyzerCmd)
	rootCmd.AddCommand(apiCmd)
}
