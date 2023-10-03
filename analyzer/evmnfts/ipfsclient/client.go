package ipfsclient

import (
	"context"
	"io"
)

// ipfsclient is a super-simple interface for getting a file from ipfs. It
// comes with an implementation that uses an IPFS HTTP gateway.

// Client is a simplified IPFS interface.
type Client interface {
	Get(ctx context.Context, path string) (io.ReadCloser, error)
}
