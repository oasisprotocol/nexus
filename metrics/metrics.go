// Package metrics contains the prometheus infrastructure.
package metrics

import (
	"fmt"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"
	"github.com/spf13/viper"

	"github.com/oasislabs/oasis-indexer/log"
)

const (
	// CfgMetricsPullAddr is the address at which Prometheus metrics will be exposed
	CfgMetricsPullAddr = "metrics.pull.addr"

	// CfgMetricsPullPort is the port at which Prometheus metrics will be exposed
	CfgMetricsPullPort = "metrics.pull.port"

	moduleName = "metrics"
)

var (
	cfgMetricsPullAddr string
	cfgMetricsPullPort string

	metricsFlags = flag.NewFlagSet("", flag.ContinueOnError)
)

type pullService struct {
	server *http.Server
	logger *log.Logger
}

func (s *pullService) StartInstrumentation() {
	s.logger.Info("Initializing pull metrics service.")
	go s.startHandler()
}

func (s *pullService) startHandler() {
	if err := s.server.ListenAndServe(); err != nil {
		s.logger.Error("Unable to initialize prometheus pull service", "listen_addr", cfgMetricsPullAddr, "listen_port", cfgMetricsPullPort)
	}
}

// Register registers the flags for configuring a metrics service.
func Register(cmd *cobra.Command) {
	metricsFlags.StringVar(&cfgMetricsPullAddr, CfgMetricsPullAddr, "localhost", "Prometheus metrics address, at which metrics will be exposed")
	metricsFlags.StringVar(&cfgMetricsPullPort, CfgMetricsPullPort, "7000", "Prometheus metrics port, at which metrics will be exposed")

	cmd.PersistentFlags().AddFlagSet(metricsFlags)

	for _, v := range []string{
		CfgMetricsPullAddr,
		CfgMetricsPullPort,
	} {
		_ = viper.BindPFlag(v, cmd.Flags().Lookup(v))
	}
}

// Creates a new prometheus pull service.
func NewPullService(rootLogger *log.Logger) (*pullService, error) {
	logger := rootLogger.With(rootLogger, "pkg", "metrics")
	addr := fmt.Sprintf("%s:%s", cfgMetricsPullAddr, cfgMetricsPullPort)

	return &pullService{
		server: &http.Server{
			Addr:           addr,
			Handler:        promhttp.Handler(),
			ReadTimeout:    10 * time.Second,
			WriteTimeout:   10 * time.Second,
			MaxHeaderBytes: 1 << 20,
		},
		logger: logger,
	}, nil
}
