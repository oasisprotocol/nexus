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
func SDKConnect(ctx context.Context, chainContext string, nodeConfig *config.NodeConfig, fastStartup bool) (connection.Connection, error) {
	fakeNet := &sdkConfig.Network{
		RPC:          nodeConfig.RPC,
		ChainContext: chainContext,
	}
	var sdkConn connection.Connection
	if fastStartup {
		var err error
		sdkConn, err = connection.ConnectNoVerify(ctx, fakeNet)
		if err != nil {
			return nil, fmt.Errorf("SDK ConnectNoVerify: %w", err)
		}
	} else {
		var err error
		sdkConn, err = connection.Connect(ctx, fakeNet)
		if err != nil {
			return nil, fmt.Errorf("SDK Connect: %w", err)
		}
	}
	return sdkConn, nil
}
