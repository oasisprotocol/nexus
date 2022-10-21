package common

import (
	"encoding/json"
	"errors"
	"net/http"
)

var (
	// ErrBadRequest is returned when the provided HTTP request
	// is malformed.
	ErrBadRequest = errors.New("invalid request parameters")
	// ErrBadChainID is returned when a malformed or missing chain ID
	// is provided.
	ErrBadChainID = errors.New("unable to resolve chain ID")
	// ErrBadRuntime is returned when a malformed or missing runtime name
	// is provided.
	ErrBadRuntime = errors.New("unable to resolve runtime")
	// ErrStorageError is returned when the underlying storage suffers
	// from an internal error.
	ErrStorageError = errors.New("internal storage error")
)

// ErrorResponse is a JSON error.
type ErrorResponse struct {
	Msg string `json:"msg"`
}

// ReplyWithError replies to an HTTP request with an error
// as JSON.
func ReplyWithError(w http.ResponseWriter, err error) error {
	var response ErrorResponse
	var code int
	switch err {
	case ErrBadRequest:
		response = ErrorResponse{err.Error()}
		code = http.StatusBadRequest
	case ErrBadChainID:
		response = ErrorResponse{err.Error()}
		code = http.StatusNotFound
	case ErrStorageError:
		response = ErrorResponse{err.Error()}
		code = http.StatusInternalServerError
	default:
		response = ErrorResponse{err.Error()}
		code = http.StatusInternalServerError
	}

	w.Header().Set("content-type", "application/json; charset=utf-8")
	w.Header().Set("x-content-type-options", "nosniff")
	w.WriteHeader(code)

	return json.NewEncoder(w).Encode(response)
}
