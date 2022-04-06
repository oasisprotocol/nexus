// Package analyzer implements the analyzer sub-command.
package analyzer

import (
	"context"
	"errors"
	"io/ioutil"
	"os"

	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/config"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"

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
	CfgNetworkFile     = "network.config"

	moduleName = "analysis"
)

var (
	cfgStorageEndpoint string
	cfgNetworkFile     string

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

	analyzer, err := NewAnalyzer()
	switch {
	case err == nil:
		analyzer.Run()
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
	logger        *log.Logger
}

// NewAnalyzer creates and starts a new Analyzer
func NewAnalyzer() (*Analyzer, error) {
	ctx := context.Background()
	logger := common.Logger().WithModule(moduleName)

	// Initialize source storage.
	rawCfg, err := ioutil.ReadFile(cfgNetworkFile)
	if err != nil {
		return nil, err
	}
	var networkCfg config.Network
	if err := yaml.Unmarshal([]byte(rawCfg), &networkCfg); err != nil {
		return nil, err
	}
	oasisNodeClient, err := source.NewOasisNodeClient(ctx, &networkCfg)
	if err != nil {
		return nil, err
	}

	// Initialize target storage.
	cockroachClient, err := target.NewCockroachClient(cfgStorageEndpoint)
	if err != nil {
		return nil, err
	}

	return &Analyzer{
		SourceStorage: oasisNodeClient,
		TargetStorage: cockroachClient,

		logger: logger,
	}, nil
}

func (a *Analyzer) Run() {
	ctx := context.Background()

	initialHeight := int64(3027601)
	height := initialHeight + 1

	for height < initialHeight+10 {
		if err := a.processBlock(ctx, height); err != nil {
			a.logger.Warn("failed to process block", "height", height, "err", err)
			continue
		}

		height += 1
	}
}

func (a *Analyzer) processBlock(ctx context.Context, height int64) error {
	blockData, err := a.SourceStorage.BlockData(ctx, height)
	if err != nil {
		return err
	}
	if err := a.TargetStorage.SetBlockData(ctx, blockData); err != nil {
		return err
	}

	return nil
}

// Register registers the process sub-command.
func Register(parentCmd *cobra.Command) {
	analyzeCmd.Flags().StringVar(&cfgStorageEndpoint, CfgStorageEndpoint, "", "a postgresql-compliant connection url")
	analyzeCmd.Flags().StringVar(&cfgNetworkFile, CfgNetworkFile, "", "path to a network configuration file")

	parentCmd.AddCommand(analyzeCmd)
}
