package v1

import (
	"context"
	"net/http"
	"reflect"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/iancoleman/strcase"
	apiTypes "github.com/oasisprotocol/oasis-indexer/api/v1/types"
	"github.com/oasisprotocol/oasis-indexer/common"
)

var (
	defaultOffset            = uint64(0)
	defaultLimit             = uint64(100)
	defaultBucketSizeSeconds = uint32(3600)
	maxLimit                 = uint64(1000)
)

// MetricsMiddleware is a middleware that measures the start and end of each request,
// as well as other useful request information.
func (h *Handler) MetricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := uuid.New()

		h.logger.Info("starting request",
			"endpoint", r.URL.Path,
			"request_id", requestID,
		)

		t := time.Now()
		timer := h.Metrics.RequestTimer(r.URL.Path)
		defer func() {
			h.logger.Info("ending request",
				"endpoint", r.URL.Path,
				"request_id", requestID,
				"time", time.Since(t),
			)
			timer.ObserveDuration()
		}()

		next.ServeHTTP(w, r.WithContext(
			context.WithValue(r.Context(), common.RequestIDContextKey, requestID),
		))
	})
}

// ChainMiddleware is a middleware that adds chain-specific information
// to the request context.
func (h *Handler) ChainMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		chainID := strcase.ToSnake(h.Client.chainID)

		// TODO: Set chainID based on provided height params.

		next.ServeHTTP(w, r.WithContext(
			context.WithValue(r.Context(), common.ChainIDContextKey, chainID),
		))
	})
}

// Sets a value for `Limit` and `Offset` fields of the given struct, if present and nil-valued.
// oapi-codegen ignores the specified default value in the openapi spec:
//
//	https://github.com/deepmap/oapi-codegen/issues?q=is%3Aissue+is%3Aopen+sort%3Aupdated-desc++default+in%3Atitle
//
// Luckily our defaults are pretty simple (just limit and offset) so we can hardcode them here.
func fixDefaultsAndLimits(p any) {
	// Check that p is a pointer to a struct.
	if p == nil || reflect.TypeOf(p).Kind() != reflect.Ptr || reflect.TypeOf(p).Elem().Kind() != reflect.Struct {
		panic("fixDefaults: p is not a pointer to a struct")
	}

	// Iterate through the struct fields. If the field name equals "Limit" or "Offset" and the value is nil,
	// set it to the default value.
	v := reflect.ValueOf(p).Elem()
	for i := 0; i < v.NumField(); i++ {
		f := v.Field(i)
		if f.Kind() == reflect.Ptr && f.IsNil() {
			switch v.Type().Field(i).Name {
			case "Limit":
				f.Set(reflect.ValueOf(&defaultLimit))
			case "Offset":
				f.Set(reflect.ValueOf(&defaultOffset))
			}
		}
	}

	// Iterate through the struct fields. If the field name equals "Limit" and it's of the right type, clamp it.
	for i := 0; i < v.NumField(); i++ {
		f := v.Field(i)
		if f.Kind() == reflect.Ptr && !f.IsNil() {
			switch v.Type().Field(i).Name { //nolint:gocritic // allow single-case switch for future expansions
			case "Limit":
				if v.Type().Field(i).Type == reflect.TypeOf(&maxLimit) && *f.Interface().(*uint64) > maxLimit {
					*f.Interface().(*uint64) = maxLimit
				}
			}
		}
	}
}

// FixDefaultsAndLimits modifies pagination parameters of the request in-place:
// If they're missing, it assigns them default values, and if they exist, it
// clamps them within the allowed limits.
// Both of these should be done by oapi-codegen, but it doesn't do it yet.
//
// _operationID is unused, but is required to match the StrictHandlerFunc signature.
// It takes values like "GetConsensusTransactions".
func FixDefaultsAndLimitsMiddleware(next apiTypes.StrictHandlerFunc, _operationID string) apiTypes.StrictHandlerFunc {
	return func(ctx context.Context, w http.ResponseWriter, r *http.Request, args interface{}) (interface{}, error) {
		// Create a new struct of the same type as `args` and copy the values from `args` into it.
		// This new struct will be "addressable" (= modifiable), unlike `args`.
		argsV := reflect.ValueOf(args)
		v := reflect.New(argsV.Type())
		v.Elem().Set(argsV)

		// Use reflection to check if `args` has a Params field.
		// If it does, we'll use it to set default values and limits.
		if v.Kind() == reflect.Ptr || v.Kind() == reflect.Interface {
			v = v.Elem()
		}
		if v.Kind() == reflect.Struct {
			f := v.FieldByName("Params")
			if f.IsValid() {
				fixDefaultsAndLimits(f.Addr().Interface())
			}
		}

		// Call the next middleware in the chain with fixed `args`.
		return next(ctx, w, r, v.Interface())
	}
}

func (h *Handler) RuntimeMiddleware(runtime string) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r.WithContext(
				context.WithValue(r.Context(), common.RuntimeContextKey, runtime),
			))
		})
	}
}

func (h *Handler) RuntimeFromURLMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/v1")

		// The first part of the path (after the version) determines the runtime.
		// Recognize only whitelisted runtimes.
		runtime := ""
		switch { //nolint:gocritic // allow single-case switch for future expansions
		case strings.HasPrefix(path, "/emerald/"):
			runtime = "emerald"
		}

		if runtime != "" {
			next.ServeHTTP(w, r.WithContext(
				context.WithValue(r.Context(), common.RuntimeContextKey, runtime),
			))
		} else {
			next.ServeHTTP(w, r)
		}
	})
}
