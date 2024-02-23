package httpproxy

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/oasisprotocol/nexus/config"
)

// Utility func: Performs HTTP GET, returns most salient parts of the response.
func simpleGet(url string) (statusCode int, myheader string, body []byte, err error) {
	var resp *http.Response
	resp, err = (&http.Client{Timeout: 1 * time.Second}).Get(url) //nolint:noctx
	if err != nil {
		return
	}
	defer resp.Body.Close()
	statusCode = resp.StatusCode
	myheader = resp.Header.Get("X-myheader") // a header used in the tests
	body, err = io.ReadAll(resp.Body)
	return
}

func TestProxyBasic(t *testing.T) {
	// Create a mock 3rd-party server. It echoes the requested URL and prints the current time.
	timeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Add("X-myheader", "foo")
		w.WriteHeader(203) // 203 Non-Authoritative Information; similar to 200 but unique enough to test the proxy forwards it.
		response := []byte(fmt.Sprintf("You requested URL %s at %d", req.URL, time.Now().UnixMilli()))
		w.Write(response) //nolint:errcheck
	}))
	defer timeServer.Close()

	// Create the proxy under test.
	os.RemoveAll("/tmp/test_proxy")
	proxy, err := NewHttpServer(
		config.CacheConfig{CacheDir: "/tmp/test_proxy"},
		config.HttpCachingProxyConfig{HostAddr: "localhost:3333", TargetURL: timeServer.URL},
	)
	require.NoError(t, err)
	go proxy.ListenAndServe()                  //nolint:errcheck
	time.Sleep(200 * time.Millisecond)         // wait for the proxy to start
	defer proxy.Shutdown(context.Background()) //nolint:errcheck

	// Make a sample request to the proxy. Verify it forwards all data from the 3rd-party server.
	statusCode, myHeader, body, err := simpleGet("http://localhost:3333/foo/bar")
	require.NoError(t, err)
	require.Equal(t, 203, statusCode)
	require.Equal(t, "foo", myHeader)
	require.Contains(t, string(body), "You requested URL /foo/bar at")

	// Make the request again. Verify that the response is served from the cache, i.e. identical to the first one.
	statusCode, myHeader, newBody, err := simpleGet("http://localhost:3333/foo/bar")
	require.NoError(t, err)
	require.Equal(t, 203, statusCode)
	require.Equal(t, "foo", myHeader)
	require.Equal(t, string(newBody), string(body))

	// Make a different request. Verify that the response is different from the first one.
	statusCode, myHeader, newBody, err = simpleGet("http://localhost:3333/x")
	require.NoError(t, err)
	require.Equal(t, 203, statusCode)
	require.Equal(t, "foo", myHeader)
	require.NotEqual(t, string(newBody), string(body))

	// Make a request to a malfunctioning 3rd-party server. Verify the error propagates.
	timeServer.Close()
	time.Sleep(200 * time.Millisecond) // wait for the server to shut down
	statusCode, _, _, err = simpleGet("http://localhost:3333/time_server_is_dead")
	require.NoError(t, err)
	require.Equal(t, 500, statusCode)
}
