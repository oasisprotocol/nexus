// Package common implements common oasis-indexer command options.
package common

import (
	"os"

	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"
	"github.com/spf13/viper"

	"github.com/oasislabs/oasis-indexer/log"
	"github.com/oasislabs/oasis-indexer/metrics"
)

const (
	// CfgLogFormat is the flag to set the structured logging format.
	CfgLogFormat = "log.format"

	// CfgLogLevel is the flag to set the minimum severity level to log.
	CfgLogLevel = "log.level"
)

var (
	cfgLogFormat = log.FmtJSON
	cfgLogLevel  = log.LevelInfo

	rootLogger = log.NewDefaultLogger("oasis-indexer")

	// loggingFlags contains common logging flags.
	loggingFlags = flag.NewFlagSet("", flag.ContinueOnError)

	// RootFlags has the flags that are common across all commands.
	RootFlags = flag.NewFlagSet("", flag.ContinueOnError)
)

// Init initializes the common environment.
func Init() error {
	logger, err := log.NewLogger("oasis-indexer", os.Stdout, cfgLogFormat, cfgLogLevel)
	if err != nil {
		return err
	}
	rootLogger = logger

	// Initialize Prometheus service.
	promServer, err := metrics.NewPullService(rootLogger)
	if err != nil {
		rootLogger.Error("failed to initialize metrics", "err", err)
		os.Exit(1)
	}
	promServer.StartInstrumentation()

	return nil
}

// Logger returns the logger defined by logging flags.
func Logger() *log.Logger {
	return rootLogger
}

func RegisterFlags(cmd *cobra.Command) {
	loggingFlags.Var(&cfgLogFormat, CfgLogFormat, "structured logging format")
	loggingFlags.Var(&cfgLogLevel, CfgLogLevel, "minimum logging severity level")

	cmd.PersistentFlags().AddFlagSet(loggingFlags)

	// Add to viper. Although currently unused, we will need viper in the near future
	// for reading env variables.
	for _, v := range []string{
		CfgLogFormat,
		CfgLogLevel,
	} {
		_ = viper.BindPFlag(v, cmd.Flags().Lookup(v))
	}
}
