package encryption

import (
	sdkTypes "github.com/oasisprotocol/oasis-sdk/client-sdk/go/types"

	"github.com/oasisprotocol/nexus/common"
)

type EVMEncryptedData struct {
	Format      common.CallFormat
	PublicKey   []byte
	DataNonce   []byte
	DataData    []byte
	ResultNonce []byte
	ResultData  []byte
	// Unlike `ResultData` and `DataData`, this field is not encrypted data. Rather,
	// it is extracted during encrypted tx parsing and processed in extract.go.
	// If non-null, the tx is an older Sapphire tx and it failed, and `ResultNonce`
	// and `ResultData` will be empty.
	FailedCallResult *sdkTypes.FailedCallResult
}
