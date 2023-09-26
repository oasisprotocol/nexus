package runtime

import (
	"testing"

	"github.com/stretchr/testify/require"
)

type mockError struct {
	module  string
	code    uint32
	message string
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
