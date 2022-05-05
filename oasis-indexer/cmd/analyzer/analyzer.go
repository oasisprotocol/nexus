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

	"github.com/oasislabs/oasis-block-indexer/go/analyzer"
	"github.com/oasislabs/oasis-block-indexer/go/analyzer/consensus"
	"github.com/oasislabs/oasis-block-indexer/go/log"
	"github.com/oasislabs/oasis-block-indexer/go/oasis-indexer/cmd/common"
	"github.com/oasislabs/oasis-block-indexer/go/storage"
	target "github.com/oasislabs/oasis-block-indexer/go/storage/cockroach"
	source "github.com/oasislabs/oasis-block-indexer/go/storage/oasis"
)

const (
	// CfgAnalysisConfig is the config file for configuration of chain analyzers.
	CfgAnalysisConfig = "analyzer.analysis_config"

	// CfgStorageEndpoint is the flag for setting the connection string to
	// the backing storage.
	CfgStorageEndpoint = "analyzer.storage_endpoint"

	moduleName = "analysis_service"
)

var (
	cfgStorageEndpoint string
	cfgAnalysisConfig  string

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
	Analyzers map[string]analyzer.Analyzer
	Nodes     map[string]storage.SourceStorage

	logger *log.Logger
}

// AnalysisServiceConfig contains configuration parameters for network analyzers.
type AnalysisServiceConfig struct {
	Spec struct {
		ChainContext string `yaml:"chaincontext"`
		Nodes        []struct {
			From      int64    `yaml:"from"`
			To        int64    `yaml:"to"`
			RPC       string   `yaml:"rpc"`
			Analyzers []string `yaml:"analyzers"`
		} `yaml:"nodes"`
	}
}

// NewAnalysisService creates new AnalysisService
func NewAnalysisService() (*AnalysisService, error) {
	ctx := context.Background()
	logger := common.Logger().WithModule(moduleName)

	// Get analyzer configurations.
	rawServiceCfg, err := ioutil.ReadFile(cfgAnalysisConfig)
	if err != nil {
		return nil, err
	}
	var serviceCfg AnalysisServiceConfig
	if err := yaml.Unmarshal(rawServiceCfg, &serviceCfg); err != nil {
		return nil, err
	}

	// Initialize sources.
	nodes := make(map[string]storage.SourceStorage)
	for _, nodeCfg := range serviceCfg.Spec.Nodes {
		networkCfg := config.Network{
			ChainContext: serviceCfg.Spec.ChainContext,
			RPC:          nodeCfg.RPC,
		}
		nodeClient, err := source.NewOasisNodeClient(ctx, &networkCfg)
		if err != nil {
			return nil, err
		}

		nodes[nodeCfg.RPC] = nodeClient
	}

	logger.Info("initialized sources")

	// Initialize target storage.
	cockroachClient, err := target.NewCockroachClient(cfgStorageEndpoint, logger)
	if err != nil {
		return nil, err
	}

	logger.Info("initialized target")

	// Initialize analyzers.
	consensusMainDamaskV1 := consensus.NewConsensusMain(cockroachClient, logger)

	analyzers := map[string]analyzer.Analyzer{
		consensusMainDamaskV1.Name(): consensusMainDamaskV1,
	}
	for _, nodeCfg := range serviceCfg.Spec.Nodes {
		for _, name := range nodeCfg.Analyzers {
			if a, ok := analyzers[name]; ok {
				a.AddRange(analyzer.RangeConfig{
					From:   nodeCfg.From,
					To:     nodeCfg.To,
					Source: nodes[nodeCfg.RPC],
				})
			}
		}
	}

	logger.Info("initialized analyzers")
	logger.Info("starting analysis service")

	return &AnalysisService{
		Analyzers: analyzers,
		Nodes:     nodes,
		logger:    logger,
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
	analyzeCmd.Flags().StringVar(&cfgAnalysisConfig, CfgAnalysisConfig, "", "path to an analysis service config file")
	analyzeCmd.Flags().StringVar(&cfgStorageEndpoint, CfgStorageEndpoint, "", "a postgresql-compliant connection url")
	parentCmd.AddCommand(analyzeCmd)
}
