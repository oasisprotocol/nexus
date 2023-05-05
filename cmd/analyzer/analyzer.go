// Package analyzer implements the `analyze` sub-command.
package analyzer

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	migrate "github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres" // postgres driver for golang_migrate
	_ "github.com/golang-migrate/migrate/v4/source/file"       // support file scheme for golang_migrate
	_ "github.com/golang-migrate/migrate/v4/source/github"     // support github scheme for golang_migrate
	"github.com/spf13/cobra"

	"github.com/oasisprotocol/nexus/analyzer"
	"github.com/oasisprotocol/nexus/analyzer/aggregate_stats"
	"github.com/oasisprotocol/nexus/analyzer/consensus"
	"github.com/oasisprotocol/nexus/analyzer/evmcontractcode"
	"github.com/oasisprotocol/nexus/analyzer/evmtokenbalances"
	"github.com/oasisprotocol/nexus/analyzer/evmtokens"
	"github.com/oasisprotocol/nexus/analyzer/evmverifier"
	"github.com/oasisprotocol/nexus/analyzer/runtime"
	cmdCommon "github.com/oasisprotocol/nexus/cmd/common"
	"github.com/oasisprotocol/nexus/common"
	"github.com/oasisprotocol/nexus/config"
	"github.com/oasisprotocol/nexus/log"
	"github.com/oasisprotocol/nexus/storage"
	source "github.com/oasisprotocol/nexus/storage/oasis"
	"github.com/oasisprotocol/nexus/storage/oasis/nodeapi"
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

