package log

// Format is a logging format.
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
