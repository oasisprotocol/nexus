package consensus

import (
	"fmt"

	staking "github.com/oasisprotocol/oasis-core/go/staking/api"
	sdkTypes "github.com/oasisprotocol/oasis-sdk/client-sdk/go/types"

	apiTypes "github.com/oasisprotocol/nexus/api/v1/types"
)

// nearly hungarian notation notes:
// ethAddr -> []byte len-20 slice
// ocAddr -> oasis-core type binary oasis address
// sdkAddr -> oasis-sdk type binary oasis address
// addr -> bech32 string oasis address
// addrTextBytes -> bech32 []byte oasis address

func stringifySDKAddress(sdkAddr *sdkTypes.Address) (apiTypes.Address, error) {
	addrTextBytes, err := sdkAddr.MarshalText()
	if err != nil {
		return "", fmt.Errorf("address marshal text: %w", err)
	}
	return apiTypes.Address(addrTextBytes), nil
}

func stringifyOCAddress(ocAddr staking.Address) (apiTypes.Address, error) {
	sdkAddr := (sdkTypes.Address)(ocAddr)
	return stringifySDKAddress(&sdkAddr)
}

func registerRelatedOCAddress(relatedAddresses map[apiTypes.Address]struct{}, ocAddr staking.Address) (apiTypes.Address, error) { //nolint:unparam // unused address result
	addr, err := stringifyOCAddress(ocAddr)
	if err != nil {
		return "", err
	}
	relatedAddresses[addr] = struct{}{}
	return addr, nil
}
