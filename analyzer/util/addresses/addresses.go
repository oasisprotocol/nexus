package addresses

import (
	"fmt"

	"github.com/oasisprotocol/oasis-core/go/common/crypto/address"
	sdkTypes "github.com/oasisprotocol/oasis-sdk/client-sdk/go/types"

	apiTypes "github.com/oasisprotocol/nexus/api/v1/types"
)

// nearly hungarian notation notes:
// ethAddr -> []byte len-20 slice
// ecAddr -> go-ethereum type binary address
// ocAddr -> oasis-core type binary oasis address
// sdkAddr -> oasis-sdk type binary oasis address
// addr -> bech32 string oasis address
// addrTextBytes -> bech32 []byte oasis address

func FromSdkAddress(sdkAddr *sdkTypes.Address) (apiTypes.Address, error) {
	addrTextBytes, err := sdkAddr.MarshalText()
	if err != nil {
		return "", fmt.Errorf("address marshal text: %w", err)
	}
	return apiTypes.Address(addrTextBytes), nil
}

func FromAddressSpec(as *sdkTypes.AddressSpec) (apiTypes.Address, error) {
	sdkAddr, err := as.Address()
	if err != nil {
		return "", fmt.Errorf("derive address: %w", err)
	}
	return FromSdkAddress(&sdkAddr)
}

func FromOCAddress(ocAddr address.Address) (apiTypes.Address, error) {
	sdkAddr := (sdkTypes.Address)(ocAddr)
	return FromSdkAddress(&sdkAddr)
}

func FromEthAddress(ethAddr []byte) (apiTypes.Address, error) {
	ctx := sdkTypes.AddressV0Secp256k1EthContext
	ocAddr := address.NewAddress(ctx, ethAddr)
	return FromOCAddress(ocAddr)
}

func SliceFromSet(accounts map[apiTypes.Address]struct{}) []string {
	addrs := make([]string, len(accounts))
	i := 0
	for a := range accounts {
		addrs[i] = a
		i++
	}
	return addrs
}
