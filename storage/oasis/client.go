// Package oasis implements the source storage interface
// backed by oasis-node.
package oasis

import (
	"context"
	"fmt"

	"github.com/oasisprotocol/oasis-indexer/common"
	"github.com/oasisprotocol/oasis-indexer/config"
	"github.com/oasisprotocol/oasis-indexer/storage/oasis/connections"
	"github.com/oasisprotocol/oasis-indexer/storage/oasis/nodeapi"
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
	currentHistoryRecord := sourceConfig.History().CurrentRecord()
	currentNode := sourceConfig.Nodes[currentHistoryRecord.ArchiveName]
	rawConn, err := connections.RawConnect(currentNode)
	if err != nil {
		return nil, fmt.Errorf("indexer RawConnect: %w", err)
	}
	sdkConn, err := connections.SDKConnect(ctx, currentHistoryRecord.ChainContext, currentNode, sourceConfig.FastStartup)
	if err != nil {
		return nil, err
	}
	sdkPT := sourceConfig.SDKNetwork().ParaTimes.All[runtime.String()]
	sdkClient := sdkConn.Runtime(sdkPT)

	info, err := sdkClient.GetInfo(ctx)
	if err != nil {
		return nil, err
	}

	return &RuntimeClient{
		nodeApi: nodeapi.NewUniversalRuntimeApiLite(info.ID, rawConn, &sdkClient),
		info:    info,
	}, nil
}
