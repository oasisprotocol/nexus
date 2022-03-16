// Package cmd implements commands for the processor executable.
package cmd

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/oasislabs/oasis-block-indexer/go/oasis-indexer/cmd/analyzer"
	"github.com/oasislabs/oasis-block-indexer/go/oasis-indexer/cmd/api"
	"github.com/oasislabs/oasis-block-indexer/go/oasis-indexer/cmd/common"
)

var rootCmd = &cobra.Command{
	Use:   "oasis-indexer",
	Short: "Oasis Indexer",
}

// Execute spawns the main entry point after handing the config file.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().AddFlagSet(common.RootFlags)

	for _, f := range []func(*cobra.Command){
		analyzer.Register,
		api.Register,
	} {
		f(rootCmd)
	}
}
