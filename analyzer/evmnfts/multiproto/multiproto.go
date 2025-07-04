// Package multiproto implements a multi-protocol file fetcher.
package multiproto

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/url"
	"strings"

	"github.com/oasisprotocol/nexus/analyzer/evmnfts/ipfsclient"
	"github.com/oasisprotocol/nexus/analyzer/httpmisc"
	"github.com/oasisprotocol/nexus/analyzer/pubclient"
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
	case "data":
		// A few NFTs publish their metadata directly in data URIs that look like
		// `data:application/json;base64,eyJuYW1lIjogIkZh....`
		// In this case we return the base64 encoded data
		enc, found := strings.CutPrefix(parsed.Opaque, "application/json;base64,")
		if !found {
			return nil, fmt.Errorf("data uri did not start with expected prefix of 'application/json;base64,'")
		}
		raw, err1 := base64.StdEncoding.DecodeString(enc)
		if err1 != nil {
			return nil, fmt.Errorf("unable to b64 decode data: %w", err1)
		}
		return io.NopCloser(bytes.NewReader(raw)), nil
	default:
		return nil, fmt.Errorf("unsupported protocol %s", parsed.Scheme)
	}
}
