// Package analyzer implements the `analyze` sub-command.
package analyzer

import (
	"context"
	"os"
	"os/signal"
	"sync"
	"syscall"

	migrate "github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres" // postgres driver for golang_migrate
	_ "github.com/golang-migrate/migrate/v4/source/file"       // support file scheme for golang_migrate
	_ "github.com/golang-migrate/migrate/v4/source/github"     // support github scheme for golang_migrate
	"github.com/spf13/cobra"

	"github.com/oasisprotocol/oasis-indexer/analyzer"
	"github.com/oasisprotocol/oasis-indexer/analyzer/consensus"
	"github.com/oasisprotocol/oasis-indexer/analyzer/evmtokenbalances"
	"github.com/oasisprotocol/oasis-indexer/analyzer/evmtokens"
	"github.com/oasisprotocol/oasis-indexer/analyzer/runtime"
	cmdCommon "github.com/oasisprotocol/oasis-indexer/cmd/common"
	"github.com/oasisprotocol/oasis-indexer/common"
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
	if err = cmdCommon.Init(cfg); err != nil {
		log.NewDefaultLogger("init").Error("init failed",
			"error", err,
		)
		os.Exit(1)
	}
	logger := cmdCommon.Logger()

	if cfg.Analysis == nil {
		logger.Error("analysis config not provided")
		os.Exit(1)
	}

	service, err := Init(cfg.Analysis)
	if err != nil {
		os.Exit(1)
	}
	service.Start()
}

