// Package cmd implements commands for the processor executable.
package cmd

import (
	"fmt"
	"os"

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

func rootMain(cmd *cobra.Command, args []string) {
	// Initialize analyzer config.
	cfg, err := config.InitConfig(configFile)
	if err != nil {
		os.Exit(1)
	}

	// Initialize common environment.
	if err := common.Init(cfg); err != nil {
		os.Exit(1)
	}

	// TODO: Start oasis-indexer
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
