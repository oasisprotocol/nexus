package history

import (
	"crypto/tls"

	cmnGrpc "github.com/oasisprotocol/oasis-core/go/common/grpc"
	sdkConfig "github.com/oasisprotocol/oasis-sdk/client-sdk/go/config"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/oasisprotocol/oasis-indexer/config"
)

// RawConnect establishes gRPC connection with the target URL,
// omitting the chain context check.
// This is based on oasis-sdk `ConnectNoVerify()` function,
// but returns a raw gRPC connection instead of the oasis-sdk `Connection` wrapper.
func RawConnect(nodeConfig *config.NodeConfig) (*grpc.ClientConn, error) {
	var dialOpts []grpc.DialOption
	fakeSDKNet := &sdkConfig.Network{
		RPC: nodeConfig.RPC,
	}
	switch fakeSDKNet.IsLocalRPC() {
	case true:
		// No TLS needed for local nodes.
		dialOpts = append(dialOpts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	case false:
		// Configure TLS for non-local nodes.
		creds := credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12})
		dialOpts = append(dialOpts, grpc.WithTransportCredentials(creds))
	}

	return cmnGrpc.Dial(nodeConfig.RPC, dialOpts...)
}
