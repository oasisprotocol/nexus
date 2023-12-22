package common

import (
	"encoding/json"
	"fmt"
	"math"
	"testing"

	"github.com/oasisprotocol/oasis-core/go/common/cbor"
	"github.com/stretchr/testify/require"
)

func TestBigIntJSON(t *testing.T) {
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

func TestBigIntPlus(t *testing.T) {
	a := NewBigInt(10)
	b := a.Plus(NewBigInt(1))
	require.EqualValues(t, 10, a.Int64())
	require.EqualValues(t, 11, b.Int64())
}

func TestBigIntTimes(t *testing.T) {
	a := NewBigInt(10)
	b := a.Times(NewBigInt(-2))
	require.EqualValues(t, 10, a.Int64())
	require.EqualValues(t, -20, b.Int64())
}

func TestBigIntCBOR(t *testing.T) {
	// Test round-trip for the following values
	for _, b := range []BigInt{
		NewBigInt(0),
		NewBigInt(-1),
		NewBigInt(1),
		NewBigInt(-1000),
		NewBigInt(1000),
		NewBigInt(math.MaxInt64).Times(NewBigInt(math.MaxInt64)),
		NewBigInt(math.MinInt64).Times(NewBigInt(math.MaxInt64)),
	} {
		var roundtripped BigInt
		serialized := cbor.Marshal(b)
		err := cbor.Unmarshal(serialized, &roundtripped)
		require.NoError(t, err)
		require.Equal(t, b.String(), roundtripped.String(), "CBOR serialization should round-trip")
	}
}
