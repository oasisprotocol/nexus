// Package metrics contains the prometheus infrastructure.
package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/oasisprotocol/oasis-indexer/log"
)

const (
	moduleName = "metrics"
)

// PullService is a service that supports the Prometheus pull method.
type PullService struct {
	pullEndpoint string
	logger       *log.Logger
}

// StartInstrumentation starts the pull metrics service.
func (s *PullService) StartInstrumentation() {
	s.logger.Info("initializing pull metrics service")
	go s.startHandler()
}

func (s *PullService) startHandler() {
	http.Handle("/metrics", promhttp.Handler())

	if err := http.ListenAndServe(s.pullEndpoint, nil); err != nil { //nolint:gosec // G114: Use of net/http serve function that has no support for setting timeouts
		s.logger.Error("unable to initialize prometheus pull service",
			"endpoint", s.pullEndpoint,
			"error", err,
		)
	}
}

// Creates a new Prometheus pull service.
func NewPullService(pullEndpoint string, rootLogger *log.Logger) (*PullService, error) {
	logger := rootLogger.WithModule(moduleName)

	return &PullService{
		pullEndpoint: pullEndpoint,
		logger:       logger,
	}, nil
}
