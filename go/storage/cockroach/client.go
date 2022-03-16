// Package cockroach implements the target storage interface
// backed by CockroachDB.
package cockroach

const (
	clientName = "cockroach"
)

type CockroachClient struct {
}

// NewCockroachClient creates a new CockroachDB client.
func NewCockroachClient() (*CockroachClient, error) {
	return &CockroachClient{}, nil
}

// Name returns the name of the CockroachDB client.
func (c *CockroachClient) Name() string {
	return clientName
}
