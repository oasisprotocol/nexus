package common

import (
	"fmt"
	"math/big"
	"strings"
)

// Arbitrary-precision integer. Wrapper around big.Int to allow for
// custom JSON marshaling.
type BigInt struct {
	big.Int
}

func NewBigInt(v int64) BigInt {
	return BigInt{*big.NewInt(v)}
}

func (b *BigInt) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf(`"%s"`, b.String())), nil
}

func (b *BigInt) UnmarshalJSON(text []byte) error {
	v := strings.Trim(string(text), "\"")
	return b.Int.UnmarshalJSON([]byte(v))
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
