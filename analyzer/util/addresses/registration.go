package addresses

import (
	"fmt"

	coreCommon "github.com/oasisprotocol/oasis-core/go/common"
	"github.com/oasisprotocol/oasis-core/go/common/cbor"
	"github.com/oasisprotocol/oasis-core/go/common/crypto/address"
	staking "github.com/oasisprotocol/oasis-core/go/staking/api"
	sdkTypes "github.com/oasisprotocol/oasis-sdk/client-sdk/go/types"

	"github.com/oasisprotocol/nexus/analyzer/util/eth"
	apiTypes "github.com/oasisprotocol/nexus/api/v1/types"
)

type PreimageData struct {
	ContextIdentifier string
	ContextVersion    int
	Data              []byte
}

func extractAddressPreimage(as *sdkTypes.AddressSpec) (*PreimageData, error) {
	// Adapted from oasis-sdk/client-sdk/go/types/transaction.go.
	var (
		ctx  address.Context
		data []byte
	)
	switch {
	case as.Signature != nil:
		spec := as.Signature
		switch {
		case spec.Ed25519 != nil:
			ctx = sdkTypes.AddressV0Ed25519Context
			data, _ = spec.Ed25519.MarshalBinary()
		case spec.Secp256k1Eth != nil:
			ctx = sdkTypes.AddressV0Secp256k1EthContext
			// Use a scheme such that we can compute Secp256k1 addresses from Ethereum
			// addresses as this makes things more interoperable.
			untaggedPk, _ := spec.Secp256k1Eth.MarshalBinaryUncompressedUntagged()
			data = eth.SliceEthAddress(eth.Keccak256(untaggedPk))
		case spec.Sr25519 != nil:
			ctx = sdkTypes.AddressV0Sr25519Context
			data, _ = spec.Sr25519.MarshalBinary()
		default:
			panic("address: unsupported public key type")
		}
	case as.Multisig != nil:
		config := as.Multisig
		ctx = sdkTypes.AddressV0MultisigContext
		data = cbor.Marshal(config)
	default:
		return nil, fmt.Errorf("malformed AddressSpec")
	}
	return &PreimageData{
		ContextIdentifier: ctx.Identifier,
		ContextVersion:    int(ctx.Version),
		Data:              data,
	}, nil
}

func RegisterAddressSpec(addressPreimages map[apiTypes.Address]*PreimageData, as *sdkTypes.AddressSpec) (apiTypes.Address, error) {
	addr, err := FromAddressSpec(as)
	if err != nil {
		return "", err
	}

	if _, ok := addressPreimages[addr]; !ok {
		preimageData, err1 := extractAddressPreimage(as)
		if err1 != nil {
			return "", fmt.Errorf("extract address preimage: %w", err1)
		}
		addressPreimages[addr] = preimageData
	}

	return addr, nil
}

func RegisterEthAddress(addressPreimages map[apiTypes.Address]*PreimageData, ethAddr []byte) (apiTypes.Address, error) {
	addr, err := FromEthAddress(ethAddr)
	if err != nil {
		return "", err
	}

	if _, ok := addressPreimages[addr]; !ok {
		addressPreimages[addr] = &PreimageData{
			ContextIdentifier: sdkTypes.AddressV0Secp256k1EthContext.Identifier,
			ContextVersion:    int(sdkTypes.AddressV0Secp256k1EthContext.Version),
			Data:              ethAddr,
		}
	}

	return addr, nil
}

func RegisterRuntimeAddress(addressPreimages map[apiTypes.Address]*PreimageData, id coreCommon.Namespace) (apiTypes.Address, error) {
	addr, err := FromRuntimeID(id)
	if err != nil {
		return "", err
	}

	if _, ok := addressPreimages[addr]; !ok {
		data, err1 := id.MarshalBinary()
		if err1 != nil {
			return "", err1
		}
		addressPreimages[addr] = &PreimageData{
			ContextIdentifier: staking.AddressRuntimeV0Context.Identifier,
			ContextVersion:    int(staking.AddressRuntimeV0Context.Version),
			Data:              data,
		}
	}

	return addr, nil
}
