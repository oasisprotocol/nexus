package abiparse

import (
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"math/big"
	"testing"

	ethCommon "github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"

	"github.com/oasisprotocol/nexus/analyzer/evmabi"
)

//go:embed test_contracts/artifacts/Varied.json
var artifactVariedJSON []byte
var Varied = evmabi.MustUnmarshalABI(artifactVariedJSON)

//go:embed test_contracts/artifacts/CustomErrors.json
var artifactCustomErrorsJSON []byte
var CustomErrors = evmabi.MustUnmarshalABI(artifactCustomErrorsJSON)

func TestParseTypes(t *testing.T) {
	// -1,1,-1,1,true,0x0101010101010101010101010101010101010101010101010101010101010101,0x0101010101010101010101010101010101010101,0x010101010101010101010101010101010101010102020202,[1,1],[1,1],0x01,"a",[1],[1],[1,"a"]
	data, err := hex.DecodeString("b23a194fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff0000000000000000000000000000000000000000000000000000000000000001ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff0000000000000000000000000000000000000000000000000000000000000001000000000000000000000000000000000000000000000000000000000000000101010101010101010101010101010101010101010101010101010101010101010000000000000000000000000101010101010101010101010101010101010101010101010101010101010101010101010101010102020202000000000000000000000000000000000000000000000000000000000000000000000000000000010000000000000000000000000000000000000000000000000000000000000001000000000000000000000000000000000000000000000000000000000000000100000000000000000000000000000000000000000000000000000000000000010000000000000000000000000000000000000000000000000000000000000220000000000000000000000000000000000000000000000000000000000000026000000000000000000000000000000000000000000000000000000000000002a000000000000000000000000000000000000000000000000000000000000002e00000000000000000000000000000000000000000000000000000000000000320000000000000000000000000000000000000000000000000000000000000000101000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000001610000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000010000000000000000000000000000000000000000000000000000000000000001000000000000000000000000000000000000000000000000000000000000000100000000000000000000000000000000000000000000000000000000000000010000000000000000000000000000000000000000000000000000000000000001000000000000000000000000000000000000000000000000000000000000004000000000000000000000000000000000000000000000000000000000000000016100000000000000000000000000000000000000000000000000000000000000")
	require.NoError(t, err)
	method, args, err := ParseData(data, Varied)
	require.NoError(t, err)
	require.Equal(t, Varied.Methods["test"], *method)
	jsonExpected := []string{
		"-1",     // int8
		"1",      // uint8
		"\"-1\"", // int256
		"\"1\"",  // uint256
		"true",   // bool
		"\"0x0101010101010101010101010101010101010101010101010101010101010101\"", // bytes32
		"\"0x0101010101010101010101010101010101010101\"",                         // address
		"\"0x010101010101010101010101010101010101010102020202\"",                 // function (uint16) external returns (uint16)
		"[1,1]",                 // uint16[2]
		"[1,1]",                 // uint8[2]
		"\"0x01\"",              // bytes
		"\"a\"",                 // string
		"[1]",                   // uint16[]
		"[1]",                   // uint8[]
		"{\"n\":1,\"s\":\"a\"}", // O
	}
	for i, input := range method.Inputs {
		transducedArg := EvmPreMarshal(args[i], input.Type)
		jsonBytesArg, err1 := json.Marshal(transducedArg)
		require.NoError(t, err1)
		require.Equal(t, jsonExpected[i], string(jsonBytesArg))
	}
}

func TestParseTxUnnamed(t *testing.T) {
	data, err := hex.DecodeString("ee799b1800000000000000000000000000000000000000000000000000000000000000010000000000000000000000000000000000000000000000000000000000000002")
	require.NoError(t, err)
	result, err := hex.DecodeString("00000000000000000000000000000000000000000000000000000000000000030000000000000000000000000000000000000000000000000000000000000004")
	require.NoError(t, err)
	method, args, err := ParseData(data, Varied)
	require.NoError(t, err)
	require.Equal(t, Varied.Methods["testUnnamed"], *method)
	require.Equal(t, []interface{}{
		uint16(1),
		uint16(2),
	}, args)
	outArgs, err := ParseResult(result, method)
	require.NoError(t, err)
	require.Equal(t, []interface{}{
		uint16(3),
		uint16(4),
	}, outArgs)
}

