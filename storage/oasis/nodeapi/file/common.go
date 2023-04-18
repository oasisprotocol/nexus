package file

import (
	"bytes"
	"encoding/gob"
	"errors"
)

type NodeApiMethod func() (interface{}, error)

var ErrUnstableRPCMethod = errors.New("this method is not cacheable because the RPC return value is not constant")

func generateCacheKey(methodName string, params ...interface{}) []byte {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
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
