package evmabibackfill

import (
	"encoding/base64"
	"encoding/json"
	"testing"

	ethCommon "github.com/ethereum/go-ethereum/common"

	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/modules/evm"
	"github.com/stretchr/testify/require"

	"github.com/oasisprotocol/nexus/analyzer/evmabi"
	"github.com/oasisprotocol/nexus/common"
	"github.com/oasisprotocol/nexus/log"
)

// TODO: Update WROSE source and add tests for more txs, mismatched txs, and custom errors

type mockParsedEvent struct {
	Name *string
	Args []*abiEncodedArg
	Sig  *ethCommon.Hash
}

type mockParsedTxCall struct {
	Name *string
	Args []*abiEncodedArg
}

func unmarshalEvmEvent(t *testing.T, body []byte) *abiEncodedEvent {
	var ev evm.Event
	err := json.Unmarshal(body, &ev)
	require.Nil(t, err)
	return &abiEncodedEvent{
		Round:     0,
		TxIndex:   nil,
		EventBody: ev,
	}
}

func unmarshalEvmTx(t *testing.T, body string, error_message *string) *abiEncodedTx {
	txData, err := base64.StdEncoding.DecodeString(body)
	require.Nil(t, err)
	return &abiEncodedTx{
		TxHash:         "", // not relevant for current unit tests
		TxData:         txData,
		TxRevertReason: error_message,
	}
}

func TestParseEvent(t *testing.T) {
	abi := evmabi.WROSE
	p := &processor{
		runtime: common.RuntimeSapphire,
		target:  nil,
		logger:  log.NewDefaultLogger("testing"),
	}
	ev := unmarshalEvmEvent(t, []byte(`{"data": "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAADCopscuTr4A=", "topics": ["3fJSrRviyJtpwrBo/DeNqpUrp/FjxKEWKPVaTfUjs+8=", "AAAAAAAAAAAAAAAAw3+DQaxuSpRTgwK81NSc8IUtMMA=", "AAAAAAAAAAAAAAAAWZiaj7/4vj0d6v/ytGtmtjrLeX4="], "address": "i8KwMLKZlk7vteHgs2mRNS5W0tM="}`))
	expected := mockParsedEvent{
		Name: common.Ptr("Transfer"),
		Sig:  common.Ptr(ethCommon.HexToHash("0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef")),
		Args: []*abiEncodedArg{
			{
				Name:    "src",
				EvmType: "address",
				Value:   ethCommon.HexToAddress("0xc37F8341Ac6e4a94538302bCd4d49Cf0852D30C0"),
			},
			{
				Name:    "dst",
				EvmType: "address",
				Value:   ethCommon.HexToAddress("0x59989A8fBff8be3d1deAFFF2b46B66B63ACB797e"),
			},
			{
				Name:    "wad",
				EvmType: "uint256",
				Value:   "876558921078386560",
			},
		},
	}
	name, args, sig, err := p.parseEvent(ev, *abi)
	require.Nil(t, err)
	verifyEvent(t, &expected, name, args, sig)
}

func TestParseUnknownEvent(t *testing.T) {
	abi := evmabi.WROSE
	p := &processor{
		runtime: common.RuntimeSapphire,
		target:  nil,
		logger:  log.NewDefaultLogger("testing"),
	}
	// Non-WROSE event; it doesn't fit the `abi` and should fail to parse.
	rawEv := []byte(`{"data": "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=", "topics": ["8nnmofXjIMypETVnbZy25EyooIwLiDQrzbEUT2URtWg=", "AAAAAAAAAAAAAAAArk2RKpUohmFUB3UShFG6FIKuqKc=", "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAI="], "address": "jZzJ7hGq+GWRPe7JOe6y3Hg4q3s="}`)
	ev := unmarshalEvmEvent(t, rawEv)

	name, args, sig, err := p.parseEvent(ev, *abi)
	require.NotNil(t, err)
	require.Nil(t, name)
	require.Nil(t, args)
	require.Nil(t, sig)
}

