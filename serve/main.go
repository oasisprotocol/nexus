package main

import (
	"context"
	"fmt"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v4/pgxpool"
)

const INDEXER_ENDPOINT = "https://index.oasislabs.com/v1"

func mainFallible(ctx context.Context) error {
	dbPool, err := pgxpool.Connect(ctx, "postgres://postgres:a@172.17.0.2/explorer")
	if err != nil {
		return err
	}

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("(:"))
	})
	r.Mount("/emerald", makeEmeraldRouter(dbPool))
	r.Mount("/consensus", makeConsensusRouter())
	if err = http.ListenAndServe("127.0.0.1:3000", r); err != nil {
		return fmt.Errorf("http.ListenAndServe: %w", err)
	}
	return nil
}

func main() {
	if err := mainFallible(context.Background()); err != nil {
		fmt.Printf("%v\n", err)
		os.Exit(1)
	}
}
