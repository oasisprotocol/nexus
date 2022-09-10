package common

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"testing"

	"github.com/oasisprotocol/oasis-core/go/common/cbor"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/modules/evm"
	sdkTypes "github.com/oasisprotocol/oasis-sdk/client-sdk/go/types"
	"github.com/stretchr/testify/require"
)

func Test_decodeEthRawTx(t *testing.T) {
	for _, tt := range []struct {
		raw              string
		expectedChainId  *int64
		expectedTo       string
		expectedValue    uint64
		expectedData     string
		expectedGasLimit uint64
		expectedGasPrice uint64
		expectedFrom     string
		expectedNonce    uint64
	}{
		{
			raw:              "f86b8085e8d4a510008227109413978aee95f38490e9769c39b2773ed763d9cd5f872386f26fc10000801ba0eab47c1a49bf2fe5d40e01d313900e19ca485867d462fe06e139e3a536c6d4f4a014a569d327dcda4b29f74f93c0e9729d2f49ad726e703f9cd90dbb0fbf6649f1",
			expectedChainId:  nil,
			expectedTo:       "13978aee95f38490e9769c39b2773ed763d9cd5f",
			expectedValue:    10_000_000_000_000_000,
			expectedData:     "",
			expectedGasLimit: 10_000,
			expectedGasPrice: 1_000_000_000_000,
			// "cow" test account
			expectedFrom:  "cd2a3d9f938e13cd947ec05abc7fe734df8dd826",
			expectedNonce: 0,
		},
	} {
		raw, err := hex.DecodeString(tt.raw)
		require.NoError(t, err)
		tx, err := decodeEthRawTx(raw, tt.expectedChainId)
		require.NoError(t, err)
		fmt.Printf("%#v\n", tx) // %%%
		require.Equal(t, tx.Call.Method, "evm.Call")
		var body evm.Call
		require.NoError(t, cbor.Unmarshal(tx.Call.Body, &body))
		expectedTo, err := hex.DecodeString(tt.expectedTo)
		require.NoError(t, err)
		require.Equal(t, expectedTo, body.Address)
		var bodyValueBI big.Int
		bodyValueBI.SetBytes(body.Value)
		require.True(t, bodyValueBI.IsUint64())
		require.Equal(t, tt.expectedValue, bodyValueBI.Uint64())
		expectedData, err := hex.DecodeString(tt.expectedData)
		require.NoError(t, err)
		require.Equal(t, expectedData, body.Data)
		require.Len(t, tx.AuthInfo.SignerInfo, 1)
		expectedFrom, err := hex.DecodeString(tt.expectedFrom)
		require.NoError(t, err)
		// todo: add the right derivation logic here
		require.Equal(t, expectedFrom, tx.AuthInfo.SignerInfo[0].AddressSpec.Signature.Secp256k1Eth)
		require.Equal(t, sdkTypes.NativeDenomination, tx.AuthInfo.Fee.Amount.Denomination)
		require.Equal(t, tt.expectedGasLimit, tx.AuthInfo.Fee.Gas)
	}
}
