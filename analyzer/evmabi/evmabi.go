package evmabi

import (
	_ "embed"
	"encoding/json"

	"github.com/ethereum/go-ethereum/accounts/abi"
)

func mustUnmarshalABI(artifactJSON []byte) *abi.ABI {
	var artifact struct {
		ABI *abi.ABI
	}
	if err := json.Unmarshal(artifactJSON, &artifact); err != nil {
		panic(err)
	}
	return artifact.ABI
}

//go:embed contracts/artifacts/ERC20.json
var artifactERC20JSON []byte
var ERC20 = mustUnmarshalABI(artifactERC20JSON)
