package config

import (
	apiTypes "github.com/oasisprotocol/nexus/api/v1/types"
	"github.com/oasisprotocol/nexus/common"
)

// ReferenceSwap identifies a factory contract and reference token in a
// runtime. These settings identify a set of swap contracts that allow Nexus
// to compare the relative value of tokens.
// Nexus considers swap pair contracts created by a specific factory because
// behave equivalently, and we can audit the factory and the pair contract
// template that it uses to ensure that it's fair.
// Nexus considers pairs that swap between the reference token and another
// token so that it can compare token values in terms of the reference token
// value directly.
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
