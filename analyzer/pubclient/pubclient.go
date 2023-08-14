package pubclient

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"syscall"
	"time"

	coreCommon "github.com/oasisprotocol/oasis-core/go/common"
)

var permittedNetworks = map[string]bool{
	"tcp4": true,
	"tcp6": true,
}

type NotPermittedError struct {
	// Note: .error is the implementation of .Error, .Unwrap etc. It is not
	// in the Unwrap chain. Use something like
	// `NotPermittedError{fmt.Errorf("...: %w", err)}` to set up an
	// instance with `err` in the Unwrap chain.
	error
}

func (err NotPermittedError) Is(target error) bool {
	if _, ok := target.(NotPermittedError); ok {
		return true
	}
	return false
}

// client is an *http.Client that permits HTTP(S) connections to hosts that
// oasis-core considers "likely to be globally reachable" on the default
// HTTP(S) ports and unreserved ports.
var client = &http.Client{
	Transport: &http.Transport{
		// Copied from http.DefaultTransport.
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			// Copied from http.DefaultTransport.
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
			// https://www.agwa.name/blog/post/preventing_server_side_request_forgery_in_golang
			// Recommends using a net.Dialer Control to interpose on local connections.
			Control: func(network, address string, c syscall.RawConn) error {
				if !permittedNetworks[network] {
					return NotPermittedError{fmt.Errorf("network %s not permitted", network)}
				}
				host, portStr, err := net.SplitHostPort(address)
				if err != nil {
					return NotPermittedError{fmt.Errorf("net.SplitHostPort %s: %w", address, err)}
				}
				ip := net.ParseIP(host)
				if ip == nil {
					return NotPermittedError{fmt.Errorf("IP %s not valid", ip)}
				}
				if !coreCommon.IsProbablyGloballyReachable(ip) {
					return NotPermittedError{fmt.Errorf("IP %s not permitted", ip)}
				}
				port, err := strconv.ParseUint(portStr, 10, 16)
				if err != nil {
					return NotPermittedError{fmt.Errorf("strconv.ParseUint %s: %w", portStr, err)}
				}
				if port != 443 && port != 80 && port < 1024 {
					return NotPermittedError{fmt.Errorf("port %d not permitted", port)}
				}
				return nil
			},
		}).DialContext,
		// Copied from http.DefaultTransport.
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	},
}

func getWithContextWithClient(ctx context.Context, client *http.Client, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	return client.Do(req)
}

func GetWithContext(ctx context.Context, url string) (*http.Response, error) {
	return getWithContextWithClient(ctx, client, url)
}
