package common

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/big"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/oasisprotocol/oasis-core/go/common/quantity"
	staking "github.com/oasisprotocol/oasis-core/go/staking/api"
	sdkTypes "github.com/oasisprotocol/oasis-sdk/client-sdk/go/types"
)

// Arbitrary-precision integer. Wrapper around big.Int to allow for
// custom JSON marshaling.
type BigInt struct {
	big.Int
}

func NewBigInt(v int64) BigInt {
	return BigInt{*big.NewInt(v)}
}

func (b BigInt) Plus(other BigInt) BigInt {
	result := big.Int{}
	result.Set(&b.Int) // creates a copy of b.Int
	result.Add(&result, &other.Int)
	return BigInt{result}
}

func (b BigInt) Minus(other BigInt) BigInt {
	result := big.Int{}
	result.Set(&b.Int) // creates a copy of b.Int
	result.Sub(&result, &other.Int)
	return BigInt{result}
}

func (b BigInt) Times(other BigInt) BigInt {
	result := big.Int{}
	result.Set(&b.Int) // creates a copy of b.Int
	result.Mul(&result, &other.Int)
	return BigInt{result}
}

func (b BigInt) Div(other BigInt) BigInt {
	result := big.Int{}
	result.Set(&b.Int) // creates a copy of b.Int
	result.Div(&result, &other.Int)
	return BigInt{result}
}

func (b BigInt) IsZero() bool {
	return b.Sign() == 0
}

func (b BigInt) Eq(other BigInt) bool {
	return b.Minus(other).IsZero()
}

func (b BigInt) MarshalText() ([]byte, error) {
	return b.Int.MarshalText()
}

func (b *BigInt) UnmarshalText(text []byte) error {
	return b.Int.UnmarshalText(text)
}

// Used by the `cbor` library.
func (b BigInt) MarshalBinary() ([]byte, error) {
	var sign byte = 1
	if b.Int.Sign() == -1 {
		sign = 255
	}
	return append([]byte{sign}, b.Bytes()...), nil // big.Int only supports serializing the abs value
}

func (b *BigInt) UnmarshalBinary(data []byte) error {
	if len(data) == 0 {
		return errors.New("empty data")
	}
	var inner big.Int
	inner.SetBytes(data[1:])
	switch data[0] {
	case 1:
		// do nothing
	case 255:
		inner.Neg(&inner)
	default:
		return fmt.Errorf("invalid sign byte: %d", data[0])
	}
	b.Int = inner
	return nil
}

func (b BigInt) MarshalJSON() ([]byte, error) {
	t, err := b.MarshalText()
	if err != nil {
		return nil, err
	}
	return json.Marshal(string(t))
}

func (b *BigInt) UnmarshalJSON(text []byte) error {
	var s string
	err := json.Unmarshal(text, &s)
	if err != nil {
		return err
	}
	return b.UnmarshalText([]byte(s))
}

func (b BigInt) String() string {
	// *big.Int does have a String() method. But the way the Go language
	// works, that method on a pointer receiver doesn't get included in
	// non-pointer BigInt's method set. In some places this doesn't matter,
	// because *big.Int's methods are included in pointer *BigInt's method
	// set, and a completely different part of the language set says that
	// writing b.String() is fine; it's shorthand for (&b).String(). But
	// reflection-driven code like fmt.Printf only looks at method sets and
	// not shorthand trickery, so we need this method to make
	// fmt.Printf("%v\n", b) show a number instead of dumping the internal
	// bytes.
	return b.Int.String()
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

func Ptr[T any](v T) *T {
	return &v
}

// Returns `v` as a JSON string. If `v` cannot be marshaled,
// returns the string "null" instead.
func TryAsJSON(v interface{}) json.RawMessage {
	encoded, err := json.Marshal(v)
	if err != nil {
		return json.RawMessage("null")
	}
	return json.RawMessage(encoded)
}

func StringOrNil[T fmt.Stringer](v *T) *string {
	if v == nil {
		return nil
	}
	return Ptr((*v).String())
}

// Key used to set values in a web request context. API uses this to set
// values, backend uses this to retrieve values.
type ContextKey string

const (
	// RuntimeContextKey is used to set the relevant runtime name
	// in a request context.
	RuntimeContextKey ContextKey = "runtime"
	// RequestIDContextKey is used to set a request id for tracing
	// in a request context.
	RequestIDContextKey ContextKey = "request_id"
)

var (
	// ErrNetworkUnknown is returned if a chain context does not correspond
	// to a known network identifier.
	ErrNetworkUnknown = errors.New("network unknown")

	// ErrRuntimeUnknown is returned if a chain context does not correspond
	// to a known runtime identifier.
	ErrRuntimeUnknown = errors.New("runtime unknown")
)

// ChainName is a name given to a sequence of networks. Used to index
// config.DefaultChains and sdkConfig.DefaultNetworks.All.
type ChainName string

const (
	// ChainNameTestnet is the identifier for testnet.
	ChainNameTestnet ChainName = "testnet"
	// ChainNameMainnet is the identifier for mainnet.
	ChainNameMainnet ChainName = "mainnet"
	// ChainNameUnknown is the identifier for an unknown network.
	ChainNameUnknown ChainName = "unknown"
)

// Layer is an identifier for either Consensus or a network runtime.
type Layer string

const (
	LayerConsensus   Layer = "consensus"
	LayerEmerald     Layer = "emerald"
	LayerCipher      Layer = "cipher"
	LayerSapphire    Layer = "sapphire"
	LayerPontusxTest Layer = "pontusx_test"
	LayerPontusxDev  Layer = "pontusx_dev"
)

// Runtime is an identifier for a runtime on the Oasis Network.
type Runtime string

const (
	RuntimeEmerald     Runtime = "emerald"
	RuntimeCipher      Runtime = "cipher"
	RuntimeSapphire    Runtime = "sapphire"
	RuntimePontusxTest Runtime = "pontusx_test"
	RuntimePontusxDev  Runtime = "pontusx_dev"
)

type CallFormat string

func NativeBalance(balances map[sdkTypes.Denomination]BigInt) BigInt {
	nativeBalance, ok := balances[sdkTypes.NativeDenomination]
	if !ok {
		// This is normal for accounts that have had no balance activity;
		// the node returns an empty map.
		return NewBigInt(0)
	}
	return nativeBalance
}

// Address is the consensus layer Oasis address.
type Address = staking.Address

// ConsensusPoolAddress is the address of the consensus consensus pool.
var ConsensusPoolAddress = staking.CommonPoolAddress
