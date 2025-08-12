package config

import (
	"testing"
	"time"

	"github.com/knadh/koanf/providers/rawbytes"
	"github.com/stretchr/testify/require"

	"github.com/oasisprotocol/nexus/common"
)

func TestServerConfigJSON(t *testing.T) {
	// Example config.
	requestTimeout := 30 * time.Second
	cfg := ServerConfig{
		Endpoint: "localhost:8080",
		Storage: &StorageConfig{
			Endpoint: "postgresql://user:pass@localhost:5432/db",
			Backend:  "postgres",
		},
		Source: &SourceConfig{
			ChainName: "mainnet",
			Nodes: map[string]*ArchiveConfig{
				"eden": {
					DefaultNode: &NodeConfig{
						RPC: "http://localhost:8546",
					},
				},
			},
		},
		RequestTimeout: &requestTimeout,
		EVMTokensCustomOrdering: map[common.Runtime][][]string{
			"emerald": {
				{"0x1234", "0x5678"},
				{"0xabcd"},
			},
		},
	}

	// Example config in yaml format.
	expectedYAML := `
server:
  endpoint: localhost:8080
  storage:
    endpoint: postgresql://user:pass@localhost:5432/db
    backend: postgres
    DANGER__WIPE_STORAGE_ON_STARTUP: false
  source:
    chain_name: mainnet
    nodes:
      eden:
        default:
          rpc: http://localhost:8546
  request_timeout: 30000000000
  evm_tokens_custom_ordering:
    emerald:
      - ["0x1234", "0x5678"]
      - ["0xabcd"]
`

	deserializedCfg, err := initConfig(rawbytes.Provider([]byte(expectedYAML)))
	require.NoError(t, err)

	require.Equal(t, cfg.Endpoint, deserializedCfg.Server.Endpoint)
	require.Equal(t, cfg.Storage, deserializedCfg.Server.Storage)
	require.Equal(t, cfg.Source, deserializedCfg.Server.Source)
	require.Equal(t, cfg.RequestTimeout, deserializedCfg.Server.RequestTimeout)
	require.Equal(t, cfg.EVMTokensCustomOrdering, deserializedCfg.Server.EVMTokensCustomOrdering)
}