func TestParseTransactionCall(t *testing.T) {
	abi := evmabi.WROSE
	p := &processor{
		runtime: common.RuntimeSapphire,
		target:  nil,
		logger:  log.NewDefaultLogger("testing"),
	}
	txs := []*abiEncodedTx{
		unmarshalEvmTx(t, "Lhp9TQAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAKcV46p6RigAA", nil),
		unmarshalEvmTx(t, "qQWcuwAAAAAAAAAAAAAAAOY8J3oxo9tm0HXYxapOlEKOuxjxAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=", nil),
		unmarshalEvmTx(t, "0OMNsA==", nil),
	}

	expected := []mockParsedTxCall{
		{
			Name: common.Ptr("withdraw"),
			Args: []*abiEncodedArg{
				{
					Name:    "wad",
					EvmType: "uint256",
					Value:   "770545888011536171008",
				},
			},
		},
		{
			Name: common.Ptr("transfer"),
			Args: []*abiEncodedArg{
				{
					Name:    "dst",
					EvmType: "address",
					Value:   ethCommon.HexToAddress("0xE63c277a31A3dB66D075D8C5aA4E94428Ebb18F1"),
				},
				{
					Name:    "wad",
					EvmType: "uint256",
					Value:   "0",
				},
			},
		},
		{
			Name: common.Ptr("deposit"),
			Args: []*abiEncodedArg{},
		},
	}
	for i, tx := range txs {
		name, args, err := p.parseTxCall(tx, *abi)
		require.Nil(t, err)
		verifyTxCall(t, &expected[i], name, args)
	}
}

func TestParseTxErrorPlaintext(t *testing.T) {
	abi := evmabi.WROSE
	p := &processor{
		runtime: common.RuntimeSapphire,
		target:  nil,
		logger:  log.NewDefaultLogger("testing"),
	}
	errMsg := "reverted: plaintext error message"
	emptyErrMsg := "reverted: "
	txs := []*abiEncodedTx{
		unmarshalEvmTx(t, "", &errMsg),
		unmarshalEvmTx(t, "", &emptyErrMsg),
	}

	for _, tx := range txs {
		parsedMsg, args, err := p.parseTxErr(tx, *abi)
		require.Nil(t, err)
		require.Nil(t, parsedMsg)
		require.Nil(t, args)
	}
}

func verifyEvent(t *testing.T, expected *mockParsedEvent, name *string, args []*abiEncodedArg, sig *ethCommon.Hash) {
	if expected.Name != nil {
		require.NotNil(t, name)
		require.Equal(t, *expected.Name, *name)
	} else {
		require.Nil(t, name)
	}
	if expected.Args != nil {
		require.NotNil(t, args)
		require.Equal(t, len(expected.Args), len(args))
		for i, ea := range expected.Args {
			require.Equal(t, ea.Name, args[i].Name)
			require.Equal(t, ea.EvmType, args[i].EvmType)
			require.Equal(t, ea.Value, args[i].Value)
		}
	} else {
		require.Nil(t, args)
	}
	if expected.Sig != nil {
		require.NotNil(t, sig)
		require.Equal(t, *expected.Sig, *sig)
	} else {
		require.Nil(t, sig)
	}
}

func verifyTxCall(t *testing.T, expected *mockParsedTxCall, name *string, args []*abiEncodedArg) {
	if expected.Name != nil {
		require.NotNil(t, name)
		require.Equal(t, *expected.Name, *name)
	} else {
		require.Nil(t, name)
	}
	if expected.Args != nil {
		require.NotNil(t, args)
		require.Equal(t, len(expected.Args), len(args))
		for i, ea := range expected.Args {
			require.Equal(t, ea.Name, args[i].Name)
			require.Equal(t, ea.EvmType, args[i].EvmType)
			require.Equal(t, ea.Value, args[i].Value)
		}
	} else {
		require.Nil(t, args)
	}
}
