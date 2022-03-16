// Package common implements common oasis-indexer command options.
package common

import (
	"os"

	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"
	"github.com/spf13/viper"

	"github.com/oasislabs/oasis-block-indexer/go/log"
)

const (
	// CfgLogFormat is the flag to set the structured logging format.
	cfgLogFormat = "log.format"

	// CfgLogLevel is the flag to set the minimum severity level to log.
	cfgLogLevel = "log.level"
)

var (
	flagLogFormat = log.FmtJSON
	flagLogLevel  = log.LevelInfo

	RootLogger = log.NewDefaultLogger("oasis-indexer")

	// loggingFlags contains common logging flags.
	loggingFlags = flag.NewFlagSet("", flag.ContinueOnError)

	// RootFlags has the flags that are common across all commands.
	RootFlags = flag.NewFlagSet("", flag.ContinueOnError)
)

// Init initializes the common environment.
func Init() error {
	logger, err := log.NewLogger("oasis-indexer", os.Stdout, flagLogFormat, flagLogLevel)
	if err != nil {
		return err
	}
	RootLogger = logger

	return nil
}

// Logger returns the logger defined by logging flags.
func Logger() *log.Logger {
	return rootLogger
}

func RegisterFlags(cmd *cobra.Command) {
	loggingFlags.Var(&flagLogFormat, cfgLogFormat, "structured logging format")
	loggingFlags.Var(&flagLogLevel, cfgLogLevel, "minimum logging severity level")

	cmd.PersistentFlags().AddFlagSet(loggingFlags)

	// Add to viper. Although currently unused, we will need viper in the near future
	// for reading env variables.
	for _, v := range []string{
		cfgLogFormat,
		cfgLogLevel,
	} {
		_ = viper.BindPFlag(v, cmd.Flags().Lookup(v))
	}
}
