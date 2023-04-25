package file

import (
	"bytes"
	"encoding/gob"
	"errors"

	"github.com/akrylysov/pogreb"
	"github.com/oasisprotocol/oasis-core/go/common/cbor"
	"github.com/oasisprotocol/oasis-indexer/log"
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

type KVStore struct{ *pogreb.DB }

func (s KVStore) Close() error {
	log.NewDefaultLogger("kvstore").Info("closing KVStore")
	return s.DB.Close()
}

// getFromCacheOrCall fetches the value of `cacheKey` from the cache if it exists,
// interpreted as a `Value`. If it does not exist, it calls `valueFunc` to get the
// value, and caches it before returning it.
// If `volatile` is true, `valueFunc` is always called, and the result is not cached.
func GetFromCacheOrCall[Value any](cache KVStore, volatile bool, cacheKey []byte, valueFunc func() (*Value, error)) (*Value, error) {
	// If the latest height was requested, the response is not cacheable, so we have to hit the backing API.
	if volatile {
		return valueFunc()
	}

	// If the value is cached, return it.
	isCached, err := cache.Has(cacheKey)
	if err != nil {
		return nil, err
	}
	if isCached {
		raw, err2 := cache.Get(cacheKey)
		if err2 != nil {
			return nil, err2
		}
		var result *Value
		err2 = cbor.Unmarshal(raw, &result)
		return result, err2
	}

	// Otherwise, the value is not cached. Call the backing API to get it.
	result, err := valueFunc()
	if err != nil {
		return nil, err
	}

	// Store value in cache for later use.
	return result, cache.Put(cacheKey, cbor.Marshal(result))
}

// Like getFromCacheOrCall, but for slice-typed return values.
func GetSliceFromCacheOrCall[Response any](cache KVStore, volatile bool, cacheKey []byte, valueFunc func() ([]Response, error)) ([]Response, error) {
	// Use `getFromCacheOrCall()` to avoid duplicating the cache update logic.
	responsePtr, err := GetFromCacheOrCall(cache, volatile, cacheKey, func() (*[]Response, error) {
		response, err := valueFunc()
		if response == nil {
			return nil, err
		}
		// Return the response wrapped in a pointer to conform to the signature of `getFromCacheOrCall()`.
		return &response, err
	})
	if responsePtr == nil {
		return nil, err
	}
	// Undo the pointer wrapping.
	return *responsePtr, err
}
