package file

import (
	"bytes"

	"github.com/fxamacker/cbor"
)

type NodeApiMethod func() (interface{}, error)

func generateCacheKey(methodName string, params ...interface{}) []byte {
	var buf bytes.Buffer
	enc := cbor.NewEncoder(&buf, cbor.CanonicalEncOptions())
	err := enc.Encode(methodName)
	if err != nil {
		panic(err)
	}
	for _, p := range params {
		err = enc.Encode(p)
		if err != nil {
			panic(err)
		}
	}
	return buf.Bytes()
}
