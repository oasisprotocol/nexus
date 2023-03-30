// Package oasis implements the source storage interface
// backed by oasis-node.
package oasis

import (
	"context"
	"crypto/tls"

	"github.com/oasisprotocol/oasis-core/go/common/crypto/signature"
	cmnGrpc "github.com/oasisprotocol/oasis-core/go/common/grpc"
	"github.com/oasisprotocol/oasis-indexer/storage/oasis/nodeapi"
	"github.com/oasisprotocol/oasis-indexer/storage/oasis/nodeapi/cobalt"
	"github.com/oasisprotocol/oasis-indexer/storage/oasis/nodeapi/damask"
	config "github.com/oasisprotocol/oasis-sdk/client-sdk/go/config"
	connection "github.com/oasisprotocol/oasis-sdk/client-sdk/go/connection"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	moduleName = "storage_oasis"
)

// ClientFactory supports connections to an oasis-node instance.
type ClientFactory struct {
	connection        *connection.Connection
	rawGrpcConnection *grpc.ClientConn
	network           *config.Network
}

// NewClientFactory creates a new oasis-node client factory.
// If `skipChainContextCheck` is true, it also checks that
// the RPC endpoint in `network` serves the chain context specified
// in `network`.
func NewClientFactory(ctx context.Context, network *config.Network, skipChainContextCheck bool) (*ClientFactory, error) {
	var conn connection.Connection
	var err error
	if skipChainContextCheck {
		conn, err = connection.ConnectNoVerify(ctx, network)
	} else {
		conn, err = connection.Connect(ctx, network)
	}
	if err != nil {
		return nil, err
	}
	rawConn, err := ConnectNoVerify(ctx, network)
	if err != nil {
		return nil, err
	}

	return &ClientFactory{
		connection:        &conn,
		rawGrpcConnection: rawConn,
		network:           network,
	}, nil
}

// Consensus creates a new ConsensusClient.
func (cf *ClientFactory) Consensus() (*ConsensusClient, error) {
	// TODO: Remove the hardcoded values in next block once indexer config supports multiple nodes.
	// The plan is to introduce a new implementation of ConsensusApiLite that
	// will forward each call to either CobaltConsensusApiLite or DamaskConsensusApiLite,
	// depending on the `height` parameter in the call.
	var nodeApi nodeapi.ConsensusApiLite
	if cf.network.ChainContext == "53852332637bacb61b91b6411ab4095168ba02a50be4c3f82448438826f23898" {
		// Cobalt mainnet.
		nodeApi = cobalt.NewCobaltConsensusApiLite(cf.rawGrpcConnection)
	} else {
		// Assume Damask.
		client := (*cf.connection).Consensus()
		nodeApi = damask.NewDamaskConsensusApiLite(client)
	}

	// Configure chain context for all signatures using chain domain separation.
	signature.SetChainContext(cf.network.ChainContext)

	c := &ConsensusClient{
		nodeApi: nodeApi,
		network: cf.network,
	}

	return c, nil
}

// Runtime creates a new RuntimeClient.
func (cf *ClientFactory) Runtime(runtimeID string) (*RuntimeClient, error) {
	ctx := context.Background()

	client := (*cf.connection).Runtime(&config.ParaTime{ID: runtimeID})
	info, err := client.GetInfo(ctx)
	if err != nil {
		return nil, err
	}

	nodeApi := nodeapi.NewUniversalRuntimeApiLite(info.ID, cf.rawGrpcConnection, &client)

	// Configure chain context for all signatures using chain domain separation.
	signature.SetChainContext(cf.network.ChainContext)

	return &RuntimeClient{
		nodeApi: nodeApi,
		info:    info,
	}, nil
}

// ConnectNoVerify establishes gRPC connection with the target URL,
// omitting the chain context check.
// This is a clone of the oasis-copy `ConnectNoVerify()` function,
// but returns a raw gRPC connection instead of the oasis-sdk `Connection` wrapper.
func ConnectNoVerify(ctx context.Context, net *config.Network) (*grpc.ClientConn, error) {
	var dialOpts []grpc.DialOption
	switch net.IsLocalRPC() {
	case true:
		// No TLS needed for local nodes.
		dialOpts = append(dialOpts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	case false:
		// Configure TLS for non-local nodes.
		creds := credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12})
		dialOpts = append(dialOpts, grpc.WithTransportCredentials(creds))
	}

	return cmnGrpc.Dial(net.RPC, dialOpts...)
}
