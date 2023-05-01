// Package cmd implements commands for the processor executable.
package cmd

import (
	"fmt"
	"os"
	"sync"

	"github.com/spf13/cobra"

	"github.com/oasisprotocol/oasis-indexer/cmd/analyzer"
	"github.com/oasisprotocol/oasis-indexer/cmd/api"
	"github.com/oasisprotocol/oasis-indexer/cmd/common"
	"github.com/oasisprotocol/oasis-indexer/config"
	"github.com/oasisprotocol/oasis-indexer/log"
)

var (
	// Path to the configuration file.
	configFile string

	rootCmd = &cobra.Command{
		Use:   "oasis-indexer",
		Short: "Oasis Indexer",
		Run:   rootMain,
	}
)

// Service is a service run by the indexer.
type Service interface {
	// Start starts the service.
	Start()
}

func rootMain(cmd *cobra.Command, args []string) {
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

	// Initialize services.
	var wg sync.WaitGroup
	runInWG := func(s Service) {
		wg.Add(1)
		go func(s Service) {
			defer wg.Done()
			s.Start()
		}(s)
	}

	if cfg.Analysis != nil {
		analysisService, err := analyzer.Init(cfg.Analysis)
		if err != nil {
			logger.Error("failed to initialize analysis service", "err", err)
			os.Exit(1)
		}
		runInWG(analysisService)
	}
	if cfg.Server != nil {
		apiService, err := api.Init(cfg.Server)
		if err != nil {
			logger.Error("failed to initialize api service", "err", err)
			os.Exit(1)
		}
		runInWG(apiService)
	}

	logger.Info("started all services")
	wg.Wait()
}

// Execute spawns the main entry point after handing the config file.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.Flags().StringVar(&configFile, "config", "./conf/server.yml", "path to the config.yml file")

	for _, f := range []func(*cobra.Command){
		analyzer.Register,
		api.Register,
	} {
		f(rootCmd)
	}
}
