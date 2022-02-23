// Package analyzer implements the analyzer sub-command.
package analyzer

import (
	"github.com/spf13/cobra"
)

var analyzeCmd = &cobra.Command{
	Use:   "analyze",
	Short: "Analyze blocks",
	Run:   runProcessor,
}

func runProcessor(cmd *cobra.Command, args []string) {
	// TODO
}

// Register registers the process sub-command.
func Register(parentCmd *cobra.Command) {
	parentCmd.AddCommand(analyzeCmd)
}
