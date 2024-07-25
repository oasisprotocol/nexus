package config

import (
	apiTypes "github.com/oasisprotocol/nexus/api/v1/types"
	"github.com/oasisprotocol/nexus/common"
)

type ReferenceSwap struct {
	FactoryAddr        apiTypes.Address `koanf:"factory_addr"`
	ReferenceTokenAddr apiTypes.Address `koanf:"reference_token_addr"`
}

var DefaultReferenceSwaps = map[common.ChainName]map[common.Runtime]ReferenceSwap{
	common.ChainNameMainnet: {
		common.RuntimeSapphire: {
			// illumineX factory
			// https://docs.illuminex.xyz/illuminex-docs/misc/contracts#oasis-sapphire-mainnet
			FactoryAddr: "oasis1qqtj2t79keq4wtq5qd3vnv2uh0hds2s9zvgyltk6",
			// Wrapped ROSE
			// https://docs.oasis.io/dapp/sapphire/addresses/
			ReferenceTokenAddr: "oasis1qpdgv5nv2dhxp4q897cgag6kgnm9qs0dccwnckuu",
		},
	},
}
