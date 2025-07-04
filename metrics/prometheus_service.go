// Package metrics contains the prometheus infrastructure.
package metrics

import (
	"context"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	cmdCommon "github.com/oasisprotocol/nexus/cmd/common"
	"github.com/oasisprotocol/nexus/log"
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
func (s *PullService) Run(ctx context.Context) error {
	http.Handle("/metrics", promhttp.Handler())

	server := &http.Server{
		Addr:           s.pullEndpoint,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}

	return cmdCommon.RunServer(ctx, server, s.logger)
}

// Creates a new Prometheus pull service.
func NewPullService(pullEndpoint string, logger *log.Logger) (*PullService, error) {
	return &PullService{
		pullEndpoint: pullEndpoint,
		logger:       logger.WithModule(moduleName),
	}, nil
}
