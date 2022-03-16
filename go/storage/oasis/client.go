// Package oasis implements the source storage interface
// backed by oasis-node.
package oasis

const (
	clientName = "oasis-node"
)

type OasisNodeClient struct {
}

// NewOasisNodeClient creates a new oasis-node client.
func NewOasisNodeClient() (*OasisNodeClient, error) {
	return &OasisNodeClient{}, nil
}

// Name returns the name of the oasis-node client.
func (c *OasisNodeClient) Name() string {
	return clientName
}
