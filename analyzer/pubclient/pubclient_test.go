package pubclient

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func wasteResp(resp *http.Response) error {
	_, err := io.Copy(io.Discard, resp.Body)
	if err != nil {
		return err
	}
	return nil
}

func TestMisc(t *testing.T) {
	var requested bool
	testServer := http.Server{
		Addr: "127.0.0.1:8001",
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Println("local server requested")
			requested = true
		}),
	}
	serverErr := make(chan error)
	go func() {
		serverErr <- testServer.ListenAndServe()
	}()

	// Default client should reach local server. This makes sure the test server is working.
	resp, err := http.Get("http://localhost:8001/test.json")
	require.NoError(t, err)
	require.NoError(t, wasteResp(resp))
	require.True(t, requested)
	requested = false

	// Hostname of test server
	resp, err = Client.Get("http://localhost:8001/test.json")
	require.Error(t, err)
	fmt.Printf("err %v\n", err)
	require.ErrorIs(t, err, NotPermittedError{})
	require.False(t, requested)

	// IP address of test server
	resp, err = Client.Get("http://127.0.0.1:8001/test.json")
	require.Error(t, err)
	fmt.Printf("err %v\n", err)
	require.ErrorIs(t, err, NotPermittedError{})
	require.False(t, requested)

	// Server that redirects to test server
	// Warning: external network dependency
	resp, err = Client.Get("https://httpbin.org/redirect-to?url=http%3A%2F%2F127.0.0.1%3A8001%2Ftest.json")
	require.Error(t, err)
	fmt.Printf("err %v\n", err)
	require.ErrorIs(t, err, NotPermittedError{})
	require.False(t, requested)

	// Domain that resolves to test server
	// Warning: external network dependency
	resp, err = Client.Get("http://127.0.0.1.nip.io:8001/test.json")
	require.Error(t, err)
	fmt.Printf("err %v\n", err)
	require.ErrorIs(t, err, NotPermittedError{})
	require.False(t, requested)

	// Well known port other than HTTP(S)
	resp, err = Client.Get("http://smtp.google.com:25/")
	require.Error(t, err)
	fmt.Printf("err %v\n", err)
	require.ErrorIs(t, err, NotPermittedError{})

	// Other requests ought to work.
	// Warning: external network dependency
	resp, err = Client.Get("https://www.example.com/")
	require.NoError(t, err)
	require.NoError(t, wasteResp(resp))

	err = testServer.Shutdown(context.Background())
	require.NoError(t, err)
	require.ErrorIs(t, <-serverErr, http.ErrServerClosed)
}
