// Package api defines API handlers for the Oasis Indexer API.
package api

import (
	"encoding/json"
	"net/http"
)

var (
	latestMetadata = APIMetadata{
		Major: 0,
		Minor: 1,
		Patch: 0,
	}
)

// APIMetadata is the Oasis Indexer API metadata.
type APIMetadata struct {
	Major uint16
	Minor uint16
	Patch uint16
}

// GetMetadata gets metadata for the Oasis Indexer API.
func GetMetadata(w http.ResponseWriter, r *http.Request) {
	var resp []byte
	resp, err := json.Marshal(latestMetadata)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("content-type", "application/json")
	w.Write(resp)
}
