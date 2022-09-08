package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"oasis-explorer-backend/common"
)

func getOkRead(url string) ([]byte, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("http get %s: %w", url, err)
	}
	defer common.CloseOrLog(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("%s status %d not ok", url, resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("%s read body: %w", url, err)
	}
	return body, nil
}

func getOkReadJson(url string, v interface{}) error {
	body, err := getOkRead(url)
	if err != nil {
		return err
	}
	if err = json.Unmarshal(body, v); err != nil {
		return fmt.Errorf("json unmarshal response from %s: %w", url, err)
	}
	return nil
}

func respondCacheableJson(w http.ResponseWriter, result interface{}, maxAge int) error {
	j, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("json marshal: %w", err)
	}
	w.Header().Set("Cache-Control", fmt.Sprintf("max-age=%d", maxAge))
	common.WriteOrLog(w, j)
	return nil
}

func fallible(f func(w http.ResponseWriter, r *http.Request) error) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := f(w, r); err != nil {
			http.Error(w, err.Error(), 500)
		}
	}
}
