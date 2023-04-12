// Package oasis implements the source storage interface
// backed by oasis-node.
package oasis

import (
	"context"
	"fmt"

	"github.com/oasisprotocol/oasis-indexer/common"
	"github.com/oasisprotocol/oasis-indexer/config"
	"github.com/oasisprotocol/oasis-indexer/storage/oasis/nodeapi"
	"github.com/oasisprotocol/oasis-indexer/storage/oasis/nodeapi/file"
	"github.com/oasisprotocol/oasis-indexer/storage/oasis/nodeapi/history"
)

const (
	moduleName = "storage_oasis"
)

// NewConsensusClient creates a new ConsensusClient.
func NewConsensusClient(ctx context.Context, sourceConfig *config.SourceConfig) (*ConsensusClient, error) {
	var nodeApi nodeapi.ConsensusApiLite
	nodeApi, err := history.NewHistoryConsensusApiLite(ctx, sourceConfig.History(), sourceConfig.Nodes, sourceConfig.FastStartup)
	if err != nil {
		return nil, fmt.Errorf("instantiating history consensus API lite: %w", err)
	}
	if sourceConfig.Cache != nil && sourceConfig.Cache.Consensus != nil {
		cacheConfig := sourceConfig.Cache.Consensus
		if cacheConfig.QueryOnCacheMiss {
			nodeApi, err = file.NewFileConsensusApiLite(cacheConfig.File, nodeApi)
		} else {
			nodeApi, err = file.NewFileConsensusApiLite(cacheConfig.File, nil)
		}
		if err != nil {
			return nil, fmt.Errorf("error instantiating cache-based consensusApi: %w", err)
		}
	}
	return &ConsensusClient{
		nodeApi: nodeApi,
		network: sourceConfig.SDKNetwork(),
	}, nil
}

// NewRuntimeClient creates a new RuntimeClient.
func NewRuntimeClient(ctx context.Context, sourceConfig *config.SourceConfig, runtime common.Runtime) (*RuntimeClient, error) {
	sdkPT := sourceConfig.SDKNetwork().ParaTimes.All[runtime.String()]
	var nodeApi nodeapi.RuntimeApiLite
	nodeApi, err := history.NewHistoryRuntimeApiLite(ctx, sourceConfig.History(), sdkPT, sourceConfig.Nodes, sourceConfig.FastStartup, runtime)
	if err != nil {
		return nil, fmt.Errorf("instantiating history runtime API lite: %w", err)
	}

	// todo: short circuit if using purely a file-based backend and avoid connecting
	// to the node at all. requires storing runtime info offline.
	if sourceConfig.Cache != nil && sourceConfig.Cache.Runtime != nil {
		cacheConfig := sourceConfig.Cache.Runtime
		if cacheConfig.QueryOnCacheMiss {
			nodeApi, err = file.NewFileRuntimeApiLite(cacheConfig.File, nodeApi)
		} else {
			nodeApi, err = file.NewFileRuntimeApiLite(cacheConfig.File, nil)
		}
		if err != nil {
			return nil, fmt.Errorf("error instantiating cache-based runtimeApi: %w", err)
		}
	}

	return &RuntimeClient{
		nodeApi: nodeApi,
		sdkPT:   sdkPT,
	}, nil
}
