package runtime

import (
	"encoding/base64"
	"testing"

	"github.com/oasisprotocol/oasis-core/go/common/cbor"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/types"
	"github.com/stretchr/testify/require"

	"github.com/oasisprotocol/nexus/common"
	"github.com/oasisprotocol/nexus/log"
	"github.com/oasisprotocol/nexus/storage/oasis/nodeapi"
)

type mockError struct {
	module  string
	code    uint32
	message string
}

// Mocks the BlockTransactionData struct, but replaces many []byte
// field types with strings to reduce boilerplate.
type mockTxData struct {
	Raw          string
	RawResult    string
	Method       string
	EvmEncrypted *mockEvmEncrypted
	Success      *bool
	Error        *TxError
}

type mockEvmEncrypted struct {
	Format      common.CallFormat
	PublicKey   string
	DataNonce   string
	DataData    string
	ResultNonce string
	ResultData  string
}

func TestPlaintextError(t *testing.T) {
	inputs := []mockError{
		{"core", 0, "core: invalid call format: Tag verification failed"},
		{"evm", 0, "execution failed: out of fund"},
		{"evm", 8, "reverted: Game not exist"},
		{"evm", 8, `reverted: invalid reason prefix: 'EAlgywAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAM'`},
	}
	for _, input := range inputs {
		msg := tryParseErrorMessage(input.module, input.code, input.message)
		require.NotNil(t, msg, "failed to extract error message")
		require.Equal(t, input.message, *msg, "parsed message should match original input message")
	}
}

func TestKnownStringError(t *testing.T) {
	inputs := map[string]mockError{
		"reverted: too late to submit": {module: "evm", code: 8, message: "reverted: CMN5oAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAgAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAABJ0b28gbGF0ZSB0byBzdWJtaXQAAAAAAAAAAAAAAAAAAA=="},
	}
	for parsed, input := range inputs {
		msg := tryParseErrorMessage(input.module, input.code, input.message)
		require.NotNil(t, msg, "failed to extract error message")
		require.Equal(t, parsed, *msg, "parsed unexpected message")
	}
}

func TestUnknownError(t *testing.T) {
	inputs := []mockError{
		{module: "evm", code: 8, message: "reverted: b+19hQ=="},                                         // TooSoon()
		{module: "evm", code: 8, message: "reverted: EYzapwAAAAAAAAAAAAAAAH3y4TP8/2EqpGmV6CpUM19xa2a+"}, // OwnableUnauthorizedAccount(address)
	}
	for _, input := range inputs {
		msg := tryParseErrorMessage(input.module, input.code, input.message)
		require.Nil(t, msg, "tryParseErrorMessage extracted a message when it should not have")
	}
}

func TestEmptyError(t *testing.T) {
	input := &mockError{module: "evm", code: 8, message: "reverted: "}
	msg := tryParseErrorMessage(input.module, input.code, input.message)
	require.Nil(t, msg, "tryParseErrorMessage extracted a message when it should not have")
}

