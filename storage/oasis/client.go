// Package oasis implements the source storage interface
// backed by oasis-node.
package oasis

import (
	"context"
	"crypto/tls"

	"github.com/oasisprotocol/oasis-core/go/common/crypto/signature"
	cmnGrpc "github.com/oasisprotocol/oasis-core/go/common/grpc"
	config "github.com/oasisprotocol/oasis-sdk/client-sdk/go/config"
	connection "github.com/oasisprotocol/oasis-sdk/client-sdk/go/connection"
	runtimeSignature "github.com/oasisprotocol/oasis-sdk/client-sdk/go/crypto/signature"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

const (
	moduleName = "storage_oasis"
)

// ClientFactory supports connections to an oasis-node instance.
type ClientFactory struct {
	connection *connection.Connection
	network    *config.Network
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

	return &ClientFactory{
		connection: &conn,
		network:    network,
	}, nil
}

// Consensus creates a new ConsensusClient.
func (cf *ClientFactory) Consensus() (*ConsensusClient, error) {
	creds := credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12})
	dialOpts := []grpc.DialOption{grpc.WithTransportCredentials(creds)}
	grpcConn, err := cmnGrpc.Dial(cf.network.RPC, dialOpts...)
	if err != nil {
		return nil, err
	}

	connection := *cf.connection
	client := connection.Consensus()

	// Configure chain context for all signatures using chain domain separation.
	signature.SetChainContext(cf.network.ChainContext)

	c := &ConsensusClient{
		grpcConn: *grpcConn,
		client:   client,
		network:  cf.network,
	}

	return c, nil
}

// Runtime creates a new RuntimeClient.
func (cf *ClientFactory) Runtime(runtimeID string) (*RuntimeClient, error) {
	ctx := context.Background()

	connection := *cf.connection
	client := connection.Runtime(&config.ParaTime{
		ID: runtimeID,
	})

	// Configure chain context for all signatures using chain domain separation.
	signature.SetChainContext(cf.network.ChainContext)

	info, err := client.GetInfo(ctx)
	if err != nil {
		return nil, err
	}

	rtCtx := runtimeSignature.DeriveChainContext(info.ID, cf.network.ChainContext)
	return &RuntimeClient{
		client:  client,
		network: cf.network,
		info:    info,
		rtCtx:   rtCtx,
	}, nil
}
