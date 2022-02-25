package log

import (
	"fmt"
	"strings"
)

// Level is a log level. It implements the pflag.Value interface.
type Level uint

const (
	// LevelDebug is the log level for debug messages.
	LevelDebug Level = iota
	// LevelInfo is the log level for informative messages.
	LevelInfo
	// LevelWarn is the log level for warning messages.
	LevelWarn
	// LevelError is the log level for error messages.
	LevelError
)

// String returns the string representation of a Level.
func (l *Level) String() string {
	switch *l {
	case LevelDebug:
		return "DEBUG"
	case LevelInfo:
		return "INFO"
	case LevelWarn:
		return "WARN"
	case LevelError:
		return "ERROR"
	default:
		panic("logging: unsupported log level")
	}
}

// Set sets the Level to the value specified by the provided string.
func (l *Level) Set(s string) error {
	switch strings.ToUpper(s) {
	case "DEBUG":
		*l = LevelDebug
	case "INFO":
		*l = LevelInfo
	case "WARN":
		*l = LevelWarn
	case "ERROR":
		*l = LevelError
	default:
		return fmt.Errorf("logging: invalid log level: '%s'", s)
	}

	return nil
}

// Type returns the list of supported Levels.
func (l *Level) Type() string {
	return "[DEBUG,INFO,WARN,ERROR]"
}
