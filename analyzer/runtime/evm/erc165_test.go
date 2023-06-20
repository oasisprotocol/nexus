package evm

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestInterfaceID(t *testing.T) {
	// Reference interface identifiers are from code comments in ERC spec sample code.
	require.Equal(t, "01ffc9a7", hex.EncodeToString(ERC165InterfaceID))
	require.Equal(t, "80ac58cd", hex.EncodeToString(ERC721InterfaceID))
	require.Equal(t, "150b7a02", hex.EncodeToString(ERC721TokenReceiverInterfaceID))
	require.Equal(t, "5b5e139f", hex.EncodeToString(ERC721MetadataInterfaceID))
	require.Equal(t, "780e9d63", hex.EncodeToString(ERC721EnumerableInterfaceID))
}
