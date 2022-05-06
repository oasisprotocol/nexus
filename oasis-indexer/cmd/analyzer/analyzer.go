// Package analyzer implements the `analyze` sub-command.
package analyzer

import (
	"context"
	"io/ioutil"
	"os"
	"sync"

	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/config"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"

	"github.com/oasislabs/oasis-indexer/analyzer"
	"github.com/oasislabs/oasis-indexer/analyzer/consensus"
	"github.com/oasislabs/oasis-indexer/log"
	"github.com/oasislabs/oasis-indexer/oasis-indexer/cmd/common"
	"github.com/oasislabs/oasis-indexer/storage"
	target "github.com/oasislabs/oasis-indexer/storage/cockroach"
	source "github.com/oasislabs/oasis-indexer/storage/oasis"
)

const (
	// CfgStorageEndpoint is the flag for setting the connection string to
	// the backing storage.
	CfgStorageEndpoint = "analyzer.storage_endpoint"

	// CfgNetworkConfig is the config file for connecting to an oasis-node.
	CfgNetworkConfig = "analyzer.network_config"

	moduleName = "analysis_service"
)

var (
	cfgStorageEndpoint string
	cfgNetworkConfig   string

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
	if err != nil {
		common.Logger().Error("service failed to start",
			"error", err,
		)
		os.Exit(1)
	}

	service.Start()
}

// AnalysisService is the Oasis Indexer's analysis service.
type AnalysisService struct {
	ChainID   string
	Analyzers map[string]analyzer.Analyzer

	source storage.SourceStorage
	target storage.TargetStorage
	logger *log.Logger
}

// NewAnalysisService creates new AnalysisService
func NewAnalysisService() (*AnalysisService, error) {
	ctx := context.Background()
	logger := common.Logger().WithModule(moduleName)

	// Initialize source storage.
	rawCfg, err := ioutil.ReadFile(cfgNetworkConfig)
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
	cockroachClient, err := target.NewCockroachClient(cfgStorageEndpoint, logger)
	if err != nil {
		return nil, err
	}

	// Initialize analyzers.
	consensusAnalyzer := consensus.NewConsensusMain(oasisNodeClient, cockroachClient, logger)

	d, err := oasisNodeClient.GenesisDocument(ctx)
	if err != nil {
		return nil, err
	}

	logger.Info("Starting oasis-indexer analysis layer.")

	return &AnalysisService{
		ChainID: d.ChainID,
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
	a.logger.Info("starting analysis service")

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
	analyzeCmd.Flags().StringVar(&cfgNetworkConfig, CfgNetworkConfig, "", "path to a network configuration file")
	parentCmd.AddCommand(analyzeCmd)
}
