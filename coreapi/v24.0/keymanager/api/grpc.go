package api

import (
	"github.com/oasisprotocol/nexus/coreapi/v24.0/keymanager/churp"
	"github.com/oasisprotocol/nexus/coreapi/v24.0/keymanager/secrets"
)

// RegisterService registers a new keymanager backend service with the given gRPC server.
// removed func

// KeymanagerClient is a gRPC keymanager client.
type KeymanagerClient struct {
	secretsClient *secrets.Client
	churpClient   *churp.Client
}

// removed func

// removed func

// NewKeymanagerClient creates a new gRPC keymanager client service.
// removed func