// Init initializes the analysis service.
func Init(cfg *config.AnalysisConfig) (*Service, error) {
	logger := cmdCommon.Logger()

	logger.Info("initializing analysis service", "config", cfg)
	if cfg.Storage.WipeStorage {
		logger.Warn("wiping storage")
		if err := wipeStorage(cfg.Storage); err != nil {
			return nil, err
		}
		logger.Info("storage wiped")
	}

	m, err := migrate.New(
		cfg.Storage.Migrations,
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
		logger.Info("no migrations needed to be applied")
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

func wipeStorage(cfg *config.StorageConfig) error {
	logger := cmdCommon.Logger().WithModule(moduleName)

	// Initialize target storage.
	storage, err := cmdCommon.NewClient(cfg, logger)
	if err != nil {
		return err
	}
	defer storage.Close()

	ctx := context.Background()
	return storage.Wipe(ctx)
}

// Service is the Oasis Indexer's analysis service.
type Service struct {
	Analyzers map[string]analyzer.Analyzer

	target storage.TargetStorage
	logger *log.Logger
}

type A = analyzer.Analyzer

// addAnalyzer adds the analyzer produced by `analyzerGenerator()` to `analyzers`.
// It expects an initial state (analyzers, errSoFar) and returns the updated state, which
// should be fed into subsequent call to the function.
// As soon as an analyzerGenerator returns an error, all subsequent calls will
// short-circuit and return the same error, leaving `analyzers` unchanged.
func addAnalyzer(analyzers map[string]A, errSoFar error, analyzerGenerator func() (A, error)) (map[string]A, error) {
	if errSoFar != nil {
		return analyzers, errSoFar
	}
	a, errSoFar := analyzerGenerator()
	if errSoFar != nil {
		return analyzers, errSoFar
	}
	analyzers[a.Name()] = a
	return analyzers, nil
}

// NewService creates new Service.
func NewService(cfg *config.AnalysisConfig) (*Service, error) {
	logger := cmdCommon.Logger().WithModule(moduleName)

	// Initialize target storage.
	client, err := cmdCommon.NewClient(cfg.Storage, logger)
	if err != nil {
		return nil, err
	}

	// Initialize analyzers.
	analyzers := map[string]A{}
	if cfg.Analyzers.Consensus != nil {
		analyzers, err = addAnalyzer(analyzers, err, func() (A, error) {
			return consensus.NewMain(&cfg.Source, cfg.Analyzers.Consensus, client, logger)
		})
	}
	if cfg.Analyzers.Emerald != nil {
		analyzers, err = addAnalyzer(analyzers, err, func() (A, error) {
			return runtime.NewRuntimeAnalyzer(common.RuntimeEmerald, &cfg.Source, cfg.Analyzers.Emerald, client, logger)
		})
	}
	if cfg.Analyzers.Sapphire != nil {
		analyzers, err = addAnalyzer(analyzers, err, func() (A, error) {
			return runtime.NewRuntimeAnalyzer(common.RuntimeSapphire, &cfg.Source, cfg.Analyzers.Sapphire, client, logger)
		})
	}
	if cfg.Analyzers.Cipher != nil {
		analyzers, err = addAnalyzer(analyzers, err, func() (A, error) {
			return runtime.NewRuntimeAnalyzer(common.RuntimeCipher, &cfg.Source, cfg.Analyzers.Cipher, client, logger)
		})
	}
	if cfg.Analyzers.EmeraldEvmTokens != nil {
		analyzers, err = addAnalyzer(analyzers, err, func() (A, error) {
			return evmtokens.NewMain(common.RuntimeEmerald, &cfg.Source, client, logger)
		})
	}
	if cfg.Analyzers.SapphireEvmTokens != nil {
		analyzers, err = addAnalyzer(analyzers, err, func() (A, error) {
			return evmtokens.NewMain(common.RuntimeSapphire, &cfg.Source, client, logger)
		})
	}
	if cfg.Analyzers.EmeraldEvmTokenBalances != nil {
		analyzers, err = addAnalyzer(analyzers, err, func() (A, error) {
			return evmtokenbalances.NewMain(common.RuntimeEmerald, &cfg.Source, client, logger)
		})
	}
	if cfg.Analyzers.SapphireEvmTokenBalances != nil {
		analyzers, err = addAnalyzer(analyzers, err, func() (A, error) {
			return evmtokenbalances.NewMain(common.RuntimeSapphire, &cfg.Source, client, logger)
		})
	}
	if cfg.Analyzers.MetadataRegistry != nil {
		analyzers, err = addAnalyzer(analyzers, err, func() (A, error) {
			return analyzer.NewMetadataRegistryAnalyzer(cfg.Analyzers.MetadataRegistry, client, logger)
		})
	}
	if cfg.Analyzers.AggregateStats != nil {
		analyzers, err = addAnalyzer(analyzers, err, func() (A, error) {
			return analyzer.NewAggregateStatsAnalyzer(cfg.Analyzers.AggregateStats, client, logger)
		})
	}
	if err != nil {
		return nil, err
	}

	logger.Info("initialized all analyzers")

	return &Service{
		Analyzers: analyzers,

		target: client,
		logger: logger,
	}, nil
}

// Start starts the analysis service.
func (a *Service) Start() {
	defer a.cleanup()
	a.logger.Info("starting analysis service")

	ctx, cancelAnalyzers := context.WithCancel(context.Background())
	defer cancelAnalyzers() // Start() only returns when analyzers are done, so this should be a no-op, but it makes the compiler happier.

	// Start all analyzers.
	var wg sync.WaitGroup
	for _, an := range a.Analyzers {
		wg.Add(1)
		go func(an analyzer.Analyzer) {
			defer wg.Done()
			an.Start(ctx)
		}(an)
	}

	// Create a channel that will close when all analyzers have completed.
	analyzersDone := make(chan struct{})
	go func() {
		wg.Wait()
		close(analyzersDone)
	}()

	// Trap Ctrl+C and SIGTERM; the latter is issued by Kubernetes to request a shutdown.
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(signalChan) // Stop catching Ctrl+C signals.

	// Wait for analyzers to finish.
	select {
	case <-analyzersDone:
		a.logger.Info("all analyzers have completed")
		return
	case <-signalChan:
		a.logger.Info("received interrupt, shutting down")
		// Cancel the analyzers' context and wait for them to exit cleanly.
		cancelAnalyzers()
		select {
		case <-analyzersDone:
			a.logger.Info("all analyzers have exited cleanly")
			return
		case <-signalChan:
			a.logger.Info("received second interrupt, forcing exit")
			return
		}
	}
}

// cleanup cleans up resources used by the service.
func (a *Service) cleanup() {
	a.target.Close()
}

// Register registers the process sub-command.
func Register(parentCmd *cobra.Command) {
	analyzeCmd.Flags().StringVar(&configFile, "config", "./config/local.yml", "path to the config.yml file")
	parentCmd.AddCommand(analyzeCmd)
}
