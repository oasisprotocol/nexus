package api

import (
	"google.golang.org/grpc"
)

// removed var block

// removed func

// removed func

// removed func

// removed func

// removed func

// removed func

// removed func

// removed func

// removed func

// removed func

// removed func

// removed func

// removed func

// removed func

// removed func

// removed func

// removed func

// removed func

// removed func

// removed func

// removed func

// removed func

// RegisterService registers a new client backend service with the given gRPC server.
// removed func

// RegisterLightService registers a new light client backend service with the given gRPC server.
// removed func

type consensusLightClient struct {
	conn *grpc.ClientConn
}

// Implements LightClientBackend.
// removed func

// Implements LightClientBackend.
// removed func

// Implements LightClientBackend.
// removed func

type stateReadSync struct {
	c *consensusLightClient
}

// Implements syncer.ReadSyncer.
// removed func

// Implements syncer.ReadSyncer.
// removed func

// Implements syncer.ReadSyncer.
// removed func

// Implements LightClientBackend.
// removed func

// Implements LightClientBackend.
// removed func

// Implements LightClientBackend.
// removed func

type consensusClient struct {
	consensusLightClient

	conn *grpc.ClientConn
}

// removed func

// removed func

// removed func

// removed func

// removed func

// removed func

// removed func

// removed func

// removed func

// removed func

// removed func

// removed func

// removed func

// removed func

// removed func

// removed func

// removed func

// removed func

// removed func

// removed func

// NewConsensusClient creates a new gRPC consensus client service.
// removed func

// NewConsensusLightClient creates a new gRPC consensus light client service.
// removed func
