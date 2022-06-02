// Package metrics contains the prometheus infrastructure.
package metrics

import (
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/oasislabs/oasis-indexer/log"
)

const (
	moduleName = "metrics"
)

type PullService struct {
	server *http.Server
	logger *log.Logger
}

// StartInstrumentation starts the pull metrics service.
func (s *PullService) StartInstrumentation() {
	s.logger.Info("Initializing pull metrics service.")
	go s.startHandler()
}

func (s *PullService) startHandler() {
	if err := s.server.ListenAndServe(); err != nil {
		s.logger.Error("Unable to initialize prometheus pull service", "endpoint", s.server.Addr)
	}
}

// Creates a new Prometheus pull service.
func NewPullService(endpoint string, rootLogger *log.Logger) (*PullService, error) {
	logger := rootLogger.WithModule(moduleName)

	return &PullService{
		server: &http.Server{
			Addr:           endpoint,
			Handler:        promhttp.Handler(),
			ReadTimeout:    10 * time.Second,
			WriteTimeout:   10 * time.Second,
			MaxHeaderBytes: 1 << 20,
		},
		logger: logger,
	}, nil
}
