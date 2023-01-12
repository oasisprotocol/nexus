package common

import (
	"errors"
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
