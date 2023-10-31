// Package config enables config file parsing.
package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/knadh/koanf"
	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/providers/file"
	sdkConfig "github.com/oasisprotocol/oasis-sdk/client-sdk/go/config"

	"github.com/oasisprotocol/nexus/common"
	"github.com/oasisprotocol/nexus/log"
)

// Config contains the CLI configuration.
type Config struct {
	Analysis *AnalysisConfig `koanf:"analysis"`
	Server   *ServerConfig   `koanf:"server"`
	Log      *LogConfig      `koanf:"log"`
	Metrics  *MetricsConfig  `koanf:"metrics"`
}

// Validate performs config validation.
func (cfg *Config) Validate() error {
	if cfg.Analysis != nil {
		if err := cfg.Analysis.Validate(); err != nil {
			return fmt.Errorf("analysis: %w", err)
		}
	}
	if cfg.Server != nil {
		if err := cfg.Server.Validate(); err != nil {
			return fmt.Errorf("server: %w", err)
		}
	}
	if cfg.Log != nil {
		if err := cfg.Log.Validate(); err != nil {
			return fmt.Errorf("log: %w", err)
		}
	}
	if cfg.Metrics != nil {
		if err := cfg.Metrics.Validate(); err != nil {
			return fmt.Errorf("metrics: %w", err)
		}
	}

	return nil
}

// AnalysisConfig is the configuration for chain analyzers.
type AnalysisConfig struct {
	// Source is the configuration for accessing oasis-node(s).
	Source SourceConfig `koanf:"source"`

	// Analyzers is the analyzer configs.
	Analyzers AnalyzersList `koanf:"analyzers"`

	Storage *StorageConfig `koanf:"storage"`
}

// Validate validates the analysis configuration.
func (cfg *AnalysisConfig) Validate() error {
	if cfg.Source.ChainName == "" && cfg.Source.CustomChain == nil {
		return fmt.Errorf("source not configured, specify either source.chain_name or source.custom_chain")
	} else if cfg.Source.ChainName != "" && cfg.Source.CustomChain != nil {
		return fmt.Errorf("source.chain_name and source.custom_chain specified, can only use one")
	}
	for archiveName, archiveConfig := range cfg.Source.Nodes {
		if archiveConfig.DefaultNode == nil && archiveConfig.ConsensusNode == nil && len(archiveConfig.RuntimeNodes) == 0 {
			return fmt.Errorf("source.nodes[%v] has none of .default, .consensus, or .runtimes", archiveName)
		}
	}
	if cfg.Analyzers.Consensus != nil {
		if err := cfg.Analyzers.Consensus.Validate(); err != nil {
			return err
		}
	}
	if cfg.Analyzers.Emerald != nil {
		if err := cfg.Analyzers.Emerald.Validate(); err != nil {
			return err
		}
	}
	if cfg.Analyzers.Sapphire != nil {
		if err := cfg.Analyzers.Sapphire.Validate(); err != nil {
			return err
		}
	}
	if cfg.Analyzers.Cipher != nil {
		if err := cfg.Analyzers.Cipher.Validate(); err != nil {
			return err
		}
	}
	if cfg.Analyzers.MetadataRegistry != nil {
		if err := cfg.Analyzers.MetadataRegistry.Validate(); err != nil {
			return err
		}
	}
	if cfg.Analyzers.EmeraldContractVerifier != nil {
		if err := cfg.Analyzers.EmeraldContractVerifier.Validate(); err != nil {
			return err
		}
	}
	if cfg.Analyzers.SapphireContractVerifier != nil {
		if err := cfg.Analyzers.SapphireContractVerifier.Validate(); err != nil {
			return err
		}
	}
	if cfg.Analyzers.NodeStats != nil {
		if err := cfg.Analyzers.NodeStats.Validate(); err != nil {
			return err
		}
	}
	if cfg.Analyzers.AggregateStats != nil {
		if err := cfg.Analyzers.AggregateStats.Validate(); err != nil {
			return err
		}
	}

	return cfg.Storage.Validate(true /* requireMigrations */)
}

