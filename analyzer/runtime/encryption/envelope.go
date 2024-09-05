package encryption

import (
	"github.com/oasisprotocol/nexus/common"
)

type EVMEncryptedData struct {
	Format      common.CallFormat
	PublicKey   []byte
	DataNonce   []byte
	DataData    []byte
	ResultNonce []byte
	ResultData  []byte
}
