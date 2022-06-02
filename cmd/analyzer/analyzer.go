// Package analyzer implements the `analyze` sub-command.
package analyzer

import (
	"context"
	"os"
	"sync"

	migrate "github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres" // postgres driver for golang_migrate
	_ "github.com/golang-migrate/migrate/v4/source/file"       // support file scheme for golang_migrate
	_ "github.com/golang-migrate/migrate/v4/source/github"     // support github scheme for golang_migrate
	oasisConfig "github.com/oasisprotocol/oasis-sdk/client-sdk/go/config"
	"github.com/spf13/cobra"

	"github.com/oasislabs/oasis-indexer/analyzer"
	"github.com/oasislabs/oasis-indexer/analyzer/consensus"
	"github.com/oasislabs/oasis-indexer/cmd/common"
	"github.com/oasislabs/oasis-indexer/config"
	"github.com/oasislabs/oasis-indexer/log"
	"github.com/oasislabs/oasis-indexer/storage"
	"github.com/oasislabs/oasis-indexer/storage/cockroach"
	source "github.com/oasislabs/oasis-indexer/storage/oasis"
	"github.com/oasislabs/oasis-indexer/storage/postgres"
)

const (
	moduleName = "analysis_service"
)

var (
	// Path to the configuration file.
	configFile string

	analyzeCmd = &cobra.Command{
		Use:   "analyze",
		Short: "Analyze blocks",
		Run:   runAnalyzer,
	}
)

func runAnalyzer(cmd *cobra.Command, args []string) {
	// Initialize config.
	cfg, err := config.InitConfig(configFile)
	if err != nil {
		os.Exit(1)
	}

	// Initialize common environment.
	if err := common.Init(cfg); err != nil {
		os.Exit(1)
	}
	logger := common.Logger()

	if cfg.Analysis == nil {
		logger.Error("analysis config not provided")
		os.Exit(1)
	}
	aCfg := cfg.Analysis

	m, err := migrate.New(
		aCfg.Migrations,
		aCfg.Storage.Endpoint,
	)
	if err != nil {
		logger.Error("migrator failed to start",
			"error", err,
		)
		os.Exit(1)
	}

	switch err = m.Up(); {
	case err == migrate.ErrNoChange:
		logger.Info("migrations are up to date")
	case err != nil:
		logger.Error("migrations failed",
			"error", err,
		)
		os.Exit(1)
	default:
		logger.Info("migrations completed")
	}

	service, err := NewAnalysisService(aCfg)
	if err != nil {
		logger.Error("service failed to start",
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

// NewAnalysisService creates new AnalysisService.
func NewAnalysisService(cfg *config.AnalysisConfig) (*AnalysisService, error) {
	ctx := context.Background()
	logger := common.Logger().WithModule(moduleName)

	// Initialize target storage.
	var backend config.StorageBackend
	if err := backend.Set(cfg.Storage.Backend); err != nil {
		return nil, err
	}

	var client storage.TargetStorage
	var err error
	switch backend {
	case config.BackendCockroach:
		client, err = cockroach.NewClient(cfg.Storage.Endpoint, logger)
	case config.BackendPostgres:
		client, err = postgres.NewClient(cfg.Storage.Endpoint, logger)
	}
	if err != nil {
		return nil, err
	}

	// Initialize analyzers.
	consensusMainDamask := consensus.NewMain(client, logger)

	analyzers := map[string]analyzer.Analyzer{
		consensusMainDamask.Name(): consensusMainDamask,
	}

	for _, analyzerCfg := range cfg.Analyzers {
		if a, ok := analyzers[analyzerCfg.Name]; ok {
			// Initialize source.
			networkCfg := oasisConfig.Network{
				ChainContext: analyzerCfg.ChainContext,
				RPC:          analyzerCfg.RPC,
			}
			source, err := source.NewClient(ctx, &networkCfg)
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
	analyzeCmd.Flags().StringVar(&configFile, "config", "./config/local.yml", "path to the config.yml file")
	parentCmd.AddCommand(analyzeCmd)
}
