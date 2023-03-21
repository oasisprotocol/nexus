package common

import (
	"fmt"
	"math/big"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/oasisprotocol/oasis-core/go/common/quantity"
)

// Arbitrary-precision integer. Wrapper around big.Int to allow for
// custom JSON marshaling.
type BigInt struct {
	big.Int
}

func NewBigInt(v int64) BigInt {
	return BigInt{*big.NewInt(v)}
}

func (b BigInt) MarshalText() ([]byte, error) {
	return []byte(b.String()), nil
}

func (b *BigInt) UnmarshalText(text []byte) error {
	return b.Int.UnmarshalText(text)
}

func (b BigInt) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf(`"%s"`, b.String())), nil
}

func (b *BigInt) UnmarshalJSON(text []byte) error {
	v := strings.Trim(string(text), "\"")
	return b.Int.UnmarshalJSON([]byte(v))
}

func BigIntFromQuantity(q quantity.Quantity) BigInt {
	return BigInt{*q.ToBigInt()}
}

// Implement NumericValuer interface for BigInt.
func (b BigInt) NumericValue() (pgtype.Numeric, error) {
	return pgtype.Numeric{Int: &b.Int, Exp: 0, NaN: false, Valid: true, InfinityModifier: pgtype.Finite}, nil
}

// Implement NumericDecoder interface for BigInt.
func (b *BigInt) ScanNumeric(n pgtype.Numeric) error {
	if !n.Valid {
		return fmt.Errorf("NULL values can't be decoded. Scan into a **BigInt to handle NULLs")
	}
	bigInt, err := NumericToBigInt(n)
	*b = bigInt
	return err
}

// NumericToBigInt converts a pgtype.Numeric to a BigInt similar to the
// private method found at https://github.com/jackc/pgtype/blob/master/numeric.go#L398
func NumericToBigInt(n pgtype.Numeric) (BigInt, error) {
	if n.Exp == 0 {
		return BigInt{Int: *n.Int}, nil
	}

	big0 := big.NewInt(0)
	big10 := big.NewInt(10)
	bi := &big.Int{}
	bi.Set(n.Int)
	if n.Exp > 0 {
		mul := &big.Int{}
		mul.Exp(big10, big.NewInt(int64(n.Exp)), nil)
		bi.Mul(bi, mul)
		return BigInt{Int: *bi}, nil
	}

	div := &big.Int{}
	div.Exp(big10, big.NewInt(int64(-n.Exp)), nil)
	remainder := &big.Int{}
	bi.DivMod(bi, div, remainder)
	if remainder.Cmp(big0) != 0 {
		return BigInt{Int: *big0}, fmt.Errorf("cannot convert %v to integer", n)
	}
	return BigInt{Int: *big0}, nil
}

// Key used to set values in a web request context. API uses this to set
// values, backend uses this to retrieve values.
type ContextKey string

const (
	// ChainIDContextKey is used to set the relevant chain ID
	// in a request context.
	ChainIDContextKey ContextKey = "chain_id"
	// RuntimeContextKey is used to set the relevant runtime name
	// in a request context.
	RuntimeContextKey ContextKey = "runtime"
	// RequestIDContextKey is used to set a request id for tracing
	// in a request context.
	RequestIDContextKey ContextKey = "request_id"
)
