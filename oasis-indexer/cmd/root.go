// Package cmd implements commands for the processor executable.
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/oasislabs/oasis-indexer/oasis-indexer/cmd/analyzer"
	"github.com/oasislabs/oasis-indexer/oasis-indexer/cmd/api"
	"github.com/oasislabs/oasis-indexer/oasis-indexer/cmd/common"
	"github.com/oasislabs/oasis-indexer/oasis-indexer/cmd/generator"
)

var rootCmd = &cobra.Command{
	Use:   "oasis-indexer",
	Short: "Oasis Indexer",
	Run:   rootMain,
}

func rootMain(cmd *cobra.Command, args []string) {
	if err := common.Init(); err != nil {
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
	common.RegisterFlags(rootCmd)

	for _, f := range []func(*cobra.Command){
		analyzer.Register,
		api.Register,
		generator.Register,
	} {
		f(rootCmd)
	}
}
