package multiproto

import (
	"context"
	"fmt"
	"io"
	"net/url"

	"github.com/oasisprotocol/nexus/analyzer/evmnfts/httpmisc"
	"github.com/oasisprotocol/nexus/analyzer/evmnfts/ipfsclient"
	"github.com/oasisprotocol/nexus/analyzer/evmnfts/pubclient"
)

// multiproto gets a file from a URL by looking at the URL's scheme and
// requesting it from suitable client. Cross-scheme redirects thus don't work,
// as we only switch once on protocol out here.

func Get(ctx context.Context, ipfsClient ipfsclient.Client, whyDidTheyNameTheirLibraryURLNowICantUseThatName string) (io.ReadCloser, error) {
	parsed, err := url.Parse(whyDidTheyNameTheirLibraryURLNowICantUseThatName)
	if err != nil {
		return nil, fmt.Errorf("parse URL: %w", err)
	}
	if parsed.Scheme == "" {
		return nil, fmt.Errorf("scheme not set")
	}
	switch parsed.Scheme {
	case "http", "https":
		resp, err1 := pubclient.GetWithContext(ctx, whyDidTheyNameTheirLibraryURLNowICantUseThatName)
		if err1 != nil {
			return nil, fmt.Errorf("HTTP get: %w", err1)
		}
		if err1 = httpmisc.ResponseOK(resp); err1 != nil {
			return nil, err1
		}
		return resp.Body, nil
	case "ipfs":
		rc, err1 := ipfsClient.Get(ctx, parsed.Host+"/"+parsed.Path)
		if err1 != nil {
			return nil, fmt.Errorf("IPFS get: %w", err1)
		}
		return rc, nil
	default:
		return nil, fmt.Errorf("unsupported protocol %s", parsed.Scheme)
	}
}
