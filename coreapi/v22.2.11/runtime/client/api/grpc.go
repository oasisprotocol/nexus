package api

import (
	"google.golang.org/grpc"
)

// removed var block

// removed func

// removed func

// removed func

// removed func

// wrappedErrNotFound is a wrapped ErrNotFound error so that it corresponds
// to the gRPC NotFound error code. It is required because Rust's gRPC bindings
// do not support fetching error details.
type wrappedErrNotFound struct {
	err error
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

// RegisterService registers a new runtime client service with the given gRPC server.
// removed func

type runtimeClient struct {
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

// NewRuntimeClient creates a new gRPC runtime client service.
// removed func
