package main

import (
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

const INDEXER_ENDPOINT = "https://index.oasislabs.com/v1"

func main() {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("(:"))
	})
	r.Mount("/emerald", makeEmeraldRouter())
	r.Mount("/consensus", makeConsensusRouter())
	if err := http.ListenAndServe("127.0.0.1:3000", r); err != nil {
		panic(fmt.Errorf("http.ListenAndServer: %w", err))
	}
}
