// Package log implements support for structured logging.
package log

import (
	"fmt"
	"io"
	"os"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
)

// Logger is a structured logger.
type Logger struct {
	logger log.Logger
	level  Level
	module string
}

// NewDefaultLogger initializes a new logger instance with default settings.
func NewDefaultLogger(module string) *Logger {
	return &Logger{
		logger: log.NewJSONLogger(log.NewSyncWriter(os.Stdout)),
		level:  LevelInfo,
		module: module,
	}
}

// NewLogger initializes a new logger instance.
func NewLogger(module string, w io.Writer, format Format, lvl Level) (*Logger, error) {
	switch format {
	case FmtLogfmt:
		return &Logger{
			logger: log.NewLogfmtLogger(log.NewSyncWriter(w)),
			level:  lvl,
			module: module,
		}, nil
	case FmtJSON:
		return &Logger{
			logger: log.NewJSONLogger(log.NewSyncWriter(w)),
			level:  lvl,
			module: module,
		}, nil
	default:
		return nil, fmt.Errorf("log: unsupported log format: %v", format)
	}
}

// Debug logs the message and key value pairs at the Debug log level.
func (l *Logger) Debug(msg string, keyvals ...interface{}) {
	if l.level > LevelDebug {
		return
	}
	keyvals = append([]interface{}{"module", l.module, "msg", msg}, keyvals...)
	_ = level.Debug(l.logger).Log(keyvals...)
}

// Info logs the message and key value pairs at the Info log level.
func (l *Logger) Info(msg string, keyvals ...interface{}) {
	if l.level > LevelInfo {
		return
	}
	keyvals = append([]interface{}{"module", l.module, "msg", msg}, keyvals...)
	_ = level.Info(l.logger).Log(keyvals...)
}

// Warn logs the message and key value pairs at the Warn log level.
func (l *Logger) Warn(msg string, keyvals ...interface{}) {
	if l.level > LevelWarn {
		return
	}
	keyvals = append([]interface{}{"module", l.module, "msg", msg}, keyvals...)
	_ = level.Warn(l.logger).Log(keyvals...)
}

// Error logs the message and key value pairs at the Error log level.
func (l *Logger) Error(msg string, keyvals ...interface{}) {
	if l.level > LevelError {
		return
	}
	keyvals = append([]interface{}{"module", l.module, "msg", msg}, keyvals...)
	_ = level.Error(l.logger).Log(keyvals...)
}

// With returns a clone of the logger with the provided key/value pairs
// added as context for all subsequent logs.
func (l *Logger) With(keyvals ...interface{}) Logger {
	return &Logger{
		logger: log.With(l.logger, keyvals...),
	}
}

// WithPrefix returns a clone of the logger with the provided key/value pairs
// added as context for all subsequent logs.
func (l *Logger) WithPrefix(keyvals ...interface{}) Logger {
	return &Logger{
		logger: log.WithPrefix(l.logger, keyvals...),
	}
}

// WithModule returns a clone of the logger with the provided module
// added as context for all subsequent logs.
func (l *Logger) WithModule(module string) *Logger {
	return &Logger{
		logger: l.logger,
		level:  l.level,
		module: module,
	}
}
