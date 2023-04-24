package common

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"
	sdkConfig "github.com/oasisprotocol/oasis-sdk/client-sdk/go/config"

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

// Network is an instance of the Oasis Network.
type Network uint8

const (
	// NetworkTestnet is the identifier for testnet.
	NetworkTestnet Network = iota
	// NetworkMainnet is the identifier for mainnet.
	NetworkMainnet
	// NetworkUnknown is the identifier for an unknown network.
	NetworkUnknown = 255
)

// FromChainContext identifies a Network using its ChainContext.
func FromChainContext(chainContext string) (Network, error) {
	// TODO: Remove this hardcoded value once indexer config supports multiple nodes.
	if chainContext == "53852332637bacb61b91b6411ab4095168ba02a50be4c3f82448438826f23898" {
		return NetworkMainnet, nil // cobalt mainnet
	}
	var network Network
	for name, nw := range sdkConfig.DefaultNetworks.All {
		if nw.ChainContext == chainContext {
			if err := network.Set(name); err != nil {
				return NetworkUnknown, err
			}
			return network, nil
		}
	}

	return NetworkUnknown, ErrNetworkUnknown
}

// Set sets the Network to the value specified by the provided string.
func (n *Network) Set(s string) error {
	switch strings.ToLower(s) {
	case "mainnet":
		*n = NetworkMainnet
	case "testnet":
		*n = NetworkTestnet
	default:
		return ErrNetworkUnknown
	}

	return nil
}

// String returns the string representation of a network.
func (n Network) String() string {
	switch n {
	case NetworkTestnet:
		return "testnet"
	case NetworkMainnet:
		return "mainnet"
	default:
		return "unknown"
	}
}

// Runtime is an identifier for a runtime on the Oasis Network.
type Runtime string

const (
	RuntimeEmerald  Runtime = "emerald"
	RuntimeCipher   Runtime = "cipher"
	RuntimeSapphire Runtime = "sapphire"
	RuntimeUnknown  Runtime = "unknown"
)

// String returns the string representation of a runtime.
func (r Runtime) String() string {
	return string(r)
}

// ID returns the ID for a Runtime on the provided network.
func (r Runtime) ID(n Network) (string, error) {
	for nname, nw := range sdkConfig.DefaultNetworks.All {
		if nname == n.String() {
			for pname, pt := range nw.ParaTimes.All {
				if pname == r.String() {
					return pt.ID, nil
				}
			}

			return "", ErrRuntimeUnknown
		}
	}

	return "", ErrRuntimeUnknown
}
