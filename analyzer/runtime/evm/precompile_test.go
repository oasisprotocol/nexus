package evm

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/oasisprotocol/oasis-core/go/common/cbor"
)

func TestPrecompileResult(t *testing.T) {
	testCases := []struct {
		name       string
		result     string
		statusCode uint32
		module     string
	}{
		// https://github.com/oasisprotocol/nexus/issues/1093
		{
			name:       "failed precompile",
			result:     "000000000000000000000000000000000000000000000000000000000000000a00000000000000000000000000000000000000000000000000000000000000400000000000000000000000000000000000000000000000000000000000000004636f726500000000000000000000000000000000000000000000000000000000",
			statusCode: 10,
			module:     "core",
		},
		{
			name:       "success precompile",
			result:     "000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000400000000000000000000000000000000000000000000000000000000000000001f600000000000000000000000000000000000000000000000000000000000000",
			statusCode: 0,
			module:     "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Prepare the precompile result.
			result, err := hex.DecodeString(tc.result)
			require.NoError(t, err)

			// Test unmarshalling.
			statusCode, module, err := EVMMaybeUnmarshalPrecompileResult(cbor.Marshal(result))
			require.NoError(t, err)
			require.Equal(t, tc.statusCode, statusCode)
			require.Equal(t, tc.module, module)
		})
	}
}
