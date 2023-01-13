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
	"github.com/oasisprotocol/oasis-indexer/log"
	"github.com/oasisprotocol/oasis-indexer/metrics"
)

var (
	defaultOffset            = uint64(0)
	defaultLimit             = uint64(100)
	defaultBucketSizeSeconds = uint32(3600)
	maxLimit                 = uint64(1000)
)

// MetricsMiddleware is a middleware that measures the start and end of each request,
// as well as other useful request information.
func MetricsMiddleware(m metrics.RequestMetrics, logger log.Logger) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestID := uuid.New()

			logger.Info("starting request",
				"endpoint", r.URL.Path,
				"request_id", requestID,
			)

			t := time.Now()
			timer := m.RequestTimer(r.URL.Path)
			defer func() {
				logger.Info("ending request",
					"endpoint", r.URL.Path,
					"request_id", requestID,
					"time", time.Since(t),
					"status_code", reflect.ValueOf(w).Elem().FieldByName("status").Int(),
				)
				timer.ObserveDuration()
			}()

			next.ServeHTTP(w, r.WithContext(
				context.WithValue(r.Context(), common.RequestIDContextKey, requestID),
			))
		})
	}
}

// ChainMiddleware is a middleware that adds chain-specific information
// to the request context.
func ChainMiddleware(chainID string) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// snake_case the chainID; it's how it's used to name the DB schemas.
			chainIDSnake := strcase.ToSnake(chainID)

			// TODO: Set chainID based on provided height params.

			next.ServeHTTP(w, r.WithContext(
				context.WithValue(r.Context(), common.ChainIDContextKey, chainIDSnake),
			))
		})
	}
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
			case "BucketSizeSeconds":
				f.Set(reflect.ValueOf(&defaultBucketSizeSeconds))
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

// Find the json annotation on a struct field, and return the json specified
// name if available, otherwise, just the field name.
func jsonName(f reflect.StructField) string {
	fieldName := f.Name
	tag := f.Tag.Get("json")
	if tag != "" {
		tagParts := strings.Split(tag, ",")
		name := tagParts[0]
		if name != "" {
			fieldName = name
		}
	}
	return fieldName
}

// ParseBigIntParamsMiddleware fixes the parsing of URL query parameters of type *BigInt.
// oapi-codegen does not really support reading URL query params into structs (but see note below).
// This middleware reproduces a portion of oapi-codegen's param-fetching logic, but then parses
// the input string with `UnmarshalText()`.
//
// LIMITATIONS: The middleware relies on assumptions that happen to hold for oasis-indexer:
//   - only works for `*BigInt` (not `BigInt`)
//   - only works for `*BigInt` fields directly under `Params`, not nested in other structs.
//   - only works for URL query parameters (like ?myNumber=123), not path parameters
//     (like .../foo/123?...) or HTTP body data.
//
// NOTE: oapi-codegen _does_ support some custom type parsing, so we don't need to patch their parsing here.
// Date and Time are two hardcoded supported structs. Also, non-struct typedefs (like `type Address [21]byte`,
// which is our `staking.Address`) work fine.
func ParseBigIntParamsMiddleware(next apiTypes.StrictHandlerFunc, _operationID string) apiTypes.StrictHandlerFunc {
	return func(ctx context.Context, w http.ResponseWriter, r *http.Request, args interface{}) (interface{}, error) {
		// Create a new struct of the same type as `args` and copy the values from `args` into it.
		// This new struct will be "addressable" (= modifiable), unlike `args`.
		argsV := reflect.ValueOf(args)
		v := reflect.New(argsV.Type())
		v.Elem().Set(argsV)

		// Use reflection to check if `args.Params` has any fields of type *BigInt.
		// If it does, we'll parse its value from the request.
		if v.Elem().Kind() == reflect.Struct { //nolint:nestif
			paramsV := v.Elem().FieldByName("Params")
			if paramsV.IsValid() && paramsV.Kind() == reflect.Struct {
				// Iterate through fields of Params.
				for i := 0; i < paramsV.NumField(); i++ {
					f := paramsV.Field(i)
					// For every *BigInt member of Params:
					if f.Type() == reflect.TypeOf(&common.BigInt{}) {
						// Fetch the string value from the query URL.
						queryKey := jsonName(paramsV.Type().Field(i))
						queryValue := r.URL.Query().Get(queryKey)
						if queryValue == "" {
							continue // no value in the query URL, skip
						}
						// Parse the string value into a *BigInt.
						bigInt := &common.BigInt{}
						if err := bigInt.UnmarshalText([]byte(queryValue)); err != nil {
							return nil, &apiTypes.InvalidParamFormatError{ParamName: queryKey, Err: err}
						}
						f.Set(reflect.ValueOf(bigInt))
					}
				}
			}
		}

		// Call the next middleware in the chain with fixed `args`.
		return next(ctx, w, r, v.Elem().Interface())
	}
}

// RuntimeFromURLMiddleware extracts the runtime from the URL and sets it in the request context.
// The runtime is expected to be the first part of the path after the `baseURL` (e.g. "/v1").
func RuntimeFromURLMiddleware(baseURL string) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			path := strings.TrimPrefix(r.URL.Path, baseURL)

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
}