type AnalyzersList struct {
	Consensus *BlockBasedAnalyzerConfig `koanf:"consensus"`
	Emerald   *BlockBasedAnalyzerConfig `koanf:"emerald"`
	Sapphire  *BlockBasedAnalyzerConfig `koanf:"sapphire"`
	Cipher    *BlockBasedAnalyzerConfig `koanf:"cipher"`

	EmeraldEvmTokens         *EvmTokensAnalyzerConfig       `koanf:"evm_tokens_emerald"`
	SapphireEvmTokens        *EvmTokensAnalyzerConfig       `koanf:"evm_tokens_sapphire"`
	EmeraldEvmNfts           *EvmTokensAnalyzerConfig       `koanf:"evm_nfts_emerald"`
	SapphireEvmNfts          *EvmTokensAnalyzerConfig       `koanf:"evm_nfts_sapphire"`
	EmeraldEvmTokenBalances  *EvmTokensAnalyzerConfig       `koanf:"evm_token_balances_emerald"`
	SapphireEvmTokenBalances *EvmTokensAnalyzerConfig       `koanf:"evm_token_balances_sapphire"`
	EmeraldContractCode      *EvmContractCodeAnalyzerConfig `koanf:"evm_contract_code_emerald"`
	SapphireContractCode     *EvmContractCodeAnalyzerConfig `koanf:"evm_contract_code_sapphire"`
	EmeraldContractVerifier  *EVMContractVerifierConfig     `koanf:"evm_contract_verifier_emerald"`
	SapphireContractVerifier *EVMContractVerifierConfig     `koanf:"evm_contract_verifier_sapphire"`

	MetadataRegistry *MetadataRegistryConfig `koanf:"metadata_registry"`
	NodeStats        *NodeStatsConfig        `koanf:"node_stats"`
	AggregateStats   *AggregateStatsConfig   `koanf:"aggregate_stats"`
}

// SourceConfig has some controls about what chain we're analyzing and how to connect.
type SourceConfig struct {
	// Cache holds the configuration for a file-based caching backend.
	Cache *CacheConfig `koanf:"cache"`

	// ChainName is the name of the chain (e.g. mainnet/testnet). Set
	// this to use one of the default chains.
	ChainName common.ChainName `koanf:"chain_name"`
	// CustomChain is information about a custom chain. Set this to use a
	// chain other than the default chains, e.g. a local testnet.
	CustomChain *CustomChainConfig `koanf:"custom_chain"`

	// Nodes describe the oasis-node(s) to connect to. Keys are "archive
	// names," which are named after mainnet releases, in lowercase e.g.
	// "cobalt" and "damask."
	Nodes map[string]*ArchiveConfig `koanf:"nodes"`

	// IPFS holds the configuration for accessing IPFS.
	IPFS *IPFSConfig `koanf:"ipfs"`

	// If set, the analyzer will skip some initial checks, e.g. that
	// `rpc` really serves the chain with the chain context we expect.
	// NOT RECOMMENDED in production; intended for faster testing.
	FastStartup bool `koanf:"fast_startup"`
}

func (sc *SourceConfig) History() *History {
	if sc.ChainName != "" {
		return DefaultChains[sc.ChainName]
	}
	return sc.CustomChain.History
}

func (sc *SourceConfig) SDKNetwork() *sdkConfig.Network {
	if sc.ChainName != "" {
		return sdkConfig.DefaultNetworks.All[string(sc.ChainName)]
	}
	return sc.CustomChain.SDKNetwork
}

func (sc *SourceConfig) SDKParaTime(runtime common.Runtime) *sdkConfig.ParaTime {
	return sc.SDKNetwork().ParaTimes.All[string(runtime)]
}

func CustomSingleNetworkSourceConfig(rpc string, chainContext string) *SourceConfig {
	return &SourceConfig{
		CustomChain: &CustomChainConfig{
			History:    SingleRecordHistory(chainContext),
			SDKNetwork: &sdkConfig.Network{},
		},
		Nodes:       SingleNetworkLookup(rpc),
		FastStartup: true,
	}
}

func SingleNetworkLookup(rpc string) map[string]*ArchiveConfig {
	return map[string]*ArchiveConfig{
		"damask": {
			DefaultNode: &NodeConfig{
				RPC: rpc,
			},
		},
	}
}

type CacheConfig struct {
	// CacheDir is the directory where the cache data is stored
	CacheDir string `koanf:"cache_dir"`
}

func (cfg *CacheConfig) Validate() error {
	if cfg.CacheDir == "" {
		return fmt.Errorf("invalid cache filepath")
	}
	return nil
}

type CustomChainConfig struct {
	// History is the sequence of networks in the chain.
	History *History `koanf:"history"`
	// SDKNetwork is the oasis-sdk Network configuration of the latest
	// network in the chain.
	SDKNetwork *sdkConfig.Network `koanf:"sdk_network"`
}

