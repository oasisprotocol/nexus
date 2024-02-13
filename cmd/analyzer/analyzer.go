// Package analyzer implements the `analyze` sub-command.
package analyzer

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
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
	"github.com/oasisprotocol/nexus/analyzer/evmabibackfill"
	"github.com/oasisprotocol/nexus/analyzer/evmcontractcode"
	"github.com/oasisprotocol/nexus/analyzer/evmnfts"
	"github.com/oasisprotocol/nexus/analyzer/evmnfts/ipfsclient"
	"github.com/oasisprotocol/nexus/analyzer/evmtokenbalances"
	"github.com/oasisprotocol/nexus/analyzer/evmtokens"
	"github.com/oasisprotocol/nexus/analyzer/evmverifier"
	"github.com/oasisprotocol/nexus/analyzer/metadata_registry"
	nodestats "github.com/oasisprotocol/nexus/analyzer/node_stats"
	"github.com/oasisprotocol/nexus/analyzer/runtime"
	"github.com/oasisprotocol/nexus/analyzer/util"
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
	logger := cmdCommon.RootLogger()

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
	logger := cmdCommon.RootLogger()

	logger.Info("initializing analysis service", "config", cfg)
	if cfg.Storage.WipeStorage {
		logger.Warn("wiping storage")
		if err := wipeStorage(cfg.Storage); err != nil {
			return nil, err
		}
		logger.Info("storage wiped")
	}

	logger.Info("checking if migrations need to be applied...")
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
	logger := cmdCommon.RootLogger().WithModule(moduleName)

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
	analyzers         []SyncedAnalyzer
	fastSyncAnalyzers []SyncedAnalyzer

	sources *sourceFactory
	target  storage.TargetStorage
	logger  *log.Logger
}

