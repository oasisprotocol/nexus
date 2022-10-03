package tests

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	v1 "github.com/oasisprotocol/oasis-indexer/api/v1"
)

// GetFrom completes an HTTP request and returns the unmarshalled response.
func GetFrom(path string, v interface{}) error {
	req, err := http.NewRequestWithContext(context.Background(), "GET", fmt.Sprintf("%s%s", baseEndpoint, path), nil)
	if err != nil {
		return err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	err = json.Unmarshal(body, v)
	if err != nil {
		return err
	}

	return nil
}

// After waits for the height to be indexed and then sends
// the indexed height on the returned channel.
func After(height int64) <-chan int64 {
	out := make(chan int64)
	go func() {
		var status v1.Status
		for {
			if err := GetFrom("/", &status); err != nil {
				out <- 0
				return
			}
			if status.LatestBlock >= height {
				out <- status.LatestBlock
				return
			}
		}
	}()
	return out
}
