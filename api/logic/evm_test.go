package logic

import (
	"encoding/hex"
	"math/big"
	"testing"

	ethCommon "github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"

	"github.com/oasisprotocol/nexus/analyzer/evmabi"
)

func TestEVMParseData(t *testing.T) {
	// https://explorer.emerald.oasis.dev/tx/0x1ac7521df4cda38c87cff56b1311ee9362168bd794230415a37f2aff3a554a5f/internal-transactions
	data, err := hex.DecodeString("095ea7b3000000000000000000000000250d48c5e78f1e85f7ab07fec61e93ba703ae6680000000000000000000000000000000000000000000000003782dace9d900000")
	require.NoError(t, err)
	method, args, err := EVMParseData(data, evmabi.ERC20)
	require.NoError(t, err)
	require.Equal(t, evmabi.ERC20.Methods["approve"], *method)
	require.Equal(t, []interface{}{
		ethCommon.HexToAddress("0x250d48c5e78f1e85f7ab07fec61e93ba703ae668"),
		big.NewInt(4000000000000000000),
	}, args)
}

func TestEVMParseEvent(t *testing.T) {
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
	event, args, err := EVMParseEvent(topics, data, evmabi.ERC20)
	require.NoError(t, err)
	require.Equal(t, evmabi.ERC20.Events["Approval"], *event)
	require.Equal(t, []interface{}{
		big.NewInt(4000000000000000000),
	}, args)
}
