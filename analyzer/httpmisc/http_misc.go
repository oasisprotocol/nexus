// Package httpmisc contains options that are common to a few places that use HTTP.
package httpmisc

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

const ClientTimeout = 30 * time.Second

func GetWithContextWithClient(ctx context.Context, client *http.Client, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	return client.Do(req)
}

type ResourceError struct {
	// Note: .error is the implementation of .Error, .Unwrap etc. It is not
	// in the Unwrap chain. Use something like
	// `ResourceError{fmt.Errorf("...: %w", err)}` to set up an
	// instance with `err` in the Unwrap chain.
	error
}

func (err ResourceError) Is(target error) bool {
	if _, ok := target.(ResourceError); ok {
		return true
	}
	return false
}

func ResponseOK(resp *http.Response) error {
	if resp.StatusCode >= 500 || resp.StatusCode == 429 {
		if err := resp.Body.Close(); err != nil {
			return fmt.Errorf("HTTP closing body due to HTTP %d: %w", resp.StatusCode, err)
		}
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	if resp.StatusCode != 200 {
		if err := resp.Body.Close(); err != nil {
			return fmt.Errorf("HTTP closing body: %w", err)
		}
		return ResourceError{fmt.Errorf("HTTP %d", resp.StatusCode)}
	}
	return nil
}
