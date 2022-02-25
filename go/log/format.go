package log

import (
	"fmt"
	"strings"
)

// Format is a logging format. It implements the pflag.Value interface.
type Format uint

const (
	// FmtLogfmt is the "logfmt" logging format.
	FmtLogfmt Format = iota
	// FmtJSON is the JSON logging format.
	FmtJSON
)

// String returns the string representation of a Format.
func (f *Format) String() string {
	switch *f {
	case FmtLogfmt:
		return "logfmt"
	case FmtJSON:
		return "JSON"
	default:
		panic("logging: unsupported format")
	}
}

// Set sets the Format to the value specified by the provided string.
func (f *Format) Set(s string) error {
	switch strings.ToUpper(s) {
	case "LOGFMT":
		*f = FmtLogfmt
	case "JSON":
		*f = FmtJSON
	default:
		return fmt.Errorf("logging: invalid log format: '%s'", s)
	}

	return nil
}

// Type returns the list of supported Formats.
func (f *Format) Type() string {
	return "[logfmt,JSON]"
}
