package common

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBigInt(t *testing.T) {
	var v BigInt

	textRef := []byte("11111111111111111111")
	err := v.UnmarshalText(textRef)
	require.NoError(t, err)
	textRoundTrip, err := v.MarshalText()
	require.NoError(t, err)
	require.Equal(t, textRef, textRoundTrip)

	jsonRef := []byte("\"22222222222222222222\"")
	err = json.Unmarshal(jsonRef, &v)
	require.NoError(t, err)
	jsonRoundTrip, err := json.Marshal(v)
	require.NoError(t, err)
	require.Equal(t, jsonRef, jsonRoundTrip)

	stringRef := "33333333333333333333"
	err = v.Int.UnmarshalText([]byte(stringRef))
	require.NoError(t, err)
	stringRoundTrip := fmt.Sprintf("%v", v)
	require.Equal(t, stringRef, stringRoundTrip)
}
