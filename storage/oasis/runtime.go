package oasis

import (
	"fmt"

	config "github.com/oasisprotocol/oasis-sdk/client-sdk/go/config"
	connection "github.com/oasisprotocol/oasis-sdk/client-sdk/go/connection"
)

// RuntimeClient is a client to a ParaTime.
type RuntimeClient struct {
	client  connection.RuntimeClient
	network *config.Network
}

// Name returns the name of the client, for the RuntimeSourceStorage interface.
func (rc *RuntimeClient) Name() string {
	// TODO: Parse the runtime name from config.DefaultNetworks.
	return fmt.Sprintf("%s_runtime")
}
