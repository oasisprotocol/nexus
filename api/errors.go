package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"reflect"

	apiTypes "github.com/oasisprotocol/oasis-indexer/api/v1/types"
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

func HttpCodeForError(err error) int {
	errType := reflect.ValueOf(err).Elem().Type()

	switch {
	case err == ErrBadChainID:
		return http.StatusNotFound
	case err == ErrStorageError:
		return http.StatusInternalServerError
	case err == ErrBadRequest:
		return http.StatusBadRequest
	case (errType == reflect.TypeOf(apiTypes.InvalidParamFormatError{}) ||
		errType == reflect.TypeOf(apiTypes.RequiredHeaderError{}) ||
		errType == reflect.TypeOf(apiTypes.RequiredParamError{}) ||
		errType == reflect.TypeOf(apiTypes.UnescapedCookieParamError{}) ||
		errType == reflect.TypeOf(apiTypes.UnmarshallingParamError{}) ||
		errType == reflect.TypeOf(apiTypes.TooManyValuesForParamError{})):
		return http.StatusBadRequest
	default:
		return http.StatusInternalServerError
	}
}

// A simple error handler that renders any error as human-readable JSON to
// the HTTP response stream `w`.
func HumanReadableJsonErrorHandler(w http.ResponseWriter, r *http.Request, err error) {
	w.Header().Set("content-type", "application/json; charset=utf-8")
	w.Header().Set("x-content-type-options", "nosniff")
	w.WriteHeader(HttpCodeForError(err))

	// Wrap the error into a trivial JSON object as specified in the OpenAPI spec.
	msg := err.Error()
	errStruct := apiTypes.HumanReadableError{Msg: msg}

	_ = json.NewEncoder(w).Encode(errStruct)
}
