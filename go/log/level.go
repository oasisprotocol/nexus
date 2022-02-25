package log

// Level is a log level.
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
