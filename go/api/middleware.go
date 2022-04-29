package api

import (
	"context"
	"net/http"

	"github.com/iancoleman/strcase"
)

type ContextKey string

const (
	ChainIDContextKey = "chain_id"
)

// loggerMiddleware is a middleware that logs the start and end of each request,
// as well as other useful request information.
func loggerMiddleware(next http.Handler) http.Handler {
	// TODO.
	return next
}

// chainMiddleware is a middleware that adds chain-specific information
// to the request context.
func chainMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		chainID := strcase.ToSnake(LatestChainID)

		// TODO: Set chainID based on provided height params.

		next.ServeHTTP(w, r.WithContext(
			context.WithValue(r.Context(), ChainIDContextKey, chainID),
		))
	})
}
