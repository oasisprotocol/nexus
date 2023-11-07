package connections

import (
	"context"
	"crypto/tls"
	"sync"

	cmnGrpc "github.com/oasisprotocol/oasis-core/go/common/grpc"
	sdkConfig "github.com/oasisprotocol/oasis-sdk/client-sdk/go/config"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/oasisprotocol/nexus/config"
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

func LazyGrpcConnect(nodeConfig config.NodeConfig) *LazyGrpcConn {
	return &LazyGrpcConn{
		inner:      nil, // The underlying connection will be initialized lazily.
		lock:       sync.Mutex{},
		nodeConfig: nodeConfig,
	}
}

type LazyGrpcConn struct {
	inner *grpc.ClientConn
	lock  sync.Mutex // For lazy initialization.

	nodeConfig config.NodeConfig // The node to connect to.
}

// ensureConn initializes `inner` if it hasn't been initialized yet.
// This function is thread-safe. If it returns nil, `inner` is guaranteed to be non-nil.
func (c *LazyGrpcConn) ensureConn() error {
	c.lock.Lock()
	defer c.lock.Unlock()

	if c.inner != nil {
		return nil
	}

	// Initialize `inner`.
	var err error
	c.inner, err = RawConnect(&c.nodeConfig)
	if err != nil {
		return err
	}

	return nil
}

func (c *LazyGrpcConn) Close() error {
	if c.inner == nil {
		return nil
	}
	return c.inner.Close()
}

func (c *LazyGrpcConn) Invoke(ctx context.Context, method string, args interface{}, reply interface{}, opts ...grpc.CallOption) error {
	if err := c.ensureConn(); err != nil {
		return err
	}

	return c.inner.Invoke(ctx, method, args, reply, opts...)
}
