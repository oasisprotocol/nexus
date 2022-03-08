// Package common implements common oasis-indexer command options.package common
package common

import (
	"os"

	flag "github.com/spf13/pflag"

	"github.com/oasislabs/oasis-block-indexer/go/log"
)

const (
	// CfgLogLevel is the flag to set the minimum severity level to log.
	CfgLogLevel = "log.level"

	// CfgLogFormat is the flag to set the structured logging format.
	CfgLogFormat = "log.format"
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

	return nil
}

// Logger returns the logger defined by logging flags.
func Logger() *log.Logger {
	return rootLogger
}

func init() {
	loggingFlags.Var(&cfgLogFormat, CfgLogFormat, "structured logging format")
	loggingFlags.Var(&cfgLogLevel, CfgLogLevel, "minimum logging severity level")

	RootFlags.AddFlagSet(loggingFlags)
}
