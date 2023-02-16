package common

import (
	"fmt"

	ethCommon "github.com/ethereum/go-ethereum/common"

	"github.com/oasisprotocol/oasis-core/go/common/crypto/address"
	sdkTypes "github.com/oasisprotocol/oasis-sdk/client-sdk/go/types"

	apiTypes "github.com/oasisprotocol/oasis-indexer/api/v1/types"
)

// nearly hungarian notation notes:
// ethAddr -> []byte len-20 slice
// ecAddr -> go-ethereum type binary address
// ocAddr -> oasis-core type binary oasis address
// sdkAddr -> oasis-sdk type binary oasis address
// addr -> bech32 string oasis address
// addrTextBytes -> bech32 []byte oasis address

func StringifySdkAddress(sdkAddr *sdkTypes.Address) (apiTypes.Address, error) {
	addrTextBytes, err := sdkAddr.MarshalText()
	if err != nil {
		return "", fmt.Errorf("address marshal text: %w", err)
	}
	return apiTypes.Address(addrTextBytes), nil
}

func StringifyAddressSpec(as *sdkTypes.AddressSpec) (apiTypes.Address, error) {
	sdkAddr, err := as.Address()
	if err != nil {
		return "", fmt.Errorf("derive address: %w", err)
	}
	return StringifySdkAddress(&sdkAddr)
}

func StringifyOcAddress(ocAddr address.Address) (apiTypes.Address, error) {
	sdkAddr := (sdkTypes.Address)(ocAddr)
	return StringifySdkAddress(&sdkAddr)
}

func StringifyEthAddress(ethAddr []byte) (apiTypes.Address, error) {
	ctx := sdkTypes.AddressV0Secp256k1EthContext
	ocAddr := address.NewAddress(ctx, ethAddr)
	return StringifyOcAddress(ocAddr)
}

func EthAddrReference(eth_addr_hex string) *string {
	if len(eth_addr_hex) == 0 {
		return nil
	}

	ref := ethCommon.HexToAddress(eth_addr_hex).String()
	return &ref
}

func ExtractAddresses(accounts map[apiTypes.Address]bool) []string {
	addrs := make([]string, len(accounts))
	i := 0
	for a := range accounts {
		addrs[i] = string(a)
		i++
	}
	return addrs
}
