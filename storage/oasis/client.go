// Package oasis implements the source storage interface
// backed by oasis-node.
package oasis

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/oasisprotocol/nexus/common"
	"github.com/oasisprotocol/nexus/config"
	"github.com/oasisprotocol/nexus/log"
	"github.com/oasisprotocol/nexus/storage/oasis/nodeapi"
	"github.com/oasisprotocol/nexus/storage/oasis/nodeapi/file"
	"github.com/oasisprotocol/nexus/storage/oasis/nodeapi/history"
)

// NewConsensusClient creates a new ConsensusClient.
func NewConsensusClient(ctx context.Context, sourceConfig *config.SourceConfig) (nodeapi.ConsensusApiLite, error) {
	// Create an API that connects to the real node, then wrap it in a caching layer.
	var nodeApi nodeapi.ConsensusApiLite
	nodeApi, err := history.NewHistoryConsensusApiLite(ctx, sourceConfig.History(), sourceConfig.Nodes, sourceConfig.FastStartup)
	if err != nil {
		return nil, fmt.Errorf("instantiating history consensus API lite: %w", err)
	}
	if sourceConfig.Cache != nil {
		cachePath := filepath.Join(sourceConfig.Cache.CacheDir, "consensus")
		wrappedNodeApi, err := file.NewFileConsensusApiLite(cachePath, nodeApi)
		if err != nil {
			// Continue without a cache, but warn.
			// (The "analyzer" tag is not technically true as we're not in an analyzer, but is helpful when reading logs)
			log.NewDefaultLogger("init").With("analyzer", "consensus").Warn("error instantiating cache-based consensusApi, continuing without a cache", "kvstore_err", err)
		} else {
			nodeApi = wrappedNodeApi
		}
	}
	return nodeApi, nil
}

// NewRuntimeClient creates a new RuntimeClient.
func NewRuntimeClient(ctx context.Context, sourceConfig *config.SourceConfig, runtime common.Runtime) (nodeapi.RuntimeApiLite, error) {
	var nodeApi nodeapi.RuntimeApiLite
	nodeApi, err := history.NewHistoryRuntimeApiLite(ctx, sourceConfig.History(), sourceConfig.SDKParaTime(runtime), sourceConfig.Nodes, sourceConfig.FastStartup, runtime)
	if err != nil {
		return nil, fmt.Errorf("instantiating history runtime API lite: %w", err)
	}
	if sourceConfig.Cache != nil {
		cachePath := filepath.Join(sourceConfig.Cache.CacheDir, string(runtime))
		wrappedNodeApi, err := file.NewFileRuntimeApiLite(runtime, cachePath, nodeApi)
		if err != nil {
			// Continue without a cache, but warn.
			// (The "analyzer" tag is not technically true as we're not in an analyzer, but is helpful when reading logs)
			log.NewDefaultLogger("init").With("analyzer", "consensus").Warn("error instantiating cache-based runtimeApi, continuing without a cache", "kvstore_err", err)
		} else {
			nodeApi = wrappedNodeApi
		}
	}

	return nodeApi, nil
}
