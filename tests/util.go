package tests

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	v1 "github.com/oasislabs/oasis-indexer/api/v1"
)

// GetFrom completes an HTTP request and returns the unmarshalled response.
func GetFrom(path string, v interface{}) error {
	resp, err := http.Get(fmt.Sprintf("%s%s", baseEndpoint, path))
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
