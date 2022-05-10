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
func ReplyWithError(w http.ResponseWriter, err error) {
	var errResponse ErrorResponse
	var code int
	switch err {
	case ErrBadRequest:
		errResponse = ErrorResponse{
			Msg: err.Error(),
		}
		code = http.StatusBadRequest
	case ErrBadChainID:
		errResponse = ErrorResponse{
			Msg: err.Error(),
		}
		code = http.StatusNotFound
	case ErrStorageError:
		errResponse = ErrorResponse{
			Msg: err.Error(),
		}
		code = http.StatusInternalServerError
	default:
		errResponse = ErrorResponse{
			Msg: err.Error(),
		}
		code = http.StatusInternalServerError
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(errResponse)
}
