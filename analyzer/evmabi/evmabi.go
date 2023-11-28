package evmabi

import (
	_ "embed"
	"encoding/json"

	"github.com/ethereum/go-ethereum/accounts/abi"
)

func MustUnmarshalABI(artifactJSON []byte) *abi.ABI {
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
var ERC20 = MustUnmarshalABI(artifactERC20JSON)

//go:embed contracts/artifacts/ERC165.json
var artifactERC165JSON []byte
var ERC165 = MustUnmarshalABI(artifactERC165JSON)

//go:embed contracts/artifacts/ERC721.json
var artifactERC721JSON []byte
var ERC721 = MustUnmarshalABI(artifactERC721JSON)

//go:embed contracts/artifacts/ERC721TokenReceiver.json
var artifactERC721TokenReceiverJSON []byte
var ERC721TokenReceiver = MustUnmarshalABI(artifactERC721TokenReceiverJSON)

//go:embed contracts/artifacts/ERC721Metadata.json
var artifactERC721MetadataJSON []byte
var ERC721Metadata = MustUnmarshalABI(artifactERC721MetadataJSON)

//go:embed contracts/artifacts/ERC721Enumerable.json
var artifactERC721EnumerableJSON []byte
var ERC721Enumerable = MustUnmarshalABI(artifactERC721EnumerableJSON)

//go:embed contracts/artifacts/WROSE.json
var artifactWROSEJSON []byte
var WROSE = MustUnmarshalABI(artifactWROSEJSON)