// sourceFactory stores singletons of the sources used by all the analyzers in a Service.
// This enables re-use of node connections as well as graceful shutdown.
// Note: NOT thread safe.
type sourceFactory struct {
	cfg config.SourceConfig

	consensus nodeapi.ConsensusApiLite
	runtimes  map[common.Runtime]nodeapi.RuntimeApiLite
	ipfs      ipfsclient.Client
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

func (s *sourceFactory) Consensus(ctx context.Context) (nodeapi.ConsensusApiLite, error) {
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

func (s *sourceFactory) IPFS(_ context.Context) (ipfsclient.Client, error) {
	if s.ipfs == nil {
		client, err := ipfsclient.NewGateway(strings.TrimSuffix(s.cfg.IPFS.Gateway, "/"))
		if err != nil {
			return nil, fmt.Errorf("error creating ipfs client: %w", err)
		}
		s.ipfs = client
	}
	return s.ipfs, nil
}

// Shorthand for use within this file.
type A = analyzer.Analyzer

// An Analyzer that is tagged with a `syncTag`.
// The `syncTag` is used for sequencing analyzers: For any non-empty tag, nexus will
// first run all fast-sync analyzers with that tag to completion, and only then start
// other analyzers with the same tag. The empty tag "" is special; it can be used
// by slow-sync analyzers that don't need to wait for any fast-sync analyzers to complete.
// This mechanism is a simple(ish) alternative to supporting a full-blown execution/dependency graph between analyzers.
type SyncedAnalyzer struct {
	Analyzer analyzer.Analyzer
	SyncTag  string
}

// addAnalyzer adds the analyzer produced by `analyzerGenerator()` to `analyzers`.
// It expects an initial state (analyzers, errSoFar) and returns the updated state, which
// should be fed into subsequent call to the function.
// As soon as an analyzerGenerator returns an error, all subsequent calls will
// short-circuit and return the same error, leaving `analyzers` unchanged.
// See `SyncedAnalyzer` for more info on `syncTag`.
func addAnalyzer(analyzers []SyncedAnalyzer, errSoFar error, syncTag string, analyzerGenerator func() (A, error)) ([]SyncedAnalyzer, error) {
	if errSoFar != nil {
		return analyzers, errSoFar
	}
	a, errSoFar := analyzerGenerator()
	if errSoFar != nil {
		return analyzers, errSoFar
	}
	analyzers = append(analyzers, SyncedAnalyzer{Analyzer: a, SyncTag: syncTag})
	return analyzers, nil
}

var (
	syncTagConsensus = "consensus"
	syncTagEmerald   = string(common.RuntimeEmerald)
	syncTagSapphire  = string(common.RuntimeSapphire)
	syncTagCipher    = string(common.RuntimeCipher)
	syncTagPontusx   = string(common.RuntimePontusx)
)

// NewService creates new Service.
func NewService(cfg *config.AnalysisConfig) (*Service, error) { //nolint:gocyclo
	ctx := context.Background()
	logger := cmdCommon.RootLogger().WithModule(moduleName)
	logger.Info("initializing analysis service", "config", cfg)

	// Initialize source storage.
	sources := newSourceFactory(cfg.Source)

	// Initialize target storage.
	dbClient, err := cmdCommon.NewClient(cfg.Storage, logger)
	if err != nil {
		return nil, err
	}

	// Initialize fast-sync analyzers.
	fastSyncAnalyzers := []SyncedAnalyzer{}
	if cfg.Analyzers.Consensus != nil {
		if fastRange := cfg.Analyzers.Consensus.FastSyncRange(); fastRange != nil {
			for i := 0; i < cfg.Analyzers.Consensus.FastSync.Parallelism; i++ {
				fastSyncAnalyzers, err = addAnalyzer(fastSyncAnalyzers, err, syncTagConsensus, func() (A, error) {
					sourceClient, err1 := sources.Consensus(ctx)
					if err1 != nil {
						return nil, err1
					}
					return consensus.NewAnalyzer(*fastRange, cfg.Analyzers.Consensus.BatchSize, analyzer.FastSyncMode, *cfg.Source.History(), sourceClient, *cfg.Source.SDKNetwork(), dbClient, logger)
				})
			}
		}
	}
	// Helper func that adds N fast-sync analyzers for a given runtime to, with N (and other properties) pulled from the config.
	// NOTE: The helper extensively reads AND WRITES variables in the parent scope.
	//       The side-effects (=writes) happen in `fastSyncAnalyzers` and `err`.
	addFastSyncRuntimeAnalyzers := func(runtimeName common.Runtime, config *config.BlockBasedAnalyzerConfig) {
		if config != nil {
			if fastRange := config.FastSyncRange(); fastRange != nil {
				for i := 0; i < config.FastSync.Parallelism; i++ {
					fastSyncAnalyzers, err = addAnalyzer(fastSyncAnalyzers, err, string(runtimeName), func() (A, error) {
						sdkPT := cfg.Source.SDKParaTime(runtimeName)
						sourceClient, err1 := sources.Runtime(ctx, runtimeName)
						if err1 != nil {
							return nil, err1
						}
						return runtime.NewRuntimeAnalyzer(runtimeName, sdkPT, *fastRange, config.BatchSize, analyzer.FastSyncMode, sourceClient, dbClient, logger)
					})
				}
			}
		}
	}
	addFastSyncRuntimeAnalyzers(common.RuntimeEmerald, cfg.Analyzers.Emerald)
	addFastSyncRuntimeAnalyzers(common.RuntimeSapphire, cfg.Analyzers.Sapphire)
	addFastSyncRuntimeAnalyzers(common.RuntimePontusx, cfg.Analyzers.Pontusx)
	addFastSyncRuntimeAnalyzers(common.RuntimeCipher, cfg.Analyzers.Cipher)

	// Initialize slow-sync analyzers.
	analyzers := []SyncedAnalyzer{}
	if cfg.Analyzers.Consensus != nil {
		analyzers, err = addAnalyzer(analyzers, err, syncTagConsensus, func() (A, error) {
			sourceClient, err1 := sources.Consensus(ctx)
			if err1 != nil {
				return nil, err1
			}
			return consensus.NewAnalyzer(cfg.Analyzers.Consensus.SlowSyncRange(), cfg.Analyzers.Consensus.BatchSize, analyzer.SlowSyncMode, *cfg.Source.History(), sourceClient, *cfg.Source.SDKNetwork(), dbClient, logger)
		})
	}
	if cfg.Analyzers.Emerald != nil {
		analyzers, err = addAnalyzer(analyzers, err, syncTagEmerald, func() (A, error) {
			runtimeMetadata := cfg.Source.SDKParaTime(common.RuntimeEmerald)
			sourceClient, err1 := sources.Runtime(ctx, common.RuntimeEmerald)
			if err1 != nil {
				return nil, err1
			}
			return runtime.NewRuntimeAnalyzer(common.RuntimeEmerald, runtimeMetadata, cfg.Analyzers.Emerald.SlowSyncRange(), cfg.Analyzers.Emerald.BatchSize, analyzer.SlowSyncMode, sourceClient, dbClient, logger)
		})
	}
	if cfg.Analyzers.Sapphire != nil {
		analyzers, err = addAnalyzer(analyzers, err, syncTagSapphire, func() (A, error) {
			runtimeMetadata := cfg.Source.SDKParaTime(common.RuntimeSapphire)
			sourceClient, err1 := sources.Runtime(ctx, common.RuntimeSapphire)
			if err1 != nil {
				return nil, err1
			}
			return runtime.NewRuntimeAnalyzer(common.RuntimeSapphire, runtimeMetadata, cfg.Analyzers.Sapphire.SlowSyncRange(), cfg.Analyzers.Sapphire.BatchSize, analyzer.SlowSyncMode, sourceClient, dbClient, logger)
		})
	}
	if cfg.Analyzers.Pontusx != nil {
		analyzers, err = addAnalyzer(analyzers, err, syncTagPontusx, func() (A, error) {
			runtimeMetadata := cfg.Source.SDKParaTime(common.RuntimePontusx)
			sourceClient, err1 := sources.Runtime(ctx, common.RuntimePontusx)
			if err1 != nil {
				return nil, err1
			}
			return runtime.NewRuntimeAnalyzer(common.RuntimePontusx, runtimeMetadata, cfg.Analyzers.Pontusx.SlowSyncRange(), cfg.Analyzers.Pontusx.BatchSize, analyzer.SlowSyncMode, sourceClient, dbClient, logger)
		})
	}
	if cfg.Analyzers.Cipher != nil {
		analyzers, err = addAnalyzer(analyzers, err, syncTagCipher, func() (A, error) {
			runtimeMetadata := cfg.Source.SDKParaTime(common.RuntimeCipher)
			sourceClient, err1 := sources.Runtime(ctx, common.RuntimeCipher)
			if err1 != nil {
				return nil, err1
			}
			return runtime.NewRuntimeAnalyzer(common.RuntimeCipher, runtimeMetadata, cfg.Analyzers.Cipher.SlowSyncRange(), cfg.Analyzers.Cipher.BatchSize, analyzer.SlowSyncMode, sourceClient, dbClient, logger)
		})
	}
	if cfg.Analyzers.EmeraldEvmTokens != nil {
		analyzers, err = addAnalyzer(analyzers, err, syncTagEmerald, func() (A, error) {
			sourceClient, err1 := sources.Runtime(ctx, common.RuntimeEmerald)
			if err1 != nil {
				return nil, err1
			}
			return evmtokens.NewAnalyzer(common.RuntimeEmerald, cfg.Analyzers.EmeraldEvmTokens.ItemBasedAnalyzerConfig, sourceClient, dbClient, logger)
		})
	}
	if cfg.Analyzers.SapphireEvmTokens != nil {
		analyzers, err = addAnalyzer(analyzers, err, syncTagSapphire, func() (A, error) {
			sourceClient, err1 := sources.Runtime(ctx, common.RuntimeSapphire)
			if err1 != nil {
				return nil, err1
			}
			return evmtokens.NewAnalyzer(common.RuntimeSapphire, cfg.Analyzers.SapphireEvmTokens.ItemBasedAnalyzerConfig, sourceClient, dbClient, logger)
		})
	}
	if cfg.Analyzers.PontusxEvmTokens != nil {
		analyzers, err = addAnalyzer(analyzers, err, syncTagPontusx, func() (A, error) {
			sourceClient, err1 := sources.Runtime(ctx, common.RuntimePontusx)
			if err1 != nil {
				return nil, err1
			}
			return evmtokens.NewAnalyzer(common.RuntimePontusx, cfg.Analyzers.PontusxEvmTokens.ItemBasedAnalyzerConfig, sourceClient, dbClient, logger)
		})
	}
	if cfg.Analyzers.EmeraldEvmNfts != nil {
		analyzers, err = addAnalyzer(analyzers, err, syncTagEmerald, func() (A, error) {
			sourceClient, err1 := sources.Runtime(ctx, common.RuntimeEmerald)
			if err1 != nil {
				return nil, err1
			}
			ipfsClient, err1 := sources.IPFS(ctx)
			if err1 != nil {
				return nil, err1
			}
			return evmnfts.NewAnalyzer(common.RuntimeEmerald, cfg.Analyzers.EmeraldEvmNfts.ItemBasedAnalyzerConfig, sourceClient, ipfsClient, dbClient, logger)
		})
	}
	if cfg.Analyzers.SapphireEvmNfts != nil {
		analyzers, err = addAnalyzer(analyzers, err, syncTagSapphire, func() (A, error) {
			sourceClient, err1 := sources.Runtime(ctx, common.RuntimeSapphire)
			if err1 != nil {
				return nil, err1
			}
			ipfsClient, err1 := sources.IPFS(ctx)
			if err1 != nil {
				return nil, err1
			}
			return evmnfts.NewAnalyzer(common.RuntimeSapphire, cfg.Analyzers.SapphireEvmNfts.ItemBasedAnalyzerConfig, sourceClient, ipfsClient, dbClient, logger)
		})
	}
	if cfg.Analyzers.PontusxEvmNfts != nil {
		analyzers, err = addAnalyzer(analyzers, err, syncTagPontusx, func() (A, error) {
			sourceClient, err1 := sources.Runtime(ctx, common.RuntimePontusx)
			if err1 != nil {
				return nil, err1
			}
			ipfsClient, err1 := sources.IPFS(ctx)
			if err1 != nil {
				return nil, err1
			}
			return evmnfts.NewAnalyzer(common.RuntimePontusx, cfg.Analyzers.PontusxEvmNfts.ItemBasedAnalyzerConfig, sourceClient, ipfsClient, dbClient, logger)
		})
	}
	if cfg.Analyzers.EmeraldEvmTokenBalances != nil {
		runtimeMetadata := cfg.Source.SDKParaTime(common.RuntimeEmerald)
		analyzers, err = addAnalyzer(analyzers, err, syncTagEmerald, func() (A, error) {
			sourceClient, err1 := sources.Runtime(ctx, common.RuntimeEmerald)
			if err1 != nil {
				return nil, err1
			}
			return evmtokenbalances.NewAnalyzer(common.RuntimeEmerald, cfg.Analyzers.EmeraldEvmTokenBalances.ItemBasedAnalyzerConfig, runtimeMetadata, sourceClient, dbClient, logger)
		})
	}
	if cfg.Analyzers.SapphireEvmTokenBalances != nil {
		runtimeMetadata := cfg.Source.SDKParaTime(common.RuntimeSapphire)
		analyzers, err = addAnalyzer(analyzers, err, syncTagSapphire, func() (A, error) {
			sourceClient, err1 := sources.Runtime(ctx, common.RuntimeSapphire)
			if err1 != nil {
				return nil, err1
			}
			return evmtokenbalances.NewAnalyzer(common.RuntimeSapphire, cfg.Analyzers.SapphireEvmTokenBalances.ItemBasedAnalyzerConfig, runtimeMetadata, sourceClient, dbClient, logger)
		})
	}
	if cfg.Analyzers.PontusxEvmTokenBalances != nil {
		runtimeMetadata := cfg.Source.SDKParaTime(common.RuntimePontusx)
		analyzers, err = addAnalyzer(analyzers, err, syncTagPontusx, func() (A, error) {
			sourceClient, err1 := sources.Runtime(ctx, common.RuntimePontusx)
			if err1 != nil {
				return nil, err1
			}
			return evmtokenbalances.NewAnalyzer(common.RuntimePontusx, cfg.Analyzers.PontusxEvmTokenBalances.ItemBasedAnalyzerConfig, runtimeMetadata, sourceClient, dbClient, logger)
		})
	}
	if cfg.Analyzers.EmeraldContractCode != nil {
		analyzers, err = addAnalyzer(analyzers, err, syncTagEmerald, func() (A, error) {
			sourceClient, err1 := sources.Runtime(ctx, common.RuntimeEmerald)
			if err1 != nil {
				return nil, err1
			}
			return evmcontractcode.NewAnalyzer(common.RuntimeEmerald, cfg.Analyzers.EmeraldContractCode.ItemBasedAnalyzerConfig, sourceClient, dbClient, logger)
		})
	}
	if cfg.Analyzers.SapphireContractCode != nil {
		analyzers, err = addAnalyzer(analyzers, err, syncTagSapphire, func() (A, error) {
			sourceClient, err1 := sources.Runtime(ctx, common.RuntimeSapphire)
			if err1 != nil {
				return nil, err1
			}
			return evmcontractcode.NewAnalyzer(common.RuntimeSapphire, cfg.Analyzers.SapphireContractCode.ItemBasedAnalyzerConfig, sourceClient, dbClient, logger)
		})
	}
	if cfg.Analyzers.PontusxContractCode != nil {
		analyzers, err = addAnalyzer(analyzers, err, syncTagPontusx, func() (A, error) {
			sourceClient, err1 := sources.Runtime(ctx, common.RuntimePontusx)
			if err1 != nil {
				return nil, err1
			}
			return evmcontractcode.NewAnalyzer(common.RuntimePontusx, cfg.Analyzers.PontusxContractCode.ItemBasedAnalyzerConfig, sourceClient, dbClient, logger)
		})
	}
	if cfg.Analyzers.EmeraldContractVerifier != nil {
		analyzers, err = addAnalyzer(analyzers, err, syncTagEmerald, func() (A, error) {
			return evmverifier.NewAnalyzer(cfg.Source.ChainName, common.RuntimeEmerald, cfg.Analyzers.EmeraldContractVerifier.ItemBasedAnalyzerConfig, cfg.Analyzers.EmeraldContractVerifier.SourcifyServerUrl, dbClient, logger)
		})
	}
	if cfg.Analyzers.SapphireContractVerifier != nil {
		analyzers, err = addAnalyzer(analyzers, err, syncTagSapphire, func() (A, error) {
			return evmverifier.NewAnalyzer(cfg.Source.ChainName, common.RuntimeSapphire, cfg.Analyzers.SapphireContractVerifier.ItemBasedAnalyzerConfig, cfg.Analyzers.SapphireContractVerifier.SourcifyServerUrl, dbClient, logger)
		})
	}
	if cfg.Analyzers.PontusxContractVerifier != nil {
		analyzers, err = addAnalyzer(analyzers, err, syncTagPontusx, func() (A, error) {
			return evmverifier.NewAnalyzer(cfg.Source.ChainName, common.RuntimePontusx, cfg.Analyzers.PontusxContractVerifier.ItemBasedAnalyzerConfig, cfg.Analyzers.PontusxContractVerifier.SourcifyServerUrl, dbClient, logger)
		})
	}
	if cfg.Analyzers.EmeraldAbi != nil {
		analyzers, err = addAnalyzer(analyzers, err, syncTagEmerald, func() (A, error) {
			return evmabibackfill.NewAnalyzer(common.RuntimeEmerald, cfg.Analyzers.EmeraldAbi.ItemBasedAnalyzerConfig, dbClient, logger)
		})
	}
	if cfg.Analyzers.SapphireAbi != nil {
		analyzers, err = addAnalyzer(analyzers, err, syncTagSapphire, func() (A, error) {
			return evmabibackfill.NewAnalyzer(common.RuntimeSapphire, cfg.Analyzers.SapphireAbi.ItemBasedAnalyzerConfig, dbClient, logger)
		})
	}
	if cfg.Analyzers.PontusxAbi != nil {
		analyzers, err = addAnalyzer(analyzers, err, syncTagPontusx, func() (A, error) {
			return evmabibackfill.NewAnalyzer(common.RuntimePontusx, cfg.Analyzers.PontusxAbi.ItemBasedAnalyzerConfig, dbClient, logger)
		})
	}
	if cfg.Analyzers.MetadataRegistry != nil {
		analyzers, err = addAnalyzer(analyzers, err, "" /*syncTag*/, func() (A, error) {
			return metadata_registry.NewAnalyzer(cfg.Analyzers.MetadataRegistry.ItemBasedAnalyzerConfig, dbClient, logger)
		})
	}
	if cfg.Analyzers.NodeStats != nil {
		analyzers, err = addAnalyzer(analyzers, err, "" /*syncTag*/, func() (A, error) {
			sourceClient, err1 := sources.Consensus(ctx)
			if err1 != nil {
				return nil, err1
			}
			emeraldClient, err1 := sources.Runtime(ctx, common.RuntimeEmerald)
			if err1 != nil {
				return nil, err1
			}
			sapphireClient, err1 := sources.Runtime(ctx, common.RuntimeSapphire)
			if err1 != nil {
				return nil, err1
			}
			return nodestats.NewAnalyzer(cfg.Analyzers.NodeStats.ItemBasedAnalyzerConfig, cfg.Analyzers.NodeStats.Layers, sourceClient, emeraldClient, sapphireClient, dbClient, logger)
		})
	}
	if cfg.Analyzers.AggregateStats != nil {
		analyzers, err = addAnalyzer(analyzers, err, "" /*syncTag*/, func() (A, error) {
			return aggregate_stats.NewAggregateStatsAnalyzer(dbClient, logger)
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

// Start starts the analysis service.
func (a *Service) Start() {
	defer a.cleanup()
	a.logger.Info("starting analysis service")

	ctx, cancelAnalyzers := context.WithCancel(context.Background())
	defer cancelAnalyzers() // Start() only returns when analyzers are done, so this should be a no-op, but it makes the compiler happier.

	// Start fast-sync analyzers.
	fastSyncWg := map[string]*sync.WaitGroup{} // syncTag -> wg with all fast-sync analyzers with that tag
	for _, an := range a.fastSyncAnalyzers {
		wg, ok := fastSyncWg[an.SyncTag]
		if !ok {
			wg = &sync.WaitGroup{}
			fastSyncWg[an.SyncTag] = wg
		}
		wg.Add(1)
		go func(an SyncedAnalyzer) {
			defer wg.Done()
			an.Analyzer.Start(ctx)
		}(an)
	}

	// Prepare slow-sync analyzers (to be started after fast-sync analyzers are done).
	var slowSyncWg sync.WaitGroup
	for _, an := range a.analyzers {
		slowSyncWg.Add(1)
		go func(an SyncedAnalyzer) {
			defer slowSyncWg.Done()

			// Find the wait group for this analyzer's sync tag.
			prereqWg, ok := fastSyncWg[an.SyncTag]
			if !ok || an.SyncTag == "" {
				// No fast-sync analyzers with this tag, start the analyzer immediately.
				prereqWg = &sync.WaitGroup{}
			}

			// Start the analyzer after fast-sync analyzers,
			// unless the context is canceled first (e.g. by ctrl+C during fast-sync).
			select {
			case <-ctx.Done():
				return
			case <-util.ClosingChannel(prereqWg):
				an.Analyzer.Start(ctx)
			}
		}(an)
	}
	analyzersDone := util.ClosingChannel(&slowSyncWg)

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
	if a.sources == nil {
		return
	}

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
