// Package analyzer implements the `analyze` sub-command.
package analyzer

import (
	"context"
	"fmt"
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
	source "github.com/oasisprotocol/oasis-indexer/storage/oasis"
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
	Analyzers []analyzer.Analyzer

	sources *sourceFactory
	target  storage.TargetStorage
	logger  *log.Logger
}

// sourceFactory stores singletons of the sources used by all the analyzers in a Service.
// This enables re-use of node connections as well as graceful shutdown.
// Note: NOT thread safe.
type sourceFactory struct {
	cfg config.SourceConfig

	consensus *source.ConsensusClient
	runtimes  map[common.Runtime]*source.RuntimeClient
}

func newSourceFactory(cfg config.SourceConfig) *sourceFactory {
	return &sourceFactory{
		cfg:      cfg,
		runtimes: make(map[common.Runtime]*source.RuntimeClient),
	}
}

func (s *sourceFactory) Close() error {
	var firstErr error
	if s.consensus != nil {
		if err := s.consensus.Close(); err != nil {
			firstErr = err
		}
	}
	for _, runtimeClient := range s.runtimes {
		if err := runtimeClient.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}

	return firstErr
}

func (s *sourceFactory) Consensus(ctx context.Context) (*source.ConsensusClient, error) {
	if s.consensus == nil {
		client, err := source.NewConsensusClient(ctx, &s.cfg)
		if err != nil {
			return nil, fmt.Errorf("error creating consensus client: %w", err)
		}
		s.consensus = client
	}

	return s.consensus, nil
}

func (s *sourceFactory) Runtime(ctx context.Context, runtime common.Runtime) (*source.RuntimeClient, error) {
	_, ok := s.runtimes[runtime]
	if !ok {
		client, err := source.NewRuntimeClient(ctx, &s.cfg, runtime)
		if err != nil {
			return nil, fmt.Errorf("error creating %s client: %w", string(runtime), err)
		}
		s.runtimes[runtime] = client
	}

	return s.runtimes[runtime], nil
}

type A = analyzer.Analyzer

// addAnalyzer adds the analyzer produced by `analyzerGenerator()` to `analyzers`.
// It expects an initial state (analyzers, errSoFar) and returns the updated state, which
// should be fed into subsequent call to the function.
// As soon as an analyzerGenerator returns an error, all subsequent calls will
// short-circuit and return the same error, leaving `analyzers` unchanged.
func addAnalyzer(analyzers []A, errSoFar error, analyzerGenerator func() (A, error)) ([]A, error) {
	if errSoFar != nil {
		return analyzers, errSoFar
	}
	a, errSoFar := analyzerGenerator()
	if errSoFar != nil {
		return analyzers, errSoFar
	}
	analyzers = append(analyzers, a)
	return analyzers, nil
}

