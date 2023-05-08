package file

import (
	"errors"
	"fmt"

	"github.com/akrylysov/pogreb"
	"github.com/oasisprotocol/oasis-core/go/common/cbor"
	"github.com/oasisprotocol/oasis-indexer/log"
)

type NodeApiMethod func() (interface{}, error)

var ErrUnstableRPCMethod = errors.New("this method is not cacheable because the RPC return value is not constant")

// A key in the KVStore.
type CacheKey []byte

func generateCacheKey(methodName string, params ...interface{}) CacheKey {
	return CacheKey(cbor.Marshal([]interface{}{methodName, params}))
}

type KVStore struct {
	*pogreb.DB

	path   string
	logger *log.Logger
}

func (s KVStore) Close() error {
	s.logger.Info("closing KVStore", "path", s.path)
	return s.DB.Close()
}

func OpenKVStore(logger *log.Logger, path string) (*KVStore, error) {
	logger.Info("(re)opening KVStore", "path", path)
	db, err := pogreb.Open(path, &pogreb.Options{BackgroundSyncInterval: -1})
	if err != nil {
		return nil, err
	}
	logger.Info(fmt.Sprintf("KVStore has %d entries", db.Count()))

	return &KVStore{DB: db, logger: logger, path: path}, nil
}

// Pretty() returns a pretty-printed, human-readable version of the cache key.
// It tries to interpret it as CBOR and returns the pretty-printed struct, otherwise
// it returns the key's raw bytes as hex.
func (cacheKey CacheKey) Pretty() string {
	var pretty string
	var parsed interface{}
	err := cbor.Unmarshal(cacheKey, &parsed)
	if err != nil {
		pretty = fmt.Sprintf("%+v", parsed)
	} else {
		pretty = fmt.Sprintf("%x", cacheKey)
	}
	if len(pretty) > 100 {
		pretty = pretty[:95] + "[...]"
	}
	return pretty
}

var errNoSuchKey = fmt.Errorf("no such key")

// fetchTypedValue fetches the value of `cacheKey` from the cache, interpreted as a `Value`.
func fetchTypedValue[Value any](cache KVStore, key CacheKey, value *Value) error {
	isCached, err := cache.Has(key)
	if err != nil {
		return err
	}
	if !isCached {
		return errNoSuchKey
	}
	raw, err := cache.Get(key)
	if err != nil {
		return fmt.Errorf("failed to fetch key %s from cache: %v", key.Pretty(), err)
	}
	err = cbor.Unmarshal(raw, value)
	if err != nil {
		return fmt.Errorf("failed to unmarshal key %s from cache into %T: %v", key.Pretty(), value, err)
	}

	return nil
}

// getFromCacheOrCall fetches the value of `cacheKey` from the cache if it exists,
// interpreted as a `Value`. If it does not exist, it calls `valueFunc` to get the
// value, and caches it before returning it.
// If `volatile` is true, `valueFunc` is always called, and the result is not cached.
func GetFromCacheOrCall[Value any](cache KVStore, volatile bool, key CacheKey, valueFunc func() (*Value, error)) (*Value, error) {
	// If the latest height was requested, the response is not cacheable, so we have to hit the backing API.
	if volatile {
		return valueFunc()
	}

	// If the value is cached, return it.
	var cached Value
	switch err := fetchTypedValue(cache, key, &cached); err {
	case nil:
		return &cached, nil
	case errNoSuchKey: // Regular cache miss; continue below.
	default:
		// Log unexpected error and continue to call the backing API.
		cache.logger.Warn(fmt.Sprintf("fetch from cache: %v", err))
	}

	// The value is not cached or couldn't be restored from the cache. Call the backing API to get it.
	computed, err := valueFunc()
	if err != nil {
		return nil, err
	}

	// Store value in cache for later use.
	return computed, cache.Put(key, cbor.Marshal(computed))
}

// Like getFromCacheOrCall, but for slice-typed return values.
func GetSliceFromCacheOrCall[Response any](cache KVStore, volatile bool, key CacheKey, valueFunc func() ([]Response, error)) ([]Response, error) {
	// Use `getFromCacheOrCall()` to avoid duplicating the cache update logic.
	responsePtr, err := GetFromCacheOrCall(cache, volatile, key, func() (*[]Response, error) {
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