func TestExtractSuccessfulEncryptedTx(t *testing.T) {
	txBody, _ := base64.StdEncoding.DecodeString("+QE3gjc+hRdIdugAgw9CQJQR1RPa4nTJ35R1o7lWypg7lLsNwIC4zaJkYm9keaNicGtYIIn0UYPeSB9UEuOdJ0HJltloztQGqFScIJMYUkKfmatxZGRhdGFYfO8udYN5tJJ50o5td4lgzWdwuZid8rWYXeO+SmY8RwDivlwELbaNRDges0gQUbfLEgPrA74C+X/eqsKLSlBJAKFsApLZsfkTfKViwucwwclHnIpoWCYaNSroY0eiIHjoK0FBhh6MOHSU0ov2OVsvlDPqSwCmbyys6bk7DD5lbm9uY2VPl0SS+WcwWj+n5rKn+wzvZmZvcm1hdAGCtiKgQ+aU2gbOviF0Oq4JO+ywOVm7L6fxisKTXmcpx24vX1+gDi0QM72w2tYqHYPqt6VcGWmvK5GyXm79xIEAVrLKPGU=")
	txResult, _ := base64.StdEncoding.DecodeString("WDahYm9romRkYXRhVfd11jNj4IPuD//0n3CIDG7/Lf2T22Vub25jZU8AAAAAADJSEQAAAAAAAAA=")
	txResultCbor := cbor.RawMessage(txResult)
	// round: 3297810, index: 0
	txrs := []nodeapi.RuntimeTransactionWithResults{
		{
			Tx: types.UnverifiedTransaction{
				Body: txBody,
				AuthProofs: []types.AuthProof{
					{
						Module: "evm.ethereum.v0",
					},
				},
			},
			Result: types.CallResult{
				Ok: txResultCbor,
			},
		},
	}
	expected := mockTxData{
		Raw:       "glkBOvkBN4I3PoUXSHboAIMPQkCUEdUT2uJ0yd+UdaO5VsqYO5S7DcCAuM2iZGJvZHmjYnBrWCCJ9FGD3kgfVBLjnSdByZbZaM7UBqhUnCCTGFJCn5mrcWRkYXRhWHzvLnWDebSSedKObXeJYM1ncLmYnfK1mF3jvkpmPEcA4r5cBC22jUQ4HrNIEFG3yxID6wO+Avl/3qrCi0pQSQChbAKS2bH5E3ylYsLnMMHJR5yKaFgmGjUq6GNHoiB46CtBQYYejDh0lNKL9jlbL5Qz6ksApm8srOm5Oww+ZW5vbmNlT5dEkvlnMFo/p+ayp/sM72Zmb3JtYXQBgrYioEPmlNoGzr4hdDquCTvssDlZuy+n8YrCk15nKcduL19foA4tEDO9sNrWKh2D6relXBlpryuRsl5u/cSBAFayyjxlgaFmbW9kdWxlb2V2bS5ldGhlcmV1bS52MA==",
		RawResult: "oWJva1g2oWJva6JkZGF0YVX3ddYzY+CD7g//9J9wiAxu/y39k9tlbm9uY2VPAAAAAAAyUhEAAAAAAAAA",
		Method:    "evm.Call",
		EvmEncrypted: &mockEvmEncrypted{
			Format:      "encrypted/x25519-deoxysii",
			PublicKey:   "ifRRg95IH1QS450nQcmW2WjO1AaoVJwgkxhSQp+Zq3E=",
			DataNonce:   "l0SS+WcwWj+n5rKn+wzv",
			DataData:    "7y51g3m0knnSjm13iWDNZ3C5mJ3ytZhd475KZjxHAOK+XAQtto1EOB6zSBBRt8sSA+sDvgL5f96qwotKUEkAoWwCktmx+RN8pWLC5zDByUecimhYJho1KuhjR6IgeOgrQUGGHow4dJTSi/Y5Wy+UM+pLAKZvLKzpuTsMPg==",
			ResultNonce: "AAAAAAAyUhEAAAAAAAAA",
			ResultData:  "93XWM2Pgg+4P//SfcIgMbv8t/ZPb",
		},
		Success: common.Ptr(true),
		Error:   nil,
	}
	blockData, err := ExtractRound(nodeapi.RuntimeBlockHeader{}, txrs, []nodeapi.RuntimeEvent{}, log.NewDefaultLogger("testing"))
	require.NoError(t, err)

	verifyTxData(t, &expected, blockData.TransactionData[0])
}

