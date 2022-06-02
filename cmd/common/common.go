// Package common implements common oasis-indexer command options.
package common

import (
	"fmt"
	"io"
	"os"

	"github.com/oasislabs/oasis-indexer/config"
	"github.com/oasislabs/oasis-indexer/log"
	"github.com/oasislabs/oasis-indexer/metrics"
)

var (
	rootLogger = log.NewDefaultLogger("oasis-indexer")
)

// Init initializes the common environment.
func Init(cfg *config.Config) error {
	var w io.Writer = os.Stdout
	format := log.FmtJSON
	level := log.LevelDebug
	if cfg.Log != nil {
		var err error
		if w, err = getLoggingStream(cfg.Log); err != nil {
			return fmt.Errorf("opening log file: %w", err)
		}
		if err := format.Set(cfg.Log.Format); err != nil {
			return err
		}
		if err := level.Set(cfg.Log.Level); err != nil {
			return err
		}
	}
	logger, err := log.NewLogger("oasis-indexer", w, format, level)
	if err != nil {
		return err
	}
	rootLogger = logger

	// Initialize Prometheus service.
	if cfg.Metrics != nil {
		promServer, err := metrics.NewPullService(cfg.Metrics.PullEndpoint, rootLogger)
		if err != nil {
			rootLogger.Error("failed to initialize metrics", "err", err)
			os.Exit(1)
		}
		promServer.StartInstrumentation()
	}

	return nil
}

// Logger returns the logger defined by logging flags.
func Logger() *log.Logger {
	return rootLogger
}

func getLoggingStream(cfg *config.LogConfig) (io.Writer, error) {
	if cfg == nil || cfg.File == "" {
		return os.Stdout, nil
	}
	w, err := os.OpenFile(cfg.File, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, err
	}
	return w, nil
}
