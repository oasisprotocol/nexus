package oasis

import (
	sdkConfig "github.com/oasisprotocol/oasis-sdk/client-sdk/go/config"

	"github.com/oasisprotocol/nexus/storage/oasis/nodeapi"
)

// ConsensusClient is a client to the consensus methods/data of oasis node. It
// differs from the nodeapi.ConsensusApiLite in that:
//   - Its methods may collect data using multiple RPCs each.
//   - The return types make no effort to closely resemble oasis-core types
//     in structure. Instead, they are structured in a way that is most convenient
//     for the analyzer.
//
// TODO: The benefits of this abstraction are miniscule, and introduce considerable
// boilerplate. Consider removing ConsensusClient, and using nodeapi.ConsensusApiLite instead.
type ConsensusClient struct {
	NodeApi nodeapi.ConsensusApiLite
	Network *sdkConfig.Network
}

func (cc *ConsensusClient) Close() error {
	return cc.NodeApi.Close()
}
