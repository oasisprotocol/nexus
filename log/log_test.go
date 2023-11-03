package log

import (
	"bytes"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

const tsRegex = `\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d{0,9}Z`

func TestLoggerLogfmt(t *testing.T) {
	var b bytes.Buffer
	l, err := NewLogger("log-test", &b, FmtLogfmt, LevelDebug)
	require.Nil(t, err)

	l.Debug("a statement")
	require.Regexp(t, regexp.MustCompile(
		`level=debug ts=`+tsRegex+` caller=log_test\.go:\d{1,4} module=log-test msg="a statement"`),
		b.String())
}

func TestLoggerJSON(t *testing.T) {
	var b bytes.Buffer
	l, err := NewLogger("log-test", &b, FmtJSON, LevelDebug)
	require.Nil(t, err)

	l.Debug("a statement")
	//nolint:goconst
	require.Regexp(t, regexp.MustCompile(
		`{"caller":"log_test\.go:\d{1,4}","level":"debug","module":"log-test","msg":"a statement","ts":"`+tsRegex+`"}\n`),
		b.String())
}

func TestLoggerInvalid(t *testing.T) {
	var b bytes.Buffer
	_, err := NewLogger("log-test", &b, Format(255), LevelDebug)
	require.NotNil(t, err)
}

func TestWith(t *testing.T) {
	var b bytes.Buffer
	l, err := NewLogger("log-test", &b, FmtJSON, LevelDebug)
	require.Nil(t, err)

	l.With("height", 8000000).Debug("a statement")
	require.Regexp(t, regexp.MustCompile(
		`{"caller":"log_test\.go:\d{1,4}","height":8000000,"level":"debug","module":"log-test","msg":"a statement","ts":"`+tsRegex+`"}\n`),
		b.String())
}

func TestWithModule(t *testing.T) {
	var b bytes.Buffer
	l, err := NewLogger("log-test", &b, FmtJSON, LevelDebug)
	require.Nil(t, err)

	l.WithModule("log-test-2").Debug("a statement")
	require.Regexp(t, regexp.MustCompile(
		`{"caller":"log_test\.go:\d{1,4}","level":"debug","module":"log-test-2","msg":"a statement","ts":"`+tsRegex+`"}\n`),
		b.String())
}

func TestDebug(t *testing.T) {
	var b bytes.Buffer
	l, err := NewLogger("log-test", &b, FmtJSON, LevelInfo)
	require.Nil(t, err)

	l.Debug("a statement")
	require.Equal(t, 0, b.Len())

	l, err = NewLogger("log-test", &b, FmtJSON, LevelDebug)
	require.Nil(t, err)

	l.Debug("another statement")
	require.NotEqual(t, 0, b.Len())
}

func TestInfo(t *testing.T) {
	var b bytes.Buffer
	l, err := NewLogger("log-test", &b, FmtJSON, LevelWarn)
	require.Nil(t, err)

	l.Info("a statement")
	require.Equal(t, 0, b.Len())

	l, err = NewLogger("log-test", &b, FmtJSON, LevelInfo)
	require.Nil(t, err)

	l.Info("another statement")
	require.NotEqual(t, 0, b.Len())
}

func TestWarn(t *testing.T) {
	var b bytes.Buffer
	l, err := NewLogger("log-test", &b, FmtJSON, LevelError)
	require.Nil(t, err)

	l.Warn("a statement")
	require.Equal(t, 0, b.Len())

	l, err = NewLogger("log-test", &b, FmtJSON, LevelWarn)
	require.Nil(t, err)

	l.Warn("another statement")
	require.NotEqual(t, 0, b.Len())
}

func TestError(t *testing.T) {
	var b bytes.Buffer
	l, err := NewLogger("log-test", &b, FmtJSON, LevelError)
	require.Nil(t, err)

	l.Error("a statement")
	require.NotEqual(t, 0, b.Len())
}

func TestLevel(t *testing.T) {
	var lvl Level
	ls := lvl.Type()

	for _, l := range strings.Split(ls[1:len(ls)-1], ",") {
		err := lvl.Set(l)
		require.Nil(t, err)
		require.Equal(t, l, lvl.String())
	}
	err := lvl.Set("invalid")
	require.NotNil(t, err)

	lvl = Level(255)
	require.Panics(t, func() { _ = lvl.String() })
}

func TestFormat(t *testing.T) {
	var fmt Format
	fs := fmt.Type()

	for _, f := range strings.Split(fs[1:len(fs)-1], ",") {
		err := fmt.Set(f)
		require.Nil(t, err)
		require.Equal(t, f, fmt.String())
	}
	err := fmt.Set("invalid")
	require.NotNil(t, err)

	fmt = Format(255)
	require.Panics(t, func() { _ = fmt.String() })
}
