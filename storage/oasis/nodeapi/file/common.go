package file

import (
	"bytes"
	"encoding/gob"
)

type NodeApiMethod func() (interface{}, error)

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
