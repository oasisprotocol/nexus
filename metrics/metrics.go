// Package metrics contains the prometheus infrastructure.
package metrics

import (
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"
	"github.com/spf13/viper"

	"github.com/oasislabs/oasis-indexer/log"
)

const (
	// CfgMetricsPullEndpoint is the endpoint at which Prometheus pull metrics will be exposed.
	CfgMetricsPullEndpoint = "metrics.pull_endpoint"

	moduleName = "metrics"
)

var (
	cfgMetricsPullEndpoint string

	metricsFlags = flag.NewFlagSet("", flag.ContinueOnError)
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
		s.logger.Error("Unable to initialize prometheus pull service", "endpoint", cfgMetricsPullEndpoint)
	}
}

// Register registers the flags for configuring a metrics service.
func Register(cmd *cobra.Command) {
	metricsFlags.StringVar(&cfgMetricsPullEndpoint, CfgMetricsPullEndpoint, "localhost:8009", "metrics endpoint at which metrics will be exposed for Prometheus")

	cmd.PersistentFlags().AddFlagSet(metricsFlags)

	for _, v := range []string{
		CfgMetricsPullEndpoint,
	} {
		_ = viper.BindPFlag(v, cmd.Flags().Lookup(v))
	}
}

// Creates a new Prometheus pull service.
func NewPullService(rootLogger *log.Logger) (*PullService, error) {
	logger := rootLogger.WithModule(moduleName)

	return &PullService{
		server: &http.Server{
			Addr:           cfgMetricsPullEndpoint,
			Handler:        promhttp.Handler(),
			ReadTimeout:    10 * time.Second,
			WriteTimeout:   10 * time.Second,
			MaxHeaderBytes: 1 << 20,
		},
		logger: logger,
	}, nil
}
