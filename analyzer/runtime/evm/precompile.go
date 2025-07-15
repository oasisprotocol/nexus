package evm

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/oasisprotocol/oasis-core/go/common/cbor"

	"github.com/oasisprotocol/nexus/analyzer/util/addresses"
	"github.com/oasisprotocol/nexus/api/v1/types"
)

var (
	// 0x0100000000000000000000000000000000000103
	subcallPrecompile = addresses.FromBech32("oasis1qzr543mmela3xwflqtz3e0l8jzp8tupf3v59r6qn")

	typeUint256, _ = abi.NewType("uint256", "", nil)
	typeBytes, _   = abi.NewType("bytes", "", nil)
)

// IsSubcallPrecompile checks if the address is the subcall precompile address.
func IsSubcallPrecompile(address types.Address) bool {
	return address == subcallPrecompile
}

// EVMMaybeUnmarshalPrecompileResult tries to unmarshal a precompile result.
func EVMMaybeUnmarshalPrecompileResult(result []byte) (uint32, string, error) {
	// Try parsing the output:
	// https://github.com/oasisprotocol/oasis-sdk/blob/deca9369166757b3075dc4414d7450340fdaa779/runtime-sdk/modules/evm/src/precompile/subcall.rs#L97-L106
	var output []byte
	if err := cbor.Unmarshal(result, &output); err != nil {
		return 0, "", fmt.Errorf("unmarshal precompile result: %w", err)
	}

	args := abi.Arguments{
		{Type: typeUint256, Name: "status_code"},
		{Type: typeBytes, Name: "module"},
	}
	out, err := args.Unpack(output)
	if err != nil {
		return 0, "", fmt.Errorf("unpack precompile result: %w", err)
	}
	if len(out) != 2 {
		return 0, "", fmt.Errorf("expected 2 arguments, got %d", len(out))
	}

	statusCode := out[0].(*big.Int).Uint64()
	if statusCode == 0 {
		return 0, "", nil
	}

	return uint32(statusCode), string(out[1].([]byte)), nil
}
