// Package analyzer implements the analyzer sub-command.
package analyzer

import (
	"context"
	"errors"
	"os"

	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/config"
	"github.com/spf13/cobra"

	"github.com/oasislabs/oasis-block-indexer/go/log"
	"github.com/oasislabs/oasis-block-indexer/go/oasis-indexer/cmd/common"
	"github.com/oasislabs/oasis-block-indexer/go/storage"
	target "github.com/oasislabs/oasis-block-indexer/go/storage/cockroach"
	source "github.com/oasislabs/oasis-block-indexer/go/storage/oasis"
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
)

func runAnalyzer(cmd *cobra.Command, args []string) {
	if err := common.Init(); err != nil {
		os.Exit(1)
	}

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
	SourceStorage storage.SourceStorage
	TargetStorage storage.TargetStorage

	logger *log.Logger
}

// NewAnalyzer creates and starts a new Analyzer
func NewAnalyzer() (*Analyzer, error) {
	ctx := context.Background()
	logger := common.Logger().WithModule(moduleName)

	var network config.Network
	oasisNodeClient, err := source.NewOasisNodeClient(ctx, &network)
	if err != nil {
		return nil, err
	}

	cockroachClient, err := target.NewCockroachClient("connstring")
	if err != nil {
		return nil, err
	}

	analyzer := &Analyzer{
		SourceStorage: oasisNodeClient,
		TargetStorage: cockroachClient,
		logger:        logger,
	}

	logger.Info("Starting oasis-indexer analysis layer.")

	return analyzer, nil
}

// Register registers the process sub-command.
func Register(parentCmd *cobra.Command) {
	analyzeCmd.Flags().StringVar(&cfgStorageEndpoint, CfgStorageEndpoint, "", "a postgresql-compliant connection url")

	parentCmd.AddCommand(analyzeCmd)
}
