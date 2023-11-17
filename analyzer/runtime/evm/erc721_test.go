package evm

import (
	"encoding/json"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/require"
)

func TestJSONRoundTrip(t *testing.T) {
	for _, s := range []string{
		// Invalid UTF-8.
		"{\"a\": \"\xff\"}",
		"{\"\xff\": \"a\"}",
		// NUL bytes.
		"{\"a\": \"\x00\"}",
		"{\"\x00\": \"a\"}",
		// Codepoint zero.
		"{\"a\": \"\\u0000\"}",
		"{\"\\u0000\": \"a\"}",
	} {
		var parsed any
		err := json.Unmarshal([]byte(s), &parsed)
		if err != nil {
			// Erroring out while unmarshalling is fine.
			t.Logf("%#v -> parsing: %v\n", s, err)
			continue
		}
		roundTripBuf, err := json.Marshal(parsed)
		if err != nil {
			// Erroring out while marshalling is uncool, but not a showstopper.
			t.Logf("%#v -> stringifying: %v\n", s, err)
			continue
		}
		roundTrip := string(roundTripBuf)
		// If it gets through the round trip, it'd better be clean.
		require.True(t, utf8.ValidString(roundTrip), "%#v -> invalid UTF-8 %#v", s, roundTrip)
		require.NotContains(t, roundTrip, 0x0, "%#v -> contains NUL byte %#v", s, roundTrip)
		t.Logf("%#v -> %#v\n", s, roundTrip)
	}
}
