package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v4/pgxpool"
	ocGrpc "github.com/oasisprotocol/oasis-core/go/common/grpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

const latestBlocksCount int = 10
const latestTransactionsCount int = 10

const IndexerEndpoint = "https://index.oasislabs.com/v1"

func mainFallible(ctx context.Context) error {
	dbPool, err := pgxpool.Connect(ctx, "postgres://postgres:a@172.17.0.2/explorer")
	if err != nil {
		return err
	}

	conn, err := ocGrpc.Dial("grpc.oasis.dev:443", grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{})))
	if err != nil {
		return err
	}

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("(:"))
	})
	r.Mount("/emerald", makeEmeraldRouter(dbPool, conn))
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
