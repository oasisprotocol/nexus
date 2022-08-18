// Package analyzer implements the `analyze` sub-command.
package analyzer

import (
	"os"
	"sync"

	migrate "github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres" // postgres driver for golang_migrate
	_ "github.com/golang-migrate/migrate/v4/source/file"       // support file scheme for golang_migrate
	_ "github.com/golang-migrate/migrate/v4/source/github"     // support github scheme for golang_migrate
	"github.com/spf13/cobra"

	"github.com/oasisprotocol/oasis-indexer/analyzer"
	"github.com/oasisprotocol/oasis-indexer/analyzer/consensus"
	"github.com/oasisprotocol/oasis-indexer/cmd/common"
	"github.com/oasisprotocol/oasis-indexer/config"
	"github.com/oasisprotocol/oasis-indexer/log"
	"github.com/oasisprotocol/oasis-indexer/storage"
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
	defer service.Shutdown()

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
	logger := common.Logger().WithModule(moduleName)

	// Initialize target storage.
	client, err := common.NewClient(cfg.Storage, logger)
	if err != nil {
		return nil, err
	}

	// Initialize analyzers.
	analyzers := map[string]analyzer.Analyzer{}
	for _, analyzerCfg := range cfg.Analyzers {
		//nolint:gocritic
		switch analyzerCfg.Name {
		case "consensus_main_damask":
			consensusMainDamask, err := consensus.NewMain(analyzerCfg, client, logger)
			if err != nil {
				return nil, err
			}
			analyzers[consensusMainDamask.Name()] = consensusMainDamask
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
