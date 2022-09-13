package common

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"
	"testing"

	"github.com/oasisprotocol/oasis-core/go/common/cbor"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/helpers"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/modules/evm"
	sdkTypes "github.com/oasisprotocol/oasis-sdk/client-sdk/go/types"
	"github.com/stretchr/testify/require"
)

// TestDecodeEthRawTx replicates tests from oasis-sdk/runtime-sdk/modules/evm/src/raw_tx.rs
func TestDecodeEthRawTx(t *testing.T) {
	chainIdOne := int64(1)
	chainIdFive := int64(5)
	// decode expect call
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
			"f86b8085e8d4a510008227109413978aee95f38490e9769c39b2773ed763d9cd5f872386f26fc10000801ba0eab47c1a49bf2fe5d40e01d313900e19ca485867d462fe06e139e3a536c6d4f4a014a569d327dcda4b29f74f93c0e9729d2f49ad726e703f9cd90dbb0fbf6649f1",
			nil,
			"13978aee95f38490e9769c39b2773ed763d9cd5f",
			10_000_000_000_000_000,
			"",
			10_000,
			1_000_000_000_000,
			// "cow" test account
			"cd2a3d9f938e13cd947ec05abc7fe734df8dd826",
			0,
		},
		// https://github.com/ethereum/tests/blob/v10.0/BlockchainTests/ValidBlocks/bcEIP1559/transType.json
		{
			"f861018203e882c35094cccccccccccccccccccccccccccccccccccccccc80801ca021539ef96c70ab75350c594afb494458e211c8c722a7a0ffb7025c03b87ad584a01d5395fe48edb306f614f0cd682b8c2537537f5fd3e3275243c42e9deff8e93d",
			nil,
			"cccccccccccccccccccccccccccccccccccccccc",
			0,
			"",
			50_000,
			1_000,
			"d02d72e067e77158444ef2020ff2d325f929b363",
			1,
		},
		{
			"01f86301028203e882c35094cccccccccccccccccccccccccccccccccccccccc8080c080a0260f95e555a1282ef49912ff849b2007f023c44529dc8fb7ecca7693cccb64caa06252cf8af2a49f4cb76fd7172feaece05124edec02db242886b36963a30c2606",
			&chainIdOne,
			"cccccccccccccccccccccccccccccccccccccccc",
			0,
			"",
			50_000,
			1_000,
			"d02d72e067e77158444ef2020ff2d325f929b363",
			2,
		},
		{"02f8640103648203e882c35094cccccccccccccccccccccccccccccccccccccccc8080c001a08480e6848952a15ae06192b8051d213d689bdccdf8f14cf69f61725e44e5e80aa057c2af627175a2ac812dab661146dfc7b9886e885c257ad9c9175c3fcec2202e",
			&chainIdOne,
			"cccccccccccccccccccccccccccccccccccccccc",
			0,
			"",
			50_000,
			100,
			"d02d72e067e77158444ef2020ff2d325f929b363",
			3,
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
		from0xChecksummed := helpers.EthAddressFromPubKey(*tx.AuthInfo.SignerInfo[0].AddressSpec.Signature.Secp256k1Eth)
		fromChecksummed := from0xChecksummed[2:]
		from := strings.ToLower(fromChecksummed)
		require.Equal(t, tt.expectedFrom, from)
		require.Equal(t, sdkTypes.NativeDenomination, tx.AuthInfo.Fee.Amount.Denomination)
		require.Equal(t, tt.expectedGasLimit, tx.AuthInfo.Fee.Gas)
	}
	// decode expect create
	for _, tt := range []struct {
		raw              string
		expectedChainId  *int64
		expectedValue    uint64
		expectedInitCode string
		expectedGasLimit uint64
		expectedGasPrice uint64
		expectedFrom     string
		expectedNonce    uint64
	}{
		{
			// We're using a transaction normalized from the original (below) to have low `s`.
			// f87f8085e8d4a510008227108080af6025515b525b600a37f260003556601b596020356000355760015b525b54602052f260255860005b525b54602052f21ba05afed0244d0da90b67cf8979b0f246432a5112c0d31e8d5eedd2bc17b171c694a0bb1035c834677c2e1185b8dc90ca6d1fa585ab3d7ef23707e1a497a98e752d1b
			"f87f8085e8d4a510008227108080af6025515b525b600a37f260003556601b596020356000355760015b525b54602052f260255860005b525b54602052f21ca05afed0244d0da90b67cf8979b0f246432a5112c0d31e8d5eedd2bc17b171c694a044efca37cb9883d1ee7a47236f3592df152931a930566933de2dc6e341c11426",
			nil,
			0,
			"6025515b525b600a37f260003556601b596020356000355760015b525b54602052f260255860005b525b54602052f2",
			10_000,
			1_000_000_000_000,
			// "horse" test account
			"13978aee95f38490e9769c39b2773ed763d9cd5f",
			0,
		},
	} {
		raw, err := hex.DecodeString(tt.raw)
		require.NoError(t, err)
		tx, err := decodeEthRawTx(raw, tt.expectedChainId)
		require.NoError(t, err)
		fmt.Printf("%#v\n", tx) // %%%
		require.Equal(t, tx.Call.Method, "evm.Create")
		var body evm.Create
		require.NoError(t, cbor.Unmarshal(tx.Call.Body, &body))
		var bodyValueBI big.Int
		bodyValueBI.SetBytes(body.Value)
		require.True(t, bodyValueBI.IsUint64())
		require.Equal(t, tt.expectedValue, bodyValueBI.Uint64())
		expectedData, err := hex.DecodeString(tt.expectedInitCode)
		require.NoError(t, err)
		require.Equal(t, expectedData, body.InitCode)
		require.Len(t, tx.AuthInfo.SignerInfo, 1)
		from0xChecksummed := helpers.EthAddressFromPubKey(*tx.AuthInfo.SignerInfo[0].AddressSpec.Signature.Secp256k1Eth)
		fromChecksummed := from0xChecksummed[2:]
		from := strings.ToLower(fromChecksummed)
		require.Equal(t, tt.expectedFrom, from)
		require.Equal(t, sdkTypes.NativeDenomination, tx.AuthInfo.Fee.Amount.Denomination)
		require.Equal(t, tt.expectedGasLimit, tx.AuthInfo.Fee.Gas)
	}
	// decode expect invalid
	for _, tt := range []struct {
		raw             string
		expectedChainId *int64
	}{
		// Test with mismatching expect_chain_id to exercise our check.
		{
			// Taken from test_decode_basic.
			"f86b8085e8d4a510008227109413978aee95f38490e9769c39b2773ed763d9cd5f872386f26fc10000801ba0eab47c1a49bf2fe5d40e01d313900e19ca485867d462fe06e139e3a536c6d4f4a014a569d327dcda4b29f74f93c0e9729d2f49ad726e703f9cd90dbb0fbf6649f1",
			&chainIdFive,
		},
		// Altered signature, out of bounds r = n.
		{
			"f86b8085e8d4a510008227109413978aee95f38490e9769c39b2773ed763d9cd5f872386f26fc10000801ba0fffffffffffffffffffffffffffffffebaaedce6af48a03bbfd25e8cd0364141a014a569d327dcda4b29f74f93c0e9729d2f49ad726e703f9cd90dbb0fbf6649f1",
			nil,
		},
		// Altered signature, high s.
		{
			"f86b8085e8d4a510008227109413978aee95f38490e9769c39b2773ed763d9cd5f872386f26fc10000801ca0eab47c1a49bf2fe5d40e01d313900e19ca485867d462fe06e139e3a536c6d4f4a0eb5a962cd82325b4d608b06c3f168d618b652f7440d8609ee6c4a37d10cff750",
			nil,
		},
	} {
		raw, err := hex.DecodeString(tt.raw)
		require.NoError(t, err)
		_, err = decodeEthRawTx(raw, tt.expectedChainId)
		require.Error(t, err)
		fmt.Printf("%#v\n", err) // %%%
	}
	// decode expect from mismatch
	for _, tt := range []struct {
		raw             string
		expectedChainId *int64
		unexpectedFrom  string
	}{
		// Altered signature, s decreased by one.
		{
			"f86b8085e8d4a510008227109413978aee95f38490e9769c39b2773ed763d9cd5f872386f26fc10000801ba0eab47c1a49bf2fe5d40e01d313900e19ca485867d462fe06e139e3a536c6d4f4a014a569d327dcda4b29f74f93c0e9729d2f49ad726e703f9cd90dbb0fbf6649f0",
			nil,
			"cd2a3d9f938e13cd947ec05abc7fe734df8dd826",
		},
	} {
		raw, err := hex.DecodeString(tt.raw)
		require.NoError(t, err)
		tx, err := decodeEthRawTx(raw, tt.expectedChainId)
		require.NoError(t, err)
		fmt.Printf("%#v\n", tx) // %%%
		require.Len(t, tx.AuthInfo.SignerInfo, 1)
		from0xChecksummed := helpers.EthAddressFromPubKey(*tx.AuthInfo.SignerInfo[0].AddressSpec.Signature.Secp256k1Eth)
		fromChecksummed := from0xChecksummed[2:]
		from := strings.ToLower(fromChecksummed)
		require.NotEqual(t, tt.unexpectedFrom, from)
	}
}
