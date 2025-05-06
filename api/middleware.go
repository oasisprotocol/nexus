package api

import (
	"context"
	"net/http"
	"reflect"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/rs/cors"

	apiTypes "github.com/oasisprotocol/nexus/api/v1/types"
	"github.com/oasisprotocol/nexus/common"
	"github.com/oasisprotocol/nexus/log"
	"github.com/oasisprotocol/nexus/metrics"
)

var (
	defaultOffset            = uint64(0)
	defaultLimit             = uint64(100)
	defaultWindowSizeSeconds = uint32(86400)
	defaultWindowStepSeconds = uint32(86400)
	maxLimit                 = uint64(1000)
)

// normalizeEndpoint removes all unique identifiers from the URL in order to
// make it possible to group the Prometheus metrics nicely.
func normalizeEndpoint(url string) string {
	var nels []string

	els := strings.Split(url, "/")
	for _, e := range els {
		// All unique IDs that we use are some hashes or integers, so we can
		// just cut everything that's too long or looks like an int here.
		//
		// In the future, a better solution would be to look at the OpenAPI
		// declaration and pass only non-parametrized parts of the query
		// through, but that might be over-engineering.
		isTooLong := len(e) >= 32
		isInt := len(e) > 0 && strings.IndexFunc(e, func(c rune) bool { return c < '0' || c > '9' }) == -1
		if isTooLong || isInt {
			nels = append(nels, "*")
		} else {
			nels = append(nels, e)
		}
	}

	return strings.Join(nels, "/")
}

// MetricsMiddleware is a middleware that measures the start and end of each request,
// as well as other useful request information.
// It should be used as the outermost middleware, so it can
// - set a requestID and make it available to all handlers and
// - observe the final HTTP status code at the end of the request.
func MetricsMiddleware(m metrics.RequestMetrics, logger log.Logger) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Pre-work and initial logging.
			requestID := uuid.New()
			logger.Info("starting request",
				"endpoint", r.URL.Path,
				"request_id", requestID,
			)
			t := time.Now()
			metricName := normalizeEndpoint(r.URL.Path)

			// Serve the request.
			next.ServeHTTP(w, r.WithContext(
				context.WithValue(r.Context(), common.RequestIDContextKey, requestID),
			))

			// Observe results and log/record them.
			httpStatus := reflect.ValueOf(w).Elem().FieldByName("status").Int()
			latency := time.Since(t)
			logger.Info("ending request",
				"query_path", r.URL.Path,
				"query_params", r.URL.RawQuery,
				"request_id", requestID,
				"latency", latency,
				"latency_bin", binQueryLatency(latency),
				"status_code", httpStatus,
			)

			statusTxt := "failure"
			if httpStatus >= 200 && httpStatus < 400 {
				statusTxt = "success"
			} else if httpStatus >= 400 && httpStatus < 500 {
				// Group all 4xx errors into the "ignored" label, as they're
				// almost always just bots and are not relevant to the metrics
				// we're interested in.
				statusTxt = "failure_4xx"
				metricName = "ignored"
			}
			// Ensure metric names are valid UTF-8 strings to prevent Prometheus panics.
			// While the HTTP server should normally handle this, we add a safety check
			// here as well.
			if !utf8.ValidString(metricName) {
				logger.Debug("invalid metric name", "metric_name", metricName)
				metricName = "ignored"
				statusTxt = "non_utf8_path"
			}
			m.RequestCounts(metricName, statusTxt).Inc()
			m.RequestLatencies(metricName).Observe(latency.Seconds())
		})
	}
}

// Bin request durations to make it easier to search
// for slow queries in Nexus logs.
func binQueryLatency(t time.Duration) string {
	switch {
	case t < 100*time.Millisecond:
		return "<100ms"
	case t < 300*time.Millisecond:
		return "100-300ms"
	case t < 500*time.Millisecond:
		return "300-500ms"
	case t < 1000*time.Millisecond:
		return "500-1000ms"
	default:
		return ">1000ms"
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
			case "WindowSizeSeconds":
				f.Set(reflect.ValueOf(&defaultWindowSizeSeconds))
			case "WindowStepSeconds":
				f.Set(reflect.ValueOf(&defaultWindowStepSeconds))
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
// LIMITATIONS: The middleware relies on assumptions that happen to hold for nexus:
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

// CorsMiddleware is a restrictive CORS middleware that only allows GET requests.
//
// NOTE: To support other methods (e.g. POST), we'd also need to support OPTIONS
// preflight requests, in which case this would have to be the outermost handler
// to run; the openapi-generated handler will reject OPTIONS requests because
// they are not in the openapi spec.
var CorsMiddleware func(http.Handler) http.Handler = cors.New(cors.Options{
	AllowedMethods: []string{
		http.MethodGet,
	},
	AllowCredentials: false,
}).Handler
