package evmabi

import (
	_ "embed"
	"encoding/json"

	"github.com/ethereum/go-ethereum/accounts/abi"
)

//go:embed contracts/artifacts/ERC20.json
var artifactERC20JSON []byte
var ERC20 *abi.ABI

func init() {
	type artifact struct {
		ABI *abi.ABI
	}
	var artifactERC20 artifact
	if err := json.Unmarshal(artifactERC20JSON, &artifactERC20); err != nil {
		panic(err)
	}
	ERC20 = artifactERC20.ABI
}
