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
