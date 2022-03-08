// Package analyzer implements the analyzer sub-command.
package analyzer

import (
	"github.com/spf13/cobra"

	"github.com/oasislabs/oasis-block-indexer/go/oasis-indexer/cmd/common"
)

var analyzeCmd = &cobra.Command{
	Use:   "analyze",
	Short: "Analyze blocks",
	Run:   runProcessor,
}

func runProcessor(cmd *cobra.Command, args []string) {
	common.Init()
}

// Register registers the process sub-command.
func Register(parentCmd *cobra.Command) {
	parentCmd.AddCommand(analyzeCmd)
}