// RunMigrations runs migrations defined in sourceURL against databaseURL.
func RunMigrations(sourceURL string, databaseURL string) error {
	m, err := migrate.New(sourceURL, databaseURL)
	if err != nil {
		return err
	}

	return m.Up()
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

	switch err := RunMigrations(cfg.Storage.Migrations, cfg.Storage.Endpoint); {
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

// Service is Oasis Nexus's analysis service.
type Service struct {
	analyzers         []analyzer.Analyzer
	fastSyncAnalyzers []analyzer.Analyzer

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
	runtimes  map[common.Runtime]nodeapi.RuntimeApiLite
}

func newSourceFactory(cfg config.SourceConfig) *sourceFactory {
	return &sourceFactory{
		cfg:      cfg,
		runtimes: make(map[common.Runtime]nodeapi.RuntimeApiLite),
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

func (s *sourceFactory) Runtime(ctx context.Context, runtime common.Runtime) (nodeapi.RuntimeApiLite, error) {
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
func NewService(cfg *config.AnalysisConfig) (*Service, error) { //nolint:gocyclo
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

	// Initialize fast-sync analyzers.
	fastSyncAnalyzers := []A{}
	if cfg.Analyzers.Consensus != nil {
		if fastRange := cfg.Analyzers.Consensus.FastSyncRange(); fastRange != nil {
			for i := 0; i < cfg.Analyzers.Consensus.FastSync.Parallelism; i++ {
				fastSyncAnalyzers, err = addAnalyzer(fastSyncAnalyzers, err, func() (A, error) {
					chainContext := cfg.Source.History().CurrentRecord().ChainContext
					sourceClient, err1 := sources.Consensus(ctx)
					if err1 != nil {
						return nil, err1
					}
					return consensus.NewAnalyzer(*fastRange, cfg.Analyzers.Consensus.BatchSize, analyzer.FastSyncMode, chainContext, sourceClient, dbClient, logger)
				})
			}
		}
	}
	if cfg.Analyzers.Emerald != nil {
		if fastRange := cfg.Analyzers.Emerald.FastSyncRange(); fastRange != nil {
			for i := 0; i < cfg.Analyzers.Emerald.FastSync.Parallelism; i++ {
				fastSyncAnalyzers, err = addAnalyzer(fastSyncAnalyzers, err, func() (A, error) {
					sdkPT := cfg.Source.SDKParaTime(common.RuntimeEmerald)
					sourceClient, err1 := sources.Runtime(ctx, common.RuntimeEmerald)
					if err1 != nil {
						return nil, err1
					}
					return runtime.NewRuntimeAnalyzer(common.RuntimeEmerald, sdkPT, *fastRange, cfg.Analyzers.Emerald.BatchSize, analyzer.FastSyncMode, sourceClient, dbClient, logger)
				})
			}
		}
	}
	if cfg.Analyzers.Sapphire != nil {
		if fastRange := cfg.Analyzers.Sapphire.FastSyncRange(); fastRange != nil {
			for i := 0; i < cfg.Analyzers.Sapphire.FastSync.Parallelism; i++ {
				fastSyncAnalyzers, err = addAnalyzer(fastSyncAnalyzers, err, func() (A, error) {
					sdkPT := cfg.Source.SDKParaTime(common.RuntimeSapphire)
					sourceClient, err1 := sources.Runtime(ctx, common.RuntimeSapphire)
					if err1 != nil {
						return nil, err1
					}
					return runtime.NewRuntimeAnalyzer(common.RuntimeSapphire, sdkPT, *fastRange, cfg.Analyzers.Sapphire.BatchSize, analyzer.FastSyncMode, sourceClient, dbClient, logger)
				})
			}
		}
	}
	if cfg.Analyzers.Cipher != nil {
		if fastRange := cfg.Analyzers.Cipher.FastSyncRange(); fastRange != nil {
			for i := 0; i < cfg.Analyzers.Cipher.FastSync.Parallelism; i++ {
				fastSyncAnalyzers, err = addAnalyzer(fastSyncAnalyzers, err, func() (A, error) {
					sdkPT := cfg.Source.SDKParaTime(common.RuntimeCipher)
					sourceClient, err1 := sources.Runtime(ctx, common.RuntimeCipher)
					if err1 != nil {
						return nil, err1
					}
					return runtime.NewRuntimeAnalyzer(common.RuntimeCipher, sdkPT, *fastRange, cfg.Analyzers.Cipher.BatchSize, analyzer.FastSyncMode, sourceClient, dbClient, logger)
				})
			}
		}
	}

	// Initialize slow-sync analyzers.
	analyzers := []A{}
	if cfg.Analyzers.Consensus != nil {
		analyzers, err = addAnalyzer(analyzers, err, func() (A, error) {
			startHeight := int64(cfg.Analyzers.Consensus.From)
			startRecord, err1 := cfg.Source.History().RecordForHeight(startHeight)
			if err1 != nil {
				return nil, fmt.Errorf("getting history record for consensus starting block %d: %w", startHeight, err1)
			}
			genesisChainContext := startRecord.ChainContext
			sourceClient, err1 := sources.Consensus(ctx)
			if err1 != nil {
				return nil, err1
			}
			return consensus.NewAnalyzer(cfg.Analyzers.Consensus.SlowSyncRange(), cfg.Analyzers.Consensus.BatchSize, analyzer.SlowSyncMode, genesisChainContext, sourceClient, dbClient, logger)
		})
	}
	if cfg.Analyzers.Emerald != nil {
		analyzers, err = addAnalyzer(analyzers, err, func() (A, error) {
			runtimeMetadata := cfg.Source.SDKParaTime(common.RuntimeEmerald)
			sourceClient, err1 := sources.Runtime(ctx, common.RuntimeEmerald)
			if err1 != nil {
				return nil, err1
			}
			return runtime.NewRuntimeAnalyzer(common.RuntimeEmerald, runtimeMetadata, cfg.Analyzers.Emerald.SlowSyncRange(), cfg.Analyzers.Emerald.BatchSize, analyzer.SlowSyncMode, sourceClient, dbClient, logger)
		})
	}
	if cfg.Analyzers.Sapphire != nil {
		analyzers, err = addAnalyzer(analyzers, err, func() (A, error) {
			runtimeMetadata := cfg.Source.SDKParaTime(common.RuntimeSapphire)
			sourceClient, err1 := sources.Runtime(ctx, common.RuntimeSapphire)
			if err1 != nil {
				return nil, err1
			}
			return runtime.NewRuntimeAnalyzer(common.RuntimeSapphire, runtimeMetadata, cfg.Analyzers.Sapphire.SlowSyncRange(), cfg.Analyzers.Sapphire.BatchSize, analyzer.SlowSyncMode, sourceClient, dbClient, logger)
		})
	}
	if cfg.Analyzers.Cipher != nil {
		analyzers, err = addAnalyzer(analyzers, err, func() (A, error) {
			runtimeMetadata := cfg.Source.SDKParaTime(common.RuntimeCipher)
			sourceClient, err1 := sources.Runtime(ctx, common.RuntimeCipher)
			if err1 != nil {
				return nil, err1
			}
			return runtime.NewRuntimeAnalyzer(common.RuntimeCipher, runtimeMetadata, cfg.Analyzers.Cipher.SlowSyncRange(), cfg.Analyzers.Cipher.BatchSize, analyzer.SlowSyncMode, sourceClient, dbClient, logger)
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
		runtimeMetadata := cfg.Source.SDKParaTime(common.RuntimeEmerald)
		analyzers, err = addAnalyzer(analyzers, err, func() (A, error) {
			sourceClient, err1 := sources.Runtime(ctx, common.RuntimeEmerald)
			if err1 != nil {
				return nil, err1
			}
			return evmtokenbalances.NewMain(common.RuntimeEmerald, runtimeMetadata, sourceClient, dbClient, logger)
		})
	}
	if cfg.Analyzers.SapphireEvmTokenBalances != nil {
		runtimeMetadata := cfg.Source.SDKParaTime(common.RuntimeSapphire)
		analyzers, err = addAnalyzer(analyzers, err, func() (A, error) {
			sourceClient, err1 := sources.Runtime(ctx, common.RuntimeSapphire)
			if err1 != nil {
				return nil, err1
			}
			return evmtokenbalances.NewMain(common.RuntimeSapphire, runtimeMetadata, sourceClient, dbClient, logger)
		})
	}
	if cfg.Analyzers.EmeraldContractCode != nil {
		analyzers, err = addAnalyzer(analyzers, err, func() (A, error) {
			sourceClient, err1 := sources.Runtime(ctx, common.RuntimeEmerald)
			if err1 != nil {
				return nil, err1
			}
			return evmcontractcode.NewMain(common.RuntimeEmerald, sourceClient, dbClient, logger)
		})
	}
	if cfg.Analyzers.SapphireContractCode != nil {
		analyzers, err = addAnalyzer(analyzers, err, func() (A, error) {
			sourceClient, err1 := sources.Runtime(ctx, common.RuntimeSapphire)
			if err1 != nil {
				return nil, err1
			}
			return evmcontractcode.NewMain(common.RuntimeSapphire, sourceClient, dbClient, logger)
		})
	}
	if cfg.Analyzers.MetadataRegistry != nil {
		analyzers, err = addAnalyzer(analyzers, err, func() (A, error) {
			return analyzer.NewMetadataRegistryAnalyzer(cfg.Analyzers.MetadataRegistry, dbClient, logger)
		})
	}
	if cfg.Analyzers.AggregateStats != nil {
		analyzers, err = addAnalyzer(analyzers, err, func() (A, error) {
			return aggregate_stats.NewAggregateStatsAnalyzer(dbClient, logger)
		})
	}
	if cfg.Analyzers.EVMContractsVerifier != nil {
		analyzers, err = addAnalyzer(analyzers, err, func() (A, error) {
			return evmverifier.NewEVMVerifierAnalyzer(cfg.Analyzers.EVMContractsVerifier, dbClient, logger)
		})
	}
	if err != nil {
		return nil, err
	}

	logger.Info("initialized all analyzers")

	return &Service{
		fastSyncAnalyzers: fastSyncAnalyzers,
		analyzers:         analyzers,

		sources: sources,
		target:  dbClient,
		logger:  logger,
	}, nil
}

// closingChannel returns a channel that closes when the wait group `wg` is done.
func closingChannel(wg *sync.WaitGroup) <-chan struct{} {
	c := make(chan struct{})
	go func() {
		wg.Wait()
		close(c)
	}()
	return c
}

// Start starts the analysis service.
func (a *Service) Start() {
	defer a.cleanup()
	a.logger.Info("starting analysis service")

	ctx, cancelAnalyzers := context.WithCancel(context.Background())
	defer cancelAnalyzers() // Start() only returns when analyzers are done, so this should be a no-op, but it makes the compiler happier.

	// Start fast-sync analyzers.
	var fastSyncWg sync.WaitGroup
	for _, an := range a.fastSyncAnalyzers {
		fastSyncWg.Add(1)
		go func(an analyzer.Analyzer) {
			defer fastSyncWg.Done()
			an.Start(ctx)
		}(an)
	}
	fastSyncAnalyzersDone := closingChannel(&fastSyncWg)

	// Prepare slow-sync analyzers (to be started after fast-sync analyzers are done).
	var wg sync.WaitGroup
	for _, an := range a.analyzers {
		wg.Add(1)
		go func(an analyzer.Analyzer) {
			defer wg.Done()
			// Start the analyzer after fast-sync analyzers,
			// unless the context is canceled first (e.g. by ctrl+C during fast-sync).
			select {
			case <-ctx.Done():
				return
			case <-fastSyncAnalyzersDone:
				an.Start(ctx)
			}
		}(an)
	}
	analyzersDone := closingChannel(&wg)

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
		// Let the default handler handle ctrl+C so people can kill the process in a hurry.
		signal.Stop(signalChan)
		// Cancel the analyzers' context and wait for them (but not forever) to exit cleanly.
		cancelAnalyzers()
		select {
		case <-analyzersDone:
			a.logger.Info("all analyzers have exited cleanly")
		case <-time.After(10 * time.Second):
			// Analyzers are taking too long to exit cleanly, don't wait for them any longer or else k8s will force-kill us.
			// It's important that cleanup() is called, as this closes the KVStore (cache) cleanly;
			// if it doesn't get closed cleanly, KVStore requires a lenghty recovery process on next startup.
			a.logger.Warn("timed out waiting for analyzers to exit cleanly; now forcing IO resource cleanup")
		}
		// We'll call a.cleanup() via a defer.
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
	a.logger.Info("target db connection closed cleanly")
}

// Register registers the process sub-command.
func Register(parentCmd *cobra.Command) {
	analyzeCmd.Flags().StringVar(&configFile, "config", "./config/local.yml", "path to the config.yml file")
	parentCmd.AddCommand(analyzeCmd)
}