// ArchiveConfig is information about the nodes for a network.
type ArchiveConfig struct {
	// DefaultNode is information about the node to get data from by default.
	DefaultNode *NodeConfig `koanf:"default"`
	// ConsensusNode is information about the node to get consensus data from,
	// instead of the default node.
	ConsensusNode *NodeConfig `koanf:"consensus"`
	// RuntimeNodes is the information about the nodes to get runtime data
	// from, instead of the default node.
	RuntimeNodes map[common.Runtime]*NodeConfig `koanf:"runtimes"`
}

func (ac *ArchiveConfig) ResolvedConsensusNode() *NodeConfig {
	nc := ac.ConsensusNode
	if nc != nil {
		return nc
	}
	return ac.DefaultNode
}

func (ac *ArchiveConfig) ResolvedRuntimeNode(runtime common.Runtime) *NodeConfig {
	nc := ac.RuntimeNodes[runtime]
	if nc != nil {
		return nc
	}
	return ac.DefaultNode
}

// NodeConfig is information about one oasis-node to connect to.
type NodeConfig struct {
	// RPC is the node endpoint.
	RPC string `koanf:"rpc"`
}

// IPFSConfig is information about accessing IPFS.
type IPFSConfig struct {
	// Gateway is the URL prefix of an IPFS HTTP gateway. Do not include a
	// trailing slash, e.g. `https://ipfs.io`. Something like
	// `/ipfs/xxxx/n.json` will be appended.
	Gateway string `koanf:"gateway"`
}

type BlockBasedAnalyzerConfig struct {
	// From is the (inclusive) starting block for this analyzer.
	From uint64 `koanf:"from"`

	// If present, the analyzer will run in fast sync mode for
	// an initial range of blocks, and then switch to slow sync.
	// If absent, the analyzer will only run in "slow", sequential mode.
	//
	// If fast sync mode is enabled:
	// - DB checks and foreign keys are disabled.
	// - Multiple analyzers run in parallel and process blocks out of order.
	// - Analyzers do not perform dead reckoning on state (notably, transfers).
	// After all analyzers finish the fast sync range:
	// - DB checks and foreign keys are re-enabled.
	// - State that would normally be dead-reckoned is fetched directly from the node
	//   via the StateToGenesis() RPC, or recalculated from the relevant DB entries.
	// - A single "slow mode" analyzer (per runtime/consensus) is started, and resumes
	//   to process blocks sequentially, with dead reckoning enabled.
	FastSync *FastSyncConfig `koanf:"fast_sync"`

	// To is the (inclusive) ending block for this analyzer.
	// Omitting this parameter or setting it to 0 means this analyzer will
	// continue processing new blocks forever.
	To uint64 `koanf:"to"`

	// BatchSize determines the maximum number of blocks the block analyzer
	// processes per batch. This is relevant only when the analyzer is
	// still catching up to the latest block.
	// Optimal value depends on block processing speed. Ideally, it should
	// be set to the number of blocks processed within 30-60 seconds.
	//
	// Uses default value of 1000 if unset/set to 0.
	BatchSize uint64 `koanf:"batch_size"`
}

type FastSyncConfig struct {
	// To is the block (inclusive) to which the analyzer
	// will run in fast sync mode. From this block onwards (exclusive),
	// the analyzer will run in slow mode.
	To uint64 `koanf:"to"`

	// The number of parallel analyzers to run in fast sync mode.
	Parallelism int `koanf:"parallelism"`
}

func (blockCfg BlockBasedAnalyzerConfig) FastSyncRange() *BlockRange {
	if blockCfg.FastSync == nil {
		return nil
	}
	return &BlockRange{
		From: blockCfg.From,
		To:   blockCfg.FastSync.To,
	}
}

func (blockCfg BlockBasedAnalyzerConfig) SlowSyncRange() BlockRange {
	if blockCfg.FastSync == nil {
		return BlockRange{
			From: blockCfg.From,
			To:   blockCfg.To,
		}
	}
	return BlockRange{
		From: blockCfg.FastSync.To + 1,
		To:   blockCfg.To,
	}
}

func (blockCfg BlockBasedAnalyzerConfig) Validate() error {
	// Check non-range constraints.
	if blockCfg.FastSync != nil && blockCfg.FastSync.To == 0 {
		return fmt.Errorf("invalid block range: .fast_sync.to must be > 0")
	}
	if blockCfg.FastSync != nil && blockCfg.FastSync.Parallelism <= 0 {
		return fmt.Errorf("invalid parallelism level: must be > 0")
	}

	// Check that the slow sync range and fast sync range are valid.
	if err := blockCfg.SlowSyncRange().Validate(); err != nil {
		return err
	}
	if f := blockCfg.FastSyncRange(); f != nil {
		if err := f.Validate(); err != nil {
			return err
		}
	}
	return nil
}

