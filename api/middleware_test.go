//nolint:noctx,bodyclose
package api

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestTimeoutMiddleware(t *testing.T) {
	withMiddleware := false
	baseHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Wait some time, so that the request timeout (10ms) should be reached.
		<-time.After(2 * time.Second)

		// Check if the request context is canceled.
		if withMiddleware {
			// If timeout middleware is used, the request context should be canceled.
			require.True(t, r.Context().Err() != nil, "request context should be canceled")
		} else {
			// The default behavior is that the request context is not canceled.
			require.False(t, r.Context().Err() != nil, "request context was not canceled")
		}
	})
	backend := httptest.NewUnstartedServer(baseHandler)
	backend.Config.WriteTimeout = 10 * time.Millisecond
	backend.Start()
	defer backend.Close()

	// Test server without timeout middleware.
	req, err := http.NewRequest("GET", backend.URL, nil)
	require.NoError(t, err)
	_, err = http.DefaultClient.Do(req)
	require.ErrorIs(t, err, io.EOF, "client received EOF")
	backend.Close()

	// Test server with timeout middleware.
	withMiddleware = true
	backendWithTimeout := httptest.NewUnstartedServer(ContextTimeoutMiddleware(10 * time.Millisecond)(baseHandler))
	backendWithTimeout.Config.WriteTimeout = 10 * time.Millisecond
	backendWithTimeout.Start()
	defer backendWithTimeout.Close()

	req, err = http.NewRequest("GET", backendWithTimeout.URL, nil)
	require.NoError(t, err)
	_, err = http.DefaultClient.Do(req)
	require.ErrorIs(t, err, io.EOF, "client received EOF")
}
