package metrics

import (
	"fmt"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"
	"github.com/spf13/viper"

	"github.com/oasislabs/oasis-block-indexer/go/log"
)

const (
	cfgMetricsPullAddr = "metrics.pull.addr"
	cfgMetricsPullPort = "metrics.pull.port"
)

var (
	flagMetricsPullAddr string
	flagMetricsPullPort string

	metricsFlags = flag.NewFlagSet("", flag.ContinueOnError)
)

type pullService struct {
	server *http.Server
	logger log.Logger
}

func (s *pullService) StartInstrumentation() {
	s.logger.Info("Initializing pull metrics service.")
	go s.startHandler()
}

func (s *pullService) startHandler() {
	if err := s.server.ListenAndServe(); err != nil {
		s.logger.Error("Unable to initialize prometheus pull service", "listen_addr", flagMetricsPullAddr, "listen_port", flagMetricsPullPort)
	}
}

func Register(cmd *cobra.Command) {
	metricsFlags.StringVar(&flagMetricsPullAddr, cfgMetricsPullAddr, "localhost", "Prometheus metrics address, at which metrics will be exposed")
	metricsFlags.StringVar(&flagMetricsPullPort, cfgMetricsPullPort, "7000", "Prometheus metrics port, at which metrics will be exposed")

	cmd.PersistentFlags().AddFlagSet(metricsFlags)

	for _, v := range []string{
		cfgMetricsPullAddr,
		cfgMetricsPullPort,
	} {
		_ = viper.BindPFlag(v, cmd.Flags().Lookup(v))
	}
}

func NewPullService(logger log.Logger) (*pullService, error) {
	logger = logger.With(logger, "pkg", "metrics")
	addr := fmt.Sprintf("%s:%s", flagMetricsPullAddr, flagMetricsPullPort)

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