type BlockRange struct {
	From uint64 // Inclusive.
	To   uint64 // Inclusive.
}

func (r BlockRange) Validate() error {
	if r.To != 0 && r.From > r.To {
		return fmt.Errorf("invalid block range from %d to %d", r.From, r.To)
	}
	return nil
}

type ItemBasedAnalyzerConfig struct {
	// BatchSize determines the maximum number of items the item analyzer
	// processes per batch. This is relevant only when the analyzer
	// work queue is sufficiently long, most often when the analyzer is first enabled after
	// the block analyzers have been running for some time.
	//
	// Optimal value depends on the item processing speed. Ideally, it should
	// be set to the number of items that can be processed within 60 seconds.
	// Note that for some items, the processor will request information from out-of-band
	// sources such as Sourcify/Github. Care should be taken not to flood them
	// with too many requests.
	//
	// Uses default value of 20 if unset/set to 0.
	BatchSize uint64 `koanf:"batch_size"`

	// If StopOnEmptyQueue is true, the analyzer will exit the main processing loop when
	// there are no items left in the work queue. This is useful during testing when
	// 1) The number of items in the queue is determinate
	// 2) We want the analyzer to naturally terminate after processing all available work items.
	//
	// Defaults to false.
	StopOnEmptyQueue bool `koanf:"stop_on_empty_queue"`

	// If Interval is set, the analyzer will process batches at a fixed cadence specified by Interval.
	// Otherwise, the analyzer will use an adaptive backoff to determine the delay between
	// batches. Note that the backoff will attempt to run as fast as possible.
	Interval time.Duration `koanf:"interval"`

	// InterItemDelay determines the length of time the analyzer waits between spawning
	// new goroutines to process items within a batch. This is useful for cases where
	// processItem() makes out of band requests to rate limited resources.
	InterItemDelay time.Duration `koanf:"inter_item_delay"`
}

type EvmTokensAnalyzerConfig struct {
	ItemBasedAnalyzerConfig `koanf:",squash"`
}

type EvmContractCodeAnalyzerConfig struct {
	ItemBasedAnalyzerConfig `koanf:",squash"`
}

// EVMContractVerifierConfig is the configuration for the EVM contracts verifier analyzer.
type EVMContractVerifierConfig struct {
	ItemBasedAnalyzerConfig `koanf:",squash"`

	// ChainName is the name of the chain (e.g. mainnet/testnet).
	// The analyzer only supports testnet/mainnet chains.
	ChainName common.ChainName `koanf:"chain_name"`

	// SourcifyServerUrl is the base URL of the Sourcify server.
	SourcifyServerUrl string `koanf:"sourcify_server_url"`
}

// Validate validates the evm contracts verifier config.
func (cfg *EVMContractVerifierConfig) Validate() error {
	return nil
}

// MetadataRegistryConfig is the configuration for the metadata registry analyzer.
type MetadataRegistryConfig struct {
	ItemBasedAnalyzerConfig `koanf:",squash"`
}

// Validate validates the configuration.
func (cfg *MetadataRegistryConfig) Validate() error {
	if cfg.Interval < time.Minute {
		return fmt.Errorf("metadata registry interval must be at least 1 minute")
	}
	return nil
}

// NodeStatsConfig is the configuration for the node stats analyzer.
type NodeStatsConfig struct {
	ItemBasedAnalyzerConfig `koanf:",squash"`
}

func (cfg *NodeStatsConfig) Validate() error {
	if cfg.Interval > 6*time.Second {
		return fmt.Errorf("node stats interval must be less than or equal to the block time of 6 seconds")
	}
	return nil
}

// AggregateStatsConfig is the configuration for the aggregate stats analyzer.
type AggregateStatsConfig struct{}

func (cfg *AggregateStatsConfig) Validate() error {
	return nil
}

// ServerConfig contains the API server configuration.
type ServerConfig struct {
	// ChainName is the name of the chain (i.e. mainnet/testnet). Custom/local nets are not supported.
	// This is only used for the runtime status endpoint.
	ChainName common.ChainName `koanf:"chain_name"`

	// Endpoint is the service endpoint from which to serve the API.
	Endpoint string `koanf:"endpoint"`

	Storage *StorageConfig `koanf:"storage"`
}

