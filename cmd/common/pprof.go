package common

import (
	"context"
	"net/http"
	"net/http/pprof"
	"time"

	"github.com/oasisprotocol/nexus/log"
)

// PprofRun runs the pprof server.
func PprofRun(ctx context.Context, endpoint string, logger *log.Logger) error {
	logger = logger.WithModule("pprof")
	logger.Info("starting pprof server", "endpoint", endpoint)

	// Create a new mux just for the pprof endpoints to avoid using the
	// global multiplexer where pprof's init function registers by default.
	mux := http.NewServeMux()
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)

	server := &http.Server{
		Addr:         endpoint,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
		Handler:      mux,
	}

	return RunServer(ctx, server, logger)
}
