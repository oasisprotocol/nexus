// Package encryption defines the types for encryption envelopes.
package encryption

import (
	"github.com/oasisprotocol/nexus/common"
)

type EncryptedData struct {
	Format      common.CallFormat
	PublicKey   []byte
	DataNonce   []byte
	DataData    []byte
	ResultNonce []byte
	ResultData  []byte
}
