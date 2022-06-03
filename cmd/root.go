// Package cmd implements commands for the processor executable.
package cmd

import (
	"fmt"
	"os"
	"sync"

	"github.com/spf13/cobra"

	"github.com/oasislabs/oasis-indexer/cmd/analyzer"
	"github.com/oasislabs/oasis-indexer/cmd/api"
	"github.com/oasislabs/oasis-indexer/cmd/common"
	"github.com/oasislabs/oasis-indexer/cmd/generator"
	"github.com/oasislabs/oasis-indexer/config"
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
		os.Exit(1)
	}

	// Initialize common environment.
	if err = common.Init(cfg); err != nil {
		os.Exit(1)
	}
	logger := common.Logger()

	// Initialize services.
	analysisService, err := analyzer.Init(cfg.Analysis)
	if err != nil {
		os.Exit(1)
	}
	apiService, err := api.Init(cfg.Server)
	if err != nil {
		os.Exit(1)
	}

	var wg sync.WaitGroup
	for _, service := range []Service{
		analysisService,
		apiService,
	} {
		wg.Add(1)
    go func() {
      defer wg.Done()
      service.Start()
    }()
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
		generator.Register,
	} {
		f(rootCmd)
	}
}