// Validate validates the server configuration.
func (cfg *ServerConfig) Validate() error {
	if cfg.Endpoint == "" {
		return fmt.Errorf("malformed server endpoint '%s'", cfg.Endpoint)
	}
	if cfg.Storage == nil {
		return fmt.Errorf("no storage config provided")
	}
	if cfg.ChainName == "" {
		return fmt.Errorf("no chain name provided")
	}

	return cfg.Storage.Validate(false /* requireMigrations */)
}

// StorageBackend is a storage backend.
type StorageBackend uint

const (
	// BackendCockroach is the CockroachDB storage backend.
	BackendCockroach StorageBackend = iota
	// BackendPostgres is the PostgreSQL storage backend.
	BackendPostgres
	// BackendInMemory is the in-memory storage backend.
	BackendInMemory
)

// String returns the string representation of a StorageBackend.
func (sb *StorageBackend) String() string {
	switch *sb {
	case BackendCockroach:
		return "cockroach"
	case BackendPostgres:
		return "postgres"
	case BackendInMemory:
		return "inmemory"
	default:
		panic("config: unsupported storage backend")
	}
}

// Set sets the StorageBackend to the value specified by the provided string.
func (sb *StorageBackend) Set(s string) error {
	switch strings.ToLower(s) {
	case "cockroach":
		*sb = BackendCockroach
	case "postgres":
		*sb = BackendPostgres
	case "inmemory":
		*sb = BackendInMemory
	default:
		return fmt.Errorf("config: invalid storage backend: '%s'", s)
	}

	return nil
}

// Type returns the list of supported StorageBackends.
func (sb *StorageBackend) Type() string {
	return "[cockroach,postgres,inmemory]"
}

// StorageConfig contains the storage layer configuration.
type StorageConfig struct {
	// Endpoint is the storage endpoint from which to read/write indexed data.
	Endpoint string `koanf:"endpoint"`

	// Backend is the storage backend to select.
	Backend string `koanf:"backend"`

	// Migrations is the directory containing schema migrations.
	Migrations string `koanf:"migrations"`

	// If true, we'll first delete all tables in the DB to
	// force a full re-index of the blockchain.
	WipeStorage bool `koanf:"DANGER__WIPE_STORAGE_ON_STARTUP"`
}

// Validate validates the storage configuration.
func (cfg *StorageConfig) Validate(requireMigrations bool) error {
	if cfg.Endpoint == "" {
		return fmt.Errorf("malformed storage endpoint '%s'", cfg.Endpoint)
	}
	if cfg.Migrations == "" && requireMigrations {
		return fmt.Errorf("invalid path to migrations '%s'", cfg.Migrations)
	}
	var sb StorageBackend
	return sb.Set(cfg.Backend)
}

// LogConfig contains the logging configuration.
type LogConfig struct {
	Format string `koanf:"format"`
	Level  string `koanf:"level"`
	File   string `koanf:"file"`
}

// Validate validates the logging configuration.
func (cfg *LogConfig) Validate() error {
	var format log.Format
	if err := format.Set(cfg.Format); err != nil {
		return err
	}
	var level log.Level
	return level.Set(cfg.Level)
}

// MetricsConfig contains the metrics configuration.
type MetricsConfig struct {
	PullEndpoint string `koanf:"pull_endpoint"`
}

// Validate validates the metrics configuration.
func (cfg *MetricsConfig) Validate() error {
	if cfg.PullEndpoint == "" {
		return fmt.Errorf("malformed Prometheus pull endpoint '%s'", cfg.PullEndpoint)
	}
	return nil
}

// InitConfig initializes configuration from file.
func InitConfig(f string) (*Config, error) {
	var config Config
	k := koanf.New(".")

	// Load configuration from the yaml config.
	if err := k.Load(file.Provider(f), yaml.Parser()); err != nil {
		return nil, err
	}

	// Load environment variables and merge into the loaded config.
	if err := k.Load(env.Provider("", ".", func(s string) string {
		// `__` is used as a hierarchy delimiter.
		return strings.ReplaceAll(strings.ToLower(s), "__", ".")
	}), nil); err != nil {
		return nil, err
	}

	// Unmarshal into config.
	if err := k.Unmarshal("", &config); err != nil {
		return nil, err
	}

	// Validate config.
	if err := config.Validate(); err != nil {
		return nil, err
	}

	return &config, nil
}
