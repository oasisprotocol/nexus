// Package analyzer implements the `analyze` sub-command.
package analyzer

import (
	"context"
	"io/ioutil"
	"os"
	"sync"

	migrate "github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/config"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"

	"github.com/oasislabs/oasis-block-indexer/go/analyzer"
	"github.com/oasislabs/oasis-block-indexer/go/analyzer/consensus"
	"github.com/oasislabs/oasis-block-indexer/go/log"
	"github.com/oasislabs/oasis-block-indexer/go/oasis-indexer/cmd/common"
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

	m, err := migrate.New(
		"file://../storage/migrations",
		cfgStorageEndpoint)
	if err != nil {
		common.Logger().Error("migrator failed to start",
			"error", err,
		)
		os.Exit(1)
	}

	err = m.Up()
	if err == migrate.ErrNoChange {
		common.Logger().Info("migrations are up to date")
	} else if err != nil {
		common.Logger().Error("migrations failed",
			"error", err,
		)
		os.Exit(1)
	} else {
		common.Logger().Info("migrations completed")
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

	logger *log.Logger
}

// AnalysisServiceConfig contains configuration parameters for network analyzers.
type AnalysisServiceConfig struct {
	Spec struct {
		Analyzers []struct {
			Name         string `yaml:"name"`
			RPC          string `yaml:"rpc"`
			ChainContext string `yaml:"chaincontext"`
			From         int64  `yaml:"from"`
			To           int64  `yaml:"to"`
		} `yaml:"analyzers"`
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

	// Initialize target storage.
	cockroachClient, err := target.NewCockroachClient(cfgStorageEndpoint, logger)
	if err != nil {
		return nil, err
	}

	// Initialize analyzers.
	consensusMainDamask := consensus.NewConsensusMain(cockroachClient, logger)

	analyzers := map[string]analyzer.Analyzer{
		consensusMainDamask.Name(): consensusMainDamask,
	}

	for _, analyzerCfg := range serviceCfg.Spec.Analyzers {
		if a, ok := analyzers[analyzerCfg.Name]; ok {
			// Initialize source.
			networkCfg := config.Network{
				ChainContext: analyzerCfg.ChainContext,
				RPC:          analyzerCfg.RPC,
			}
			source, err := source.NewOasisNodeClient(ctx, &networkCfg)
			if err != nil {
				return nil, err
			}

			// Configure analyzer.
			a.SetRange(analyzer.RangeConfig{
				From:   analyzerCfg.From,
				To:     analyzerCfg.To,
				Source: source,
			})
		}
	}

	logger.Info("initialized analyzers")

	return &AnalysisService{
		Analyzers: analyzers,
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
