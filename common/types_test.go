package common

import (
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
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

func TestBigIntMinus(t *testing.T) {
	a := NewBigInt(10)
	b := a.Minus(NewBigInt(1))
	require.EqualValues(t, 10, a.Int64())
	require.EqualValues(t, 9, b.Int64())
}

func TestBigEq(t *testing.T) {
	require.True(t, NewBigInt(10).Eq(NewBigInt(10)))
	require.False(t, NewBigInt(10).Eq(NewBigInt(-10)))
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

func TestBigDecimalJSON(t *testing.T) {
	var v BigDecimal

	jsonRef := []byte("\"12345.678901234567890123\"")
	err := json.Unmarshal(jsonRef, &v)
	require.NoError(t, err)
	jsonRoundTrip, err := json.Marshal(v)
	require.NoError(t, err)
	require.Equal(t, jsonRef, jsonRoundTrip)

	stringRef := "12345.678901234567890123"
	err = v.Decimal.UnmarshalText([]byte(stringRef))
	require.NoError(t, err)
	stringRoundTrip := v.String()
	require.Equal(t, stringRef, stringRoundTrip)
}

func TestBigDecimalNumeric(t *testing.T) {
	for _, tc := range []struct {
		value string
	}{
		{value: "12.345"},
		{value: "678.90"},
		{value: "12345.678901234567890123"},
		{value: "0.00000123456789"},
		{value: "0"},
		{value: "123456"},
		{value: "0.0000000000000000000043024235"},
	} {
		dec, err := NewBigDecimal(tc.value)
		require.NoError(t, err, "failed to create BigDecimal from string %s", tc.value)

		val, err := dec.NumericValue()
		require.NoError(t, err, "failed to convert BigDecimal to Numeric for value %s", tc.value)

		var roundTripped BigDecimal
		err = roundTripped.ScanNumeric(val)
		require.NoError(t, err, "failed to scan Numeric into BigDecimal for value %s", tc.value)

		require.Equal(t, dec.String(), roundTripped.String(), "BigDecimal should match after Numeric conversion for value %s", tc.value)
		require.EqualValues(t, dec, roundTripped, "BigDecimal should match after Numeric conversion for value %s", tc.value)
	}
}

func TestNumericToBigInt(t *testing.T) {
	for _, tc := range []struct {
		name     string
		numeric  pgtype.Numeric
		expected int64
		hasError bool
	}{
		{
			name:     "zero exponent",
			numeric:  pgtype.Numeric{Int: big.NewInt(12345), Exp: 0, Valid: true},
			expected: 12345,
		},
		{
			name:     "positive exponent",
			numeric:  pgtype.Numeric{Int: big.NewInt(123), Exp: 2, Valid: true},
			expected: 12300,
		},
		{
			name:     "negative exponent exact division",
			numeric:  pgtype.Numeric{Int: big.NewInt(12300), Exp: -2, Valid: true},
			expected: 123,
		},
		{
			name:     "negative exponent with remainder",
			numeric:  pgtype.Numeric{Int: big.NewInt(12345), Exp: -2, Valid: true},
			hasError: true,
		},
		{
			name:     "zero value",
			numeric:  pgtype.Numeric{Int: big.NewInt(0), Exp: 0, Valid: true},
			expected: 0,
		},
		{
			name:     "negative value",
			numeric:  pgtype.Numeric{Int: big.NewInt(-500), Exp: 1, Valid: true},
			expected: -5000,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result, err := NumericToBigInt(tc.numeric)
			if tc.hasError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.expected, result.Int64())
			}
		})
	}
}
