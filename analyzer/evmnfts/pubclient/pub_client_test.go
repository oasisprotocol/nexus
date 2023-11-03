//nolint:bodyclose  // This is a test, we can be sloppy and not close HTTP request bodies.
package pubclient

import (
	"context"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/oasisprotocol/nexus/analyzer/evmnfts/httpmisc"
)

func wasteResp(resp *http.Response) error {
	_, err := io.Copy(io.Discard, resp.Body)
	if err != nil {
		return err
	}
	return nil
}

func requireErrorAndWaste(t *testing.T, resp *http.Response, err error) {
	if err == nil {
		require.NoError(t, wasteResp(resp))
	}
	require.Error(t, err)
}

func requireNoErrorAndWaste(t *testing.T, resp *http.Response, err error) {
	if err == nil {
		require.NoError(t, wasteResp(resp))
	}
	require.NoError(t, err)
}

func TestMisc(t *testing.T) {
	var requested bool
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requested = true
	})
	testServer := http.Server{
		Addr:              "127.0.0.1:8001",
		Handler:           handler,
		ReadHeaderTimeout: 30 * time.Second,
	}
	serverErr := make(chan error)
	go func() {
		serverErr <- testServer.ListenAndServe()
	}()
	testServer6 := http.Server{
		Addr:              "[::1]:8001",
		Handler:           handler,
		ReadHeaderTimeout: 30 * time.Second,
	}
	serverErr6 := make(chan error)
	go func() {
		serverErr6 <- testServer6.ListenAndServe()
	}()
	ctx := context.Background()

	// Default client should reach local server. This makes sure the test server is working.
	resp, err := httpmisc.GetWithContextWithClient(ctx, http.DefaultClient, "http://localhost:8001/test.json")
	requireNoErrorAndWaste(t, resp, err)
	require.True(t, requested)
	requested = false
	resp, err = httpmisc.GetWithContextWithClient(ctx, http.DefaultClient, "http://[::1]:8001/test.json")
	requireNoErrorAndWaste(t, resp, err)
	require.True(t, requested)
	requested = false

	// Hostname of test server
	resp, err = GetWithContext(ctx, "http://localhost:8001/test.json")
	requireErrorAndWaste(t, resp, err)
	require.ErrorIs(t, err, NotPermittedError{})
	require.False(t, requested)

	// IP address of test server
	resp, err = GetWithContext(ctx, "http://127.0.0.1:8001/test.json")
	requireErrorAndWaste(t, resp, err)
	require.ErrorIs(t, err, NotPermittedError{})
	require.False(t, requested)
	resp, err = GetWithContext(ctx, "http://[::1]:8001/test.json")
	requireErrorAndWaste(t, resp, err)
	require.ErrorIs(t, err, NotPermittedError{})
	require.False(t, requested)

	// Server that redirects to test server
	// Warning: external network dependency
	resp, err = GetWithContext(ctx, "https://httpbin.org/redirect-to?url=http%3A%2F%2F127.0.0.1%3A8001%2Ftest.json")
	requireErrorAndWaste(t, resp, err)
	require.ErrorIs(t, err, NotPermittedError{})
	require.False(t, requested)
	resp, err = GetWithContext(ctx, "https://httpbin.org/redirect-to?url=http%3A%2F%2F%5B%3A%3A1%5D%3A8001%2Ftest.json")
	requireErrorAndWaste(t, resp, err)
	require.ErrorIs(t, err, NotPermittedError{})
	require.False(t, requested)

	// Domain that resolves to test server
	// Warning: external network dependency
	resp, err = GetWithContext(ctx, "http://127.0.0.1.nip.io:8001/test.json")
	requireErrorAndWaste(t, resp, err)
	require.ErrorIs(t, err, NotPermittedError{})
	require.False(t, requested)
	resp, err = GetWithContext(ctx, "http://0--1.sslip.io:8001/test.json")
	requireErrorAndWaste(t, resp, err)
	require.ErrorIs(t, err, NotPermittedError{})
	require.False(t, requested)

	// Well known port other than HTTP(S)
	resp, err = GetWithContext(ctx, "http://smtp.google.com:25/")
	requireErrorAndWaste(t, resp, err)
	require.ErrorIs(t, err, NotPermittedError{})

	// Other requests ought to work.
	// Warning: external network dependency
	resp, err = GetWithContext(ctx, "https://www.example.com/")
	requireNoErrorAndWaste(t, resp, err)

	err = testServer.Shutdown(ctx)
	require.NoError(t, err)
	require.ErrorIs(t, <-serverErr, http.ErrServerClosed)
	err = testServer6.Shutdown(ctx)
	require.NoError(t, err)
	require.ErrorIs(t, <-serverErr6, http.ErrServerClosed)
}
