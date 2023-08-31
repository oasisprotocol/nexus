package httpmisc

import (
	"context"
	"net/http"
	"time"
)

// httpmisc is a bunch of opinions that are common to a few places that use
// HTTP.

const ClientTimeout = 30 * time.Second

func GetWithContextWithClient(ctx context.Context, client *http.Client, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	return client.Do(req)
}
