package v1

import (
	"context"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/iancoleman/strcase"
)

type ContextKey string

const (
	// ChainIDContextKey is used to set the relevant chain ID
	// in a request context.
	ChainIDContextKey ContextKey = "chain_id"
	// RequestIDContextKey is used to set a request id for tracing
	// in a request context.
	RequestIDContextKey ContextKey = "request_id"
)

// loggerMiddleware is a middleware that logs the start and end of each request,
// as well as other useful request information.
func (h *Handler) loggerMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := uuid.New()

		h.logger.Info("starting request",
			"endpoint", r.URL.Path,
			"request_id", requestID,
		)

		t := time.Now()
		defer func() {
			h.logger.Info("ending request",
				"endpoint", r.URL.Path,
				"request_id", requestID,
				"time", time.Since(t),
			)
		}()

		next.ServeHTTP(w, r.WithContext(
			context.WithValue(r.Context(), RequestIDContextKey, requestID),
		))
	})
}

// chainMiddleware is a middleware that adds chain-specific information
// to the request context.
func (h *Handler) chainMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		chainID := strcase.ToSnake(LatestChainID)

		// TODO: Set chainID based on provided height params.

		next.ServeHTTP(w, r.WithContext(
			context.WithValue(r.Context(), ChainIDContextKey, chainID),
		))
	})
}
