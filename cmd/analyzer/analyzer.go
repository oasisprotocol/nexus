// Package analyzer implements the `analyze` sub-command.
package analyzer

import (
	"context"
	"os"
	"sync"
	"time"

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
	source "github.com/oasislabs/oasis-indexer/storage/oasis"
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
		log.NewDefaultLogger("init").Error("config init failed",
			"error", err,
		)
		os.Exit(1)
	}

	// Initialize common environment.
	if err = common.Init(cfg); err != nil {
		log.NewDefaultLogger("init").Error("init failed",
			"error", err,
		)
		os.Exit(1)
	}
	logger := common.Logger()

	if cfg.Analysis == nil {
		logger.Error("analysis config not provided")
		os.Exit(1)
	}

	service, err := Init(cfg.Analysis)
	if err != nil {
		os.Exit(1)
	}
	service.Shutdown()

	service.Start()
}

// Init initializes the analysis service.
func Init(cfg *config.AnalysisConfig) (*Service, error) {
	logger := common.Logger()

	m, err := migrate.New(
		cfg.Migrations,
		cfg.Storage.Endpoint,
	)
	if err != nil {
		logger.Error("migrator failed to start",
			"error", err,
		)
		return nil, err
	}

	switch err = m.Up(); {
	case err == migrate.ErrNoChange:
		logger.Info("migrations are up to date")
	case err != nil:
		logger.Error("migrations failed",
			"error", err,
		)
		return nil, err
	default:
		logger.Info("migrations completed")
	}

	service, err := NewService(cfg)
	if err != nil {
		logger.Error("service failed to start",
			"error", err,
		)
		return nil, err
	}
	return service, nil
}

// Service is the Oasis Indexer's analysis service.
type Service struct {
	Analyzers map[string]analyzer.Analyzer

	target storage.TargetStorage
	logger *log.Logger
}

// NewService creates new Service.
func NewService(cfg *config.AnalysisConfig) (*Service, error) {
	ctx := context.Background()
	logger := common.Logger().WithModule(moduleName)

	// Initialize target storage.
	client, err := common.NewClient(cfg.Storage, logger)
	if err != nil {
		return nil, err
	}

	// Initialize analyzers.
	consensusMainDamask := consensus.NewMain(client, logger)

	analyzers := map[string]analyzer.Analyzer{
		consensusMainDamask.Name(): consensusMainDamask,
	}

	for _, analyzerCfg := range cfg.Analyzers {
		a, ok := analyzers[analyzerCfg.Name]
		if !ok {
			continue
		}
		if analyzerCfg.Interval == "" {
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
			blockRange := analyzer.Range{
				From: analyzerCfg.From,
				To:   analyzerCfg.To,
			}
			a.SetConfig(analyzer.Config{
				ChainID:    analyzerCfg.ChainID,
				BlockRange: blockRange,
				Source:     source,
			})
		} else {
			interval, err := time.ParseDuration(analyzerCfg.Interval)
			if err != nil {
				return nil, err
			}

			// Configure analyzer.
			a.SetConfig(analyzer.Config{
				ChainID:  analyzerCfg.ChainID,
				Interval: interval,
			})
		}
	}

	logger.Info("initialized analyzers")

	return &Service{
		Analyzers: analyzers,

		target: client,
		logger: logger,
	}, nil
}

// Start starts the analysis service.
func (a *Service) Start() {
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

// Shutdown gracefully shuts down the service.
func (a *Service) Shutdown() {
	a.target.Shutdown()
}

// Register registers the process sub-command.
func Register(parentCmd *cobra.Command) {
	analyzeCmd.Flags().StringVar(&configFile, "config", "./config/local.yml", "path to the config.yml file")
	parentCmd.AddCommand(analyzeCmd)
}
