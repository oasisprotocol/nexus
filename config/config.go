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

	"github.com/oasisprotocol/oasis-indexer/log"
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
	// Node is the configuration for accessing oasis-node.
	Node NodeConfig `koanf:"node"`

	// Analyzers is the analyzer configs.
	Analyzers AnalyzersList `koanf:"analyzers"`

	Storage *StorageConfig `koanf:"storage"`
}

// Validate validates the analysis configuration.
func (cfg *AnalysisConfig) Validate() error {
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

	EmeraldEvmTokens         *EvmTokensAnalyzerConfig `koanf:"evm_tokens_emerald"`
	SapphireEvmTokens        *EvmTokensAnalyzerConfig `koanf:"evm_tokens_sapphire"`
	EmeraldEvmTokenBalances  *EvmTokensAnalyzerConfig `koanf:"evm_token_balances_emerald"`
	SapphireEvmTokenBalances *EvmTokensAnalyzerConfig `koanf:"evm_token_balances_sapphire"`

	MetadataRegistry *MetadataRegistryConfig `koanf:"metadata_registry"`
	AggregateStats   *AggregateStatsConfig   `koanf:"aggregate_stats"`
}

// NodeConfig is the configuration for chain analyzers.
type NodeConfig struct {
	// ChainID is the chain ID of the chain this analyzer will process.
	// Used to identify the chain in the database.
	ChainID string `koanf:"chain_id"`

	// RPC is the node endpoint.
	RPC string `koanf:"rpc"`

	// ChainContext is the domain separation context.
	// It uniquely identifies the chain; it is derived as a hash of the genesis data.
	// Used as safety check to prevent accidental use of the wrong RPC endpoint.
	ChainContext string `koanf:"chaincontext"`

	// If set, the analyzer will skip some initial checks, e.g. that
	// `rpc` really serves the chain with `chain_context`.
	// NOT RECOMMENDED in production; intended for faster testing.
	FastStartup bool `koanf:"fast_startup"`
}

type BlockBasedAnalyzerConfig struct {
	// From is the (inclusive) starting block for this analyzer.
	From int64 `koanf:"from"`

	// To is the (inclusive) ending block for this analyzer.
	// Omitting this parameter means this analyzer will
	// continue processing new blocks until the next breaking
	// upgrade.
	To int64 `koanf:"to"`
}

type IntervalBasedAnalyzerConfig struct {
	// Interval is the time interval at which to run the analyzer.
	Interval time.Duration `koanf:"interval"`
}

type EvmTokensAnalyzerConfig struct{}

// Validate validates the range configuration.
func (cfg *BlockBasedAnalyzerConfig) Validate() error {
	if (cfg.To != 0 && cfg.From > cfg.To) || cfg.To < 0 || cfg.From < 0 {
		return fmt.Errorf("malformed analysis range from %d to %d", cfg.From, cfg.To)
	}
	return nil
}

// MetadataRegistryConfig is the configuration for the metadata registry analyzer.
type MetadataRegistryConfig struct {
	IntervalBasedAnalyzerConfig `koanf:",squash"`
}

// Validate validates the configuration.
func (cfg *MetadataRegistryConfig) Validate() error {
	if cfg.Interval < time.Minute {
		return fmt.Errorf("metadata registry interval must be at least 1 minute")
	}
	return nil
}

// AggregateStatsConfig is the configuration for the aggregate stats analyzer.
type AggregateStatsConfig struct {
	// TxVolumeInterval is the time interval for recomputing the tx volume aggregate stats.
	// We would ideally recompute stats (which include 5-min aggregates) every 5 min, but the
	// current naive implementation is too inefficient for that.
	TxVolumeInterval time.Duration `koanf:"tx_volume_interval"`
}

// Validate validates the aggregate stats config.
func (cfg *AggregateStatsConfig) Validate() error {
	if cfg.TxVolumeInterval < 5*time.Minute {
		return fmt.Errorf("tx_volume_interval must be at least 5 minutes")
	}
	return nil
}

// ServerConfig contains the API server configuration.
type ServerConfig struct {
	// ChainID is the chain ID (normally latest) for the server API
	ChainID string `koanf:"chain_id"`

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
