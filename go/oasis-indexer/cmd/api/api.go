// Package api implements the api sub-command.
package api

import (
	"github.com/spf13/cobra"

	"github.com/oasislabs/oasis-block-indexer/go/oasis-indexer/cmd/common"
)

var apiCmd = &cobra.Command{
	Use:   "serve",
	Short: "Serve Oasis Indexer API",
	Run:   runServer,
}

func runServer(cmd *cobra.Command, args []string) {
	common.Init()
}

// Register registers the process sub-command.
func Register(parentCmd *cobra.Command) {
	parentCmd.AddCommand(apiCmd)
}
