// Package oasis implements the source storage interface
// backed by oasis-node.
package oasis

import (
	"context"

	"github.com/oasisprotocol/oasis-core/go/common/crypto/signature"
	config "github.com/oasisprotocol/oasis-sdk/client-sdk/go/config"
	connection "github.com/oasisprotocol/oasis-sdk/client-sdk/go/connection"
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
func NewClientFactory(ctx context.Context, network *config.Network) (*ClientFactory, error) {
	connection, err := connection.Connect(ctx, network)
	if err != nil {
		return nil, err
	}

	chainContext, err := connection.Consensus().GetChainContext(ctx)
	if err != nil {
		return nil, err
	}

	// Configure chain context for all signatures using chain domain separation.
	signature.SetChainContext(chainContext)

	return &ClientFactory{
		connection: &connection,
		network:    network,
	}, nil
}

// Consensus creates a new ConsensusClient.
func (cf *ClientFactory) Consensus() (*ConsensusClient, error) {
	connection := *cf.connection
	client := connection.Consensus()
	return &ConsensusClient{
		client:  client,
		network: cf.network,
	}, nil
}

// Runtime creates a new RuntimeClient.
func (cf *ClientFactory) Runtime(runtimeID string) (*RuntimeClient, error) {
	connection := *cf.connection
	client := connection.Runtime(&config.ParaTime{
		ID: runtimeID,
	})
	return &RuntimeClient{
		client:  client,
		network: cf.network,
	}, nil
}
