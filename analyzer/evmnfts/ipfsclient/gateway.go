package ipfsclient

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/oasisprotocol/nexus/analyzer/evmnfts/httpmisc"
)

// gatewayHTTPClient is an *http.Client to use for accessing gateways.
// We can't use the pubclient package because we use a local ipfs gateway.
var gatewayHTTPClient = &http.Client{
	Timeout: httpmisc.ClientTimeout,
}

// Gateway is a Client that fetches through an IPFS HTTP gateway.
type Gateway struct {
	base string
}

var _ Client = (*Gateway)(nil)

func NewGateway(base string) (*Gateway, error) {
	return &Gateway{
		base: base,
	}, nil
}

func (g *Gateway) Get(ctx context.Context, path string) (io.ReadCloser, error) {
	url := g.base + "/ipfs/" + path
	resp, err := httpmisc.GetWithContextWithClient(ctx, gatewayHTTPClient, url)
	if err != nil {
		return nil, fmt.Errorf("requesting from gateway: %w", err)
	}
	if err = httpmisc.ResponseOK(resp); err != nil {
		return nil, err
	}
	return resp.Body, nil
}
