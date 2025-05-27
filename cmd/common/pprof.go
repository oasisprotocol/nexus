package common

import (
	"net"
	"net/http"
	"net/http/pprof"
	"time"
)

func startPprof(endpoint string) {
	listener, err := net.Listen("tcp", endpoint)
	if err != nil {
		rootLogger.Error("failed to create pprof listener", "err", err)
		return
	}

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
	}

	go func() {
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			rootLogger.Error("pprof server stopped", "err", err)
		}
	}()
}
