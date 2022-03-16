// Package analyzer implements the analyzer sub-command.
package analyzer

import (
	"context"
	"errors"
	"os"

	"github.com/spf13/cobra"

<<<<<<< HEAD
	"github.com/oasislabs/oasis-block-indexer/go/oasis-indexer/cmd/common"
=======
	"github.com/oasislabs/oasis-block-indexer/go/log"
	"github.com/oasislabs/oasis-block-indexer/go/oasis-indexer/cmd/common"
	"github.com/oasislabs/oasis-block-indexer/go/storage"
)

const (
	// CfgStorageEndpoint is the flag for setting the connection string to
	// the backing storage.
	CfgStorageEndpoint = "storage.endpoint"

	moduleName = "analysis"
)

var (
	cfgStorageEndpoint string

	analyzeCmd = &cobra.Command{
		Use:   "analyze",
		Short: "Analyze blocks",
		Run:   runAnalyzer,
	}
>>>>>>> b9e9d99 (Remove processor module)
)

func runAnalyzer(cmd *cobra.Command, args []string) {
	common.Init()

	_, err := NewAnalyzer()
	switch {
	case err == nil:
	case errors.Is(err, context.Canceled):
		// Shutdown requested during startup.
		return
	default:
		os.Exit(1)
	}
}

// Analyzer is the Oasis Indexer's analysis service.
type Analyzer struct {
	TargetStorage storage.TargetStorage

	logger *log.Logger
}

// NewAnalyzer creates and starts a new Analyzer
func NewAnalyzer() (*Analyzer, error) {
	logger := common.Logger().WithModule(moduleName)

	analyzer := &Analyzer{
		logger: logger,
	}

	logger.Info("Starting oasis-indexer analysis layer.")

	return analyzer, nil
}

// Register registers the process sub-command.
func Register(parentCmd *cobra.Command) {
	analyzeCmd.Flags().StringVar(&cfgStorageEndpoint, CfgStorageEndpoint, "", "a postgresql-compliant connection url")

	parentCmd.AddCommand(analyzeCmd)
}