// NewService creates new Service.
func NewService(cfg *config.AnalysisConfig) (*Service, error) {
	ctx := context.Background()
	logger := cmdCommon.Logger().WithModule(moduleName)
	logger.Info("initializing analysis service", "config", cfg)

	// Initialize source storage.
	sources := newSourceFactory(cfg.Source)

	// Initialize target storage.
	dbClient, err := cmdCommon.NewClient(cfg.Storage, logger)
	if err != nil {
		return nil, err
	}

	// Initialize analyzers.
	analyzers := []A{}
	if cfg.Analyzers.Consensus != nil {
		analyzers, err = addAnalyzer(analyzers, err, func() (A, error) {
			genesisChainContext := cfg.Source.History().CurrentRecord().ChainContext
			sourceClient, err1 := sources.Consensus(ctx)
			if err1 != nil {
				return nil, err1
			}
			return consensus.NewConsensusAnalyzer(cfg.Analyzers.Consensus, genesisChainContext, sourceClient, dbClient, logger)
		})
	}
	if cfg.Analyzers.Emerald != nil {
		analyzers, err = addAnalyzer(analyzers, err, func() (A, error) {
			runtimeMetadata := cfg.Source.SDKParaTime(common.RuntimeEmerald)
			sourceClient, err1 := sources.Runtime(ctx, common.RuntimeEmerald)
			if err1 != nil {
				return nil, err1
			}
			return runtime.NewRuntimeAnalyzer(common.RuntimeEmerald, runtimeMetadata, cfg.Analyzers.Emerald, sourceClient, dbClient, logger)
		})
	}
	if cfg.Analyzers.Sapphire != nil {
		analyzers, err = addAnalyzer(analyzers, err, func() (A, error) {
			runtimeMetadata := cfg.Source.SDKParaTime(common.RuntimeSapphire)
			sourceClient, err1 := sources.Runtime(ctx, common.RuntimeSapphire)
			if err1 != nil {
				return nil, err1
			}
			return runtime.NewRuntimeAnalyzer(common.RuntimeSapphire, runtimeMetadata, cfg.Analyzers.Sapphire, sourceClient, dbClient, logger)
		})
	}
	if cfg.Analyzers.Cipher != nil {
		analyzers, err = addAnalyzer(analyzers, err, func() (A, error) {
			runtimeMetadata := cfg.Source.SDKParaTime(common.RuntimeCipher)
			sourceClient, err1 := sources.Runtime(ctx, common.RuntimeCipher)
			if err1 != nil {
				return nil, err1
			}
			return runtime.NewRuntimeAnalyzer(common.RuntimeCipher, runtimeMetadata, cfg.Analyzers.Cipher, sourceClient, dbClient, logger)
		})
	}
	if cfg.Analyzers.EmeraldEvmTokens != nil {
		analyzers, err = addAnalyzer(analyzers, err, func() (A, error) {
			sourceClient, err1 := sources.Runtime(ctx, common.RuntimeEmerald)
			if err1 != nil {
				return nil, err1
			}
			return evmtokens.NewMain(common.RuntimeEmerald, sourceClient, dbClient, logger)
		})
	}
	if cfg.Analyzers.SapphireEvmTokens != nil {
		analyzers, err = addAnalyzer(analyzers, err, func() (A, error) {
			sourceClient, err1 := sources.Runtime(ctx, common.RuntimeSapphire)
			if err1 != nil {
				return nil, err1
			}
			return evmtokens.NewMain(common.RuntimeSapphire, sourceClient, dbClient, logger)
		})
	}
	if cfg.Analyzers.EmeraldEvmTokenBalances != nil {
		analyzers, err = addAnalyzer(analyzers, err, func() (A, error) {
			sourceClient, err1 := sources.Runtime(ctx, common.RuntimeEmerald)
			if err1 != nil {
				return nil, err1
			}
			return evmtokenbalances.NewMain(common.RuntimeEmerald, sourceClient, dbClient, logger)
		})
	}
	if cfg.Analyzers.SapphireEvmTokenBalances != nil {
		analyzers, err = addAnalyzer(analyzers, err, func() (A, error) {
			sourceClient, err1 := sources.Runtime(ctx, common.RuntimeSapphire)
			if err1 != nil {
				return nil, err1
			}
			return evmtokenbalances.NewMain(common.RuntimeSapphire, sourceClient, dbClient, logger)
		})
	}
	if cfg.Analyzers.MetadataRegistry != nil {
		analyzers, err = addAnalyzer(analyzers, err, func() (A, error) {
			return analyzer.NewMetadataRegistryAnalyzer(cfg.Analyzers.MetadataRegistry, dbClient, logger)
		})
	}
	if cfg.Analyzers.AggregateStats != nil {
		analyzers, err = addAnalyzer(analyzers, err, func() (A, error) {
			return analyzer.NewAggregateStatsAnalyzer(cfg.Analyzers.AggregateStats, dbClient, logger)
		})
	}
	if err != nil {
		return nil, err
	}

	logger.Info("initialized all analyzers")

	return &Service{
		Analyzers: analyzers,

		sources: sources,
		target:  dbClient,
		logger:  logger,
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
		signal.Stop(signalChan) // Let the default handler handle ctrl+C so people can kill the process in a hurry.
		<-analyzersDone
		a.logger.Info("all analyzers have exited cleanly")
		return
	}
}

// cleanup cleans up resources used by the service.
func (a *Service) cleanup() {
	if err := a.sources.Close(); err != nil {
		a.logger.Error("failed to cleanly close data source",
			"firstErr", err.Error(),
		)
	}
	a.logger.Info("all source connections have closed cleanly")
	a.target.Close()
	a.logger.Info("indexer db connection closed cleanly")
}

// Register registers the process sub-command.
func Register(parentCmd *cobra.Command) {
	analyzeCmd.Flags().StringVar(&configFile, "config", "./config/local.yml", "path to the config.yml file")
	parentCmd.AddCommand(analyzeCmd)
}