func TestParseEventUnnamed(t *testing.T) {
	topicsHex := []string{
		"54789e7ea9ad7e0cf2c8e336a0e24206777d784a11ec566b33ae6402ee7c2518",
		"0000000000000000000000000000000000000000000000000000000000000001",
		"0000000000000000000000000000000000000000000000000000000000000002",
	}
	topics := make([][]byte, 0, len(topicsHex))
	for _, topicHex := range topicsHex {
		topic, err := hex.DecodeString(topicHex)
		require.NoError(t, err)
		topics = append(topics, topic)
	}
	data, err := hex.DecodeString("00000000000000000000000000000000000000000000000000000000000000030000000000000000000000000000000000000000000000000000000000000004")
	require.NoError(t, err)
	event, args, err := ParseEvent(topics, data, Varied)
	require.NoError(t, err)
	require.Equal(t, Varied.Events["TestUnnamed"], *event)
	require.Equal(t, []interface{}{
		uint16(1),
		uint16(2),
		uint16(3),
		uint16(4),
	}, args)
}

func TestParseTx(t *testing.T) {
	// https://explorer.emerald.oasis.dev/tx/0x1ac7521df4cda38c87cff56b1311ee9362168bd794230415a37f2aff3a554a5f/internal-transactions
	data, err := hex.DecodeString("095ea7b3000000000000000000000000250d48c5e78f1e85f7ab07fec61e93ba703ae6680000000000000000000000000000000000000000000000003782dace9d900000")
	require.NoError(t, err)
	// oasis.misc.toHex(oasis.misc.fromCBOR(await networks.mainnet.nic.runtimeClientGetTransactionsWithResults({runtime_id: networks.mainnet.runtimes.emerald.id, round: 5709967})[1].result).ok)
	result, err := hex.DecodeString("0000000000000000000000000000000000000000000000000000000000000001")
	require.NoError(t, err)
	method, inArgs, err := ParseData(data, evmabi.ERC20)
	require.NoError(t, err)
	require.Equal(t, evmabi.ERC20.Methods["approve"], *method)
	require.Equal(t, []interface{}{
		ethCommon.HexToAddress("0x250d48c5e78f1e85f7ab07fec61e93ba703ae668"),
		big.NewInt(4000000000000000000),
	}, inArgs)
	outArgs, err := ParseResult(result, method)
	require.NoError(t, err)
	require.Equal(t, []interface{}{
		true,
	}, outArgs)
}

func TestParseEvent(t *testing.T) {
	// https://explorer.emerald.oasis.dev/tx/0x1ac7521df4cda38c87cff56b1311ee9362168bd794230415a37f2aff3a554a5f/logs
	topicsHex := []string{
		"8c5be1e5ebec7d5bd14f71427d1e84f3dd0314c0f7b2291e5b200ac8c7c3b925",
		"0000000000000000000000000ecf5262e5b864e1612875f8fc18f151315b5e91",
		"000000000000000000000000250d48c5e78f1e85f7ab07fec61e93ba703ae668",
	}
	topics := make([][]byte, 0, len(topicsHex))
	for _, topicHex := range topicsHex {
		topic, err := hex.DecodeString(topicHex)
		require.NoError(t, err)
		topics = append(topics, topic)
	}
	data, err := hex.DecodeString("0000000000000000000000000000000000000000000000003782dace9d900000")
	require.NoError(t, err)
	event, args, err := ParseEvent(topics, data, evmabi.ERC20)
	require.NoError(t, err)
	require.Equal(t, evmabi.ERC20.Events["Approval"], *event)
	require.Equal(t, []interface{}{
		ethCommon.HexToAddress("0ecf5262e5b864e1612875f8fc18f151315b5e91"),
		ethCommon.HexToAddress("250d48c5e78f1e85f7ab07fec61e93ba703ae668"),
		big.NewInt(4000000000000000000),
	}, args)
}

func TestParseError(t *testing.T) {
	revertBytes, err := hex.DecodeString("41a0efd30000000000000000000000000000000000000000000000000000000000000001000000000000000000000000000000000000000000000000000000000000004000000000000000000000000000000000000000000000000000000000000000016100000000000000000000000000000000000000000000000000000000000000")
	require.NoError(t, err)
	abiError, args, err := ParseError(revertBytes, CustomErrors)
	require.NoError(t, err)
	require.Equal(t, CustomErrors.Errors["E"], *abiError)
	require.Equal(t, []interface{}{
		uint16(1),
		"a",
	}, args)
}
