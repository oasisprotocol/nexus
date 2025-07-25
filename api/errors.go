// Package api defines the server API types.
package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"reflect"

	apiTypes "github.com/oasisprotocol/nexus/api/v1/types"
	"github.com/oasisprotocol/nexus/common"
	"github.com/oasisprotocol/nexus/log"
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
	// ErrNotFound is returned when handling a request for an item that
	// does not exist in the DB.
	ErrNotFound = errors.New("item not found")
	// ErrUnavailable is returned when the endpoint is disabled.
	ErrUnavailable = errors.New("unavailable")
)

type ErrStorageError struct{ Err error }

func (e ErrStorageError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("storage error: %s", e.Err.Error())
	}
	// ErrStorageError shouldn't be constructed with a nil Err, but format it just in case.
	return "storage error: internal bug, incorrectly instantiated error object with nil"
}

func HttpCodeForError(err error) int {
	errVal := reflect.ValueOf(err)
	// dereference an interface or pointer
	if errVal.Kind() == reflect.Interface || errVal.Kind() == reflect.Ptr {
		errVal = errVal.Elem()
	}
	errType := errVal.Type()

	switch {
	case errors.Is(err, ErrBadChainID):
		return http.StatusNotFound
	case errors.Is(err, ErrBadRequest):
		return http.StatusBadRequest
	case errors.Is(err, ErrNotFound):
		return http.StatusNotFound
	case errors.Is(err, ErrUnavailable):
		return http.StatusServiceUnavailable
	case errType == reflect.TypeOf(ErrStorageError{}):
		return http.StatusInternalServerError
	case errType == reflect.TypeOf(apiTypes.InvalidParamFormatError{}) ||
		errType == reflect.TypeOf(apiTypes.RequiredHeaderError{}) ||
		errType == reflect.TypeOf(apiTypes.RequiredParamError{}) ||
		errType == reflect.TypeOf(apiTypes.UnescapedCookieParamError{}) ||
		errType == reflect.TypeOf(apiTypes.UnmarshalingParamError{}) ||
		errType == reflect.TypeOf(apiTypes.TooManyValuesForParamError{}) ||
		errors.Is(err, apiTypes.ErrMalformedInputAddress):
		return http.StatusBadRequest
	default:
		return http.StatusInternalServerError
	}
}

// A simple error handler that logs and renders any error as human-readable JSON to
// the HTTP response stream `w`.
func HumanReadableJsonErrorHandler(logger log.Logger) func(http.ResponseWriter, *http.Request, error) {
	return func(w http.ResponseWriter, r *http.Request, err error) {
		logger.Debug("request failed, handling human readable error",
			"err", err,
			"request_id", r.Context().Value(common.RequestIDContextKey),
			"ctx_err", r.Context().Err(),
		)

		// If request context is closed, don't bother writing a response.
		if r.Context().Err() != nil {
			return
		}

		w.Header().Set("content-type", "application/json; charset=utf-8")
		w.Header().Set("x-content-type-options", "nosniff")
		w.WriteHeader(HttpCodeForError(err))

		// Wrap the error into a trivial JSON object as specified in the OpenAPI spec.
		msg := err.Error()
		errStruct := apiTypes.HumanReadableError{Msg: msg}

		_ = json.NewEncoder(w).Encode(errStruct)
	}
}
