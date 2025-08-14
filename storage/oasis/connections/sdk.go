package connections

import (
	"context"
	"fmt"

	sdkConfig "github.com/oasisprotocol/oasis-sdk/client-sdk/go/config"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/connection"

	"github.com/oasisprotocol/nexus/config"
)

// SDKConnect creates an Oasis SDK Connection to a node running the current
// version of oasis-node.
func SDKConnect(ctx context.Context, chainContext string, nodeConfig *config.NodeConfig) (connection.Connection, error) {
	fakeNet := &sdkConfig.Network{
		RPC:          nodeConfig.RPC,
		ChainContext: chainContext,
	}
	sdkConn, err := connection.ConnectNoVerify(ctx, fakeNet)
	if err != nil {
		return nil, fmt.Errorf("SDK ConnectNoVerify: %w", err)
	}
	return sdkConn, nil
}
