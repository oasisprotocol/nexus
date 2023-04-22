// Package oasis implements the source storage interface
// backed by oasis-node.
package oasis

import (
	"context"
	"fmt"

	"github.com/oasisprotocol/oasis-indexer/common"
	"github.com/oasisprotocol/oasis-indexer/config"
	"github.com/oasisprotocol/oasis-indexer/storage/oasis/nodeapi/history"
)

const (
	moduleName = "storage_oasis"
)

// NewConsensusClient creates a new ConsensusClient.
func NewConsensusClient(ctx context.Context, sourceConfig *config.SourceConfig) (*ConsensusClient, error) {
	nodeApi, err := history.NewHistoryConsensusApiLite(ctx, sourceConfig.History(), sourceConfig.Nodes, sourceConfig.FastStartup)
	if err != nil {
		return nil, fmt.Errorf("instantiating history consensus API lite: %w", err)
	}
	return &ConsensusClient{
		nodeApi: nodeApi,
		network: sourceConfig.SDKNetwork(),
	}, nil
}

// NewRuntimeClient creates a new RuntimeClient.
func NewRuntimeClient(ctx context.Context, sourceConfig *config.SourceConfig, runtime common.Runtime) (*RuntimeClient, error) {
	sdkPT := sourceConfig.SDKNetwork().ParaTimes.All[runtime.String()]
	nodeApi, err := history.NewHistoryRuntimeApiLite(ctx, sourceConfig.History(), sdkPT, sourceConfig.Nodes, sourceConfig.FastStartup, runtime)
	if err != nil {
		return nil, fmt.Errorf("instantiating history runtime API lite: %w", err)
	}

	return &RuntimeClient{
		nodeApi: nodeApi,
		sdkPT:   sdkPT,
	}, nil
}
