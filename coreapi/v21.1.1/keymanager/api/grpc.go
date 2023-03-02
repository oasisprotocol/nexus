package api

import (
	"google.golang.org/grpc"
)

// removed var block

type enclaveRPCEndpoint struct{}

// Implements enclaverpc.Endpoint.
// removed func

// removed func

// removed func

// RegisterService registers a new keymanager backend service with the given gRPC server.
// removed func

// KeymanagerClient is a gRPC keymanager client.
type KeymanagerClient struct {
	conn *grpc.ClientConn
}

// removed func

// removed func

// NewKeymanagerClient creates a new gRPC keymanager client service.
// removed func

// removed func
