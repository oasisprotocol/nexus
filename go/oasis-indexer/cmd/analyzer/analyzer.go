// Package analyzer implements the analyzer sub-command.
package analyzer

import (
	"context"
	"errors"
	"io/ioutil"
	"os"
	"sync"

	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/config"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"

	"github.com/oasislabs/oasis-block-indexer/go/analyzer"
	"github.com/oasislabs/oasis-block-indexer/go/analyzer/consensus"
	"github.com/oasislabs/oasis-block-indexer/go/log"
	"github.com/oasislabs/oasis-block-indexer/go/oasis-indexer/cmd/common"
	"github.com/oasislabs/oasis-block-indexer/go/storage"
	target "github.com/oasislabs/oasis-block-indexer/go/storage/cockroach"
	"github.com/oasislabs/oasis-block-indexer/go/storage/migrations/generator"
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

	service, err := NewAnalysisService()
	switch {
	case err == nil:
		service.Start()
	case errors.Is(err, context.Canceled):
		// Shutdown requested during startup.
		return
	default:
		os.Exit(1)
	}
}

// AnalysisService is the Oasis Indexer's analysis service.
type AnalysisService struct {
	Analyzers map[string]analyzer.Analyzer

	source storage.SourceStorage
	target storage.TargetStorage
	logger *log.Logger
}

// NewAnalysisService creates and starts a new AnalysisService
func NewAnalysisService() (*AnalysisService, error) {
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

	// TODO: This is just for quick-and-dirty validation
	document, err := oasisNodeClient.GenesisDocument(ctx)
	if err != nil {
		return nil, err
	}
	g := generator.NewMigrationGenerator(logger)
	if err := g.WriteGenesisDocumentMigration("/Users/nikhilsharma/oasis-block-indexer/go/storage/migrations/0001-state-init.sql", document); err != nil {
		return nil, err
	}

	// Initialize target storage.
	cockroachClient, err := target.NewCockroachClient(cfgStorageEndpoint)
	if err != nil {
		return nil, err
	}

	// Initialize analyzers.
	consensusAnalyzer := consensus.NewConsensusAnalyzer(oasisNodeClient, cockroachClient, logger)

	return &AnalysisService{
		Analyzers: map[string]analyzer.Analyzer{
			consensusAnalyzer.Name(): consensusAnalyzer,
		},
		source: oasisNodeClient,
		target: cockroachClient,
		logger: logger,
	}, nil
}

// Start starts the analysis service.
func (a *AnalysisService) Start() {
	var wg sync.WaitGroup

	for _, an := range a.Analyzers {
		wg.Add(1)
		go func(an analyzer.Analyzer) {
			defer wg.Done()
			an.Start()
		}(an)
	}

	wg.Wait()
}

// Register registers the process sub-command.
func Register(parentCmd *cobra.Command) {
	analyzeCmd.Flags().StringVar(&cfgStorageEndpoint, CfgStorageEndpoint, "", "a postgresql-compliant connection url")
	analyzeCmd.Flags().StringVar(&cfgNetworkFile, CfgNetworkFile, "", "path to a network configuration file")

	parentCmd.AddCommand(analyzeCmd)
}
