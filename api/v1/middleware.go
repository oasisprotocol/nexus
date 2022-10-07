package v1

import (
	"context"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/iancoleman/strcase"

	storage "github.com/oasisprotocol/oasis-indexer/storage/client"
)

// metricsMiddleware is a middleware that measures the start and end of each request,
// as well as other useful request information.
func (h *Handler) metricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := uuid.New()

		h.logger.Info("starting request",
			"endpoint", r.URL.Path,
			"request_id", requestID,
		)

		t := time.Now()
		timer := h.metrics.RequestTimer(r.URL.Path)
		defer func() {
			h.logger.Info("ending request",
				"endpoint", r.URL.Path,
				"request_id", requestID,
				"time", time.Since(t),
			)
			timer.ObserveDuration()
		}()

		next.ServeHTTP(w, r.WithContext(
			context.WithValue(r.Context(), storage.RequestIDContextKey, requestID),
		))
	})
}

// chainMiddleware is a middleware that adds chain-specific information
// to the request context.
func (h *Handler) chainMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		chainID := strcase.ToSnake(h.client.chainID)

		// TODO: Set chainID based on provided height params.

		next.ServeHTTP(w, r.WithContext(
			context.WithValue(r.Context(), storage.ChainIDContextKey, chainID),
		))
	})
}