func TestExtractFailedEncryptedTx(t *testing.T) {
	txBody, _ := base64.StdEncoding.DecodeString("+QE4gwL1bIUXSHboAIMPQkCU2h48CsdPLxC7DHY1ydxovT2gyVuAuM2iZGJvZHmjYnBrWCDUnh5gkwUk6Cw4huLOP3lTMFtCmN8n4ltIpAlVZ94NZ2RkYXRhWHyVr0ki6C9hWguv2FPHI046DSIKbXpE5K4ls2RFbPy9M6M5HzuqJBbuA74qCub93GK1QV/kHqcsxBZP2MVRgGlfj81krakaipNzwho+Xe/ETHPigxbBCtO/gA92dewZfYnZ6WrFSZ4MUpNOh0RI+mGbZkrpuyJogMk7B4ehZW5vbmNlT2U966XgO5N7jx2V90nP42Zmb3JtYXQBgrYhoCbdbtYSkhEAaGzp4EOa6vymDawzpqGUAcRKUpCItUoPoGFF2WiD2hvI89jTHHP6lCvJVMBdA0+vdSAO29YnCMeq")
	// round: 3297812, index: 0
	txrs := []nodeapi.RuntimeTransactionWithResults{
		{
			Tx: types.UnverifiedTransaction{
				Body: txBody,
				AuthProofs: []types.AuthProof{
					{
						Module: "evm.ethereum.v0",
					},
				},
			},
			Result: types.CallResult{
				Failed: &types.FailedCallResult{
					Module:  "evm",
					Code:    8,
					Message: "reverted: CMN5oAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAgAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAABJ0b28gbGF0ZSB0byBzdWJtaXQAAAAAAAAAAAAAAAAAAA==",
				},
			},
		},
	}
	expected := mockTxData{
		Raw:       "glkBO/kBOIMC9WyFF0h26ACDD0JAlNoePArHTy8Quwx2NcncaL09oMlbgLjNomRib2R5o2Jwa1gg1J4eYJMFJOgsOIbizj95UzBbQpjfJ+JbSKQJVWfeDWdkZGF0YVh8la9JIugvYVoLr9hTxyNOOg0iCm16ROSuJbNkRWz8vTOjOR87qiQW7gO+Kgrm/dxitUFf5B6nLMQWT9jFUYBpX4/NZK2pGoqTc8IaPl3vxExz4oMWwQrTv4APdnXsGX2J2elqxUmeDFKTTodESPphm2ZK6bsiaIDJOweHoWVub25jZU9lPeul4DuTe48dlfdJz+NmZm9ybWF0AYK2IaAm3W7WEpIRAGhs6eBDmur8pg2sM6ahlAHESlKQiLVKD6BhRdlog9obyPPY0xxz+pQryVTAXQNPr3UgDtvWJwjHqoGhZm1vZHVsZW9ldm0uZXRoZXJldW0udjA=",
		RawResult: "oWRmYWlso2Rjb2RlCGZtb2R1bGVjZXZtZ21lc3NhZ2V4knJldmVydGVkOiBDTU41b0FBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQWdBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUJKMGIyOGdiR0YwWlNCMGJ5QnpkV0p0YVhRQUFBQUFBQUFBQUFBQUFBQUFBQT09",
		Method:    "evm.Call",
		EvmEncrypted: &mockEvmEncrypted{
			Format:      "encrypted/x25519-deoxysii",
			PublicKey:   "1J4eYJMFJOgsOIbizj95UzBbQpjfJ+JbSKQJVWfeDWc=",
			DataNonce:   "ZT3rpeA7k3uPHZX3Sc/j",
			DataData:    "la9JIugvYVoLr9hTxyNOOg0iCm16ROSuJbNkRWz8vTOjOR87qiQW7gO+Kgrm/dxitUFf5B6nLMQWT9jFUYBpX4/NZK2pGoqTc8IaPl3vxExz4oMWwQrTv4APdnXsGX2J2elqxUmeDFKTTodESPphm2ZK6bsiaIDJOweHoQ==",
			ResultNonce: "",
			ResultData:  "",
		},
		Success: common.Ptr(false),
		Error: &TxError{
			Code:       8,
			Module:     "evm",
			RawMessage: common.Ptr("reverted: CMN5oAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAgAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAABJ0b28gbGF0ZSB0byBzdWJtaXQAAAAAAAAAAAAAAAAAAA=="),
			Message:    common.Ptr("reverted: too late to submit"),
		},
	}
	blockData, err := ExtractRound(nodeapi.RuntimeBlockHeader{}, txrs, []nodeapi.RuntimeEvent{}, log.NewDefaultLogger("testing"))
	require.NoError(t, err)

	verifyTxData(t, &expected, blockData.TransactionData[0])
}

func TestExtractSuccessfulUnecryptedTx(t *testing.T) {
	txBody, _ := base64.StdEncoding.DecodeString("+HGDEpOvhRdIdugAgnUwlO2jlWZuVt2eLvO9x27uNztzhkDdiAFjRXjCzycUgIK2IqDZ9hicLCKMvkYPfcnxp7uKn+9ayRFBIZlSKutUaXuM/KBAnuTyW6fGXEVAEPOiMdEYPzyQDkW4EjFUMVVhpNf9/Q==")
	txResult, _ := base64.StdEncoding.DecodeString("QA==")
	// round: 3297812, index: 1
	txrs := []nodeapi.RuntimeTransactionWithResults{
		{
			Tx: types.UnverifiedTransaction{
				Body: txBody,
				AuthProofs: []types.AuthProof{
					{
						Module: "evm.ethereum.v0",
					},
				},
			},
			Result: types.CallResult{
				Ok: cbor.RawMessage(txResult),
			},
		},
	}
	expected := mockTxData{
		Raw:          "glhz+HGDEpOvhRdIdugAgnUwlO2jlWZuVt2eLvO9x27uNztzhkDdiAFjRXjCzycUgIK2IqDZ9hicLCKMvkYPfcnxp7uKn+9ayRFBIZlSKutUaXuM/KBAnuTyW6fGXEVAEPOiMdEYPzyQDkW4EjFUMVVhpNf9/YGhZm1vZHVsZW9ldm0uZXRoZXJldW0udjA=",
		RawResult:    "oWJva0A=",
		Method:       "evm.Call",
		EvmEncrypted: nil,
		Success:      common.Ptr(true),
		Error:        nil,
	}
	blockData, err := ExtractRound(nodeapi.RuntimeBlockHeader{}, txrs, []nodeapi.RuntimeEvent{}, log.NewDefaultLogger("testing"))
	require.NoError(t, err)

	verifyTxData(t, &expected, blockData.TransactionData[0])
}

func TestExtractFailedUnecryptedTx(t *testing.T) {
	txBody, _ := base64.StdEncoding.DecodeString("+QEuggdghRdIdugAgw9CQJSm2rSCCcCmlseYXunsRiANpUt3A4C4xKKU2XUAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAACAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAZTaGFyb24AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAANUGhpbG1hX0tyYWtlbgAAAAAAAAAAAAAAAAAAAAAAAACCtiGgFPX+w6su01SbkC+02CPG+dt9p94lDkuyFrCb46AfnyWgXBA3aAX4QyXQZqO4DW+/Owk1kL0g/Sir5ftcBkz83BU=")
	// round: 3297811, index: 0
	txrs := []nodeapi.RuntimeTransactionWithResults{
		{
			Tx: types.UnverifiedTransaction{
				Body: txBody,
				AuthProofs: []types.AuthProof{
					{
						Module: "evm.ethereum.v0",
					},
				},
			},
			Result: types.CallResult{
				Failed: &types.FailedCallResult{
					Module:  "evm",
					Code:    2,
					Message: "execution failed: out of gas",
				},
			},
		},
	}
	expected := mockTxData{
		Raw:          "glkBMfkBLoIHYIUXSHboAIMPQkCUptq0ggnAppbHmF7p7EYgDaVLdwOAuMSilNl1AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAgAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAGU2hhcm9uAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAADVBoaWxtYV9LcmFrZW4AAAAAAAAAAAAAAAAAAAAAAAAAgrYhoBT1/sOrLtNUm5AvtNgjxvnbfafeJQ5Lshawm+OgH58loFwQN2gF+EMl0GajuA1vvzsJNZC9IP0oq+X7XAZM/NwVgaFmbW9kdWxlb2V2bS5ldGhlcmV1bS52MA==",
		RawResult:    "oWRmYWlso2Rjb2RlAmZtb2R1bGVjZXZtZ21lc3NhZ2V4HGV4ZWN1dGlvbiBmYWlsZWQ6IG91dCBvZiBnYXM=",
		Method:       "evm.Call",
		EvmEncrypted: nil,
		Success:      common.Ptr(false),
		Error: &TxError{
			Code:       2,
			Module:     "evm",
			RawMessage: common.Ptr("execution failed: out of gas"),
			Message:    common.Ptr("execution failed: out of gas"),
		},
	}
	blockData, err := ExtractRound(nodeapi.RuntimeBlockHeader{}, txrs, []nodeapi.RuntimeEvent{}, log.NewDefaultLogger("testing"))
	require.NoError(t, err)

	verifyTxData(t, &expected, blockData.TransactionData[0])
}

func verifyTxData(t *testing.T, expected *mockTxData, actual *BlockTransactionData) {
	require.Equal(t, expected.Raw, base64.StdEncoding.EncodeToString(actual.Raw))
	require.Equal(t, expected.RawResult, base64.StdEncoding.EncodeToString(actual.RawResult))
	require.Equal(t, expected.Method, actual.Method)
	require.Equal(t, *expected.Success, *actual.Success)
	if expected.EvmEncrypted != nil {
		require.NotNil(t, actual.EVMEncrypted)
		require.Equal(t, expected.EvmEncrypted.Format, actual.EVMEncrypted.Format)
		require.Equal(t, expected.EvmEncrypted.PublicKey, base64.StdEncoding.EncodeToString(actual.EVMEncrypted.PublicKey))
		require.Equal(t, expected.EvmEncrypted.DataNonce, base64.StdEncoding.EncodeToString(actual.EVMEncrypted.DataNonce))
		require.Equal(t, expected.EvmEncrypted.DataData, base64.StdEncoding.EncodeToString(actual.EVMEncrypted.DataData))
		require.Equal(t, expected.EvmEncrypted.ResultNonce, base64.StdEncoding.EncodeToString(actual.EVMEncrypted.ResultNonce))
		require.Equal(t, expected.EvmEncrypted.ResultData, base64.StdEncoding.EncodeToString(actual.EVMEncrypted.ResultData))
	} else {
		require.Nil(t, actual.EVMEncrypted)
	}
	if expected.Error != nil {
		require.NotNil(t, actual.Error)
		require.Equal(t, expected.Error.Code, actual.Error.Code)
		require.Equal(t, expected.Error.Module, actual.Error.Module)
		if expected.Error.RawMessage != nil {
			require.NotNil(t, actual.Error.RawMessage)
			require.Equal(t, *expected.Error.RawMessage, *actual.Error.RawMessage)
		}
		if expected.Error.Message != nil {
			require.NotNil(t, actual.Error.Message)
			require.Equal(t, *expected.Error.Message, *actual.Error.Message)
		}
	} else {
		require.Nil(t, actual.Error)
	}
}
