package file

import (
	"fmt"
	"sync/atomic"
	"time"

	"github.com/akrylysov/pogreb"
	"github.com/oasisprotocol/oasis-core/go/common/cbor"

	"github.com/oasisprotocol/nexus/log"
)

// A key in the KVStore.
type CacheKey []byte

func generateCacheKey(methodName string, params ...interface{}) CacheKey {
	return CacheKey(cbor.Marshal([]interface{}{methodName, params}))
}

// A key-value store. Additional method-like functions that give a typed interface
// to the store (i.e. with typed values/keys instead of []byte) are provided below,
// taking KVStore as the first argument so they can use generics.
type KVStore interface {
	Has(key []byte) (bool, error)
	Get(key []byte) ([]byte, error)
	Put(key []byte, value []byte) error
	Close() error
}

type pogrebKVStore struct {
	db *pogreb.DB

	path   string
	logger *log.Logger

	// Address of the atomic variable that indicates whether the store is initialized.
	// Synchronisation is required because the store is opened in background goroutine.
	initialized uint32
}

var _ KVStore = (*pogrebKVStore)(nil)

func (s *pogrebKVStore) isInitialized() bool {
	return atomic.LoadUint32(&s.initialized) == 1
}

// Get implements KVStore.
func (s *pogrebKVStore) Get(key []byte) ([]byte, error) {
	if b := s.isInitialized(); !b {
		return nil, fmt.Errorf("kvstore: not initialized yet")
	}
	return s.db.Get(key)
}

// Has implements KVStore.
func (s *pogrebKVStore) Has(key []byte) (bool, error) {
	if b := s.isInitialized(); !b {
		return false, nil
	}
	return s.db.Has(key)
}

// Put implements KVStore.
func (s *pogrebKVStore) Put(key []byte, value []byte) error {
	if b := s.isInitialized(); !b {
		// If the store is not initialized yet, skip writing to it.
		s.logger.Debug("skipping write to uninitialized KVStore", "key", key)
		return nil
	}
	return s.db.Put(key, value)
}

// Close implements KVStore.
func (s pogrebKVStore) Close() error {
	s.logger.Info("closing KVStore", "path", s.path)
	return s.db.Close()
}

func (s *pogrebKVStore) init() error {
	s.logger.Info("(re)opening KVStore", "path", s.path)
	db, err := pogreb.Open(s.path, &pogreb.Options{BackgroundSyncInterval: -1})
	if err != nil {
		s.logger.Error("failed to initialize pogreb store", "err", err)
		return err
	}

	s.db = db
	atomic.StoreUint32(&s.initialized, 1)
	s.logger.Info(fmt.Sprintf("KVStore has %d entries", db.Count()))
	return nil
}

func OpenKVStore(logger *log.Logger, path string) (KVStore, error) {
	store := &pogrebKVStore{
		logger: logger,
		path:   path,
	}

	// Open the database in background as it is possible it will do a full-reindex on startup after a crash:
	// https://github.com/akrylysov/pogreb/issues/35
	initErrCh := make(chan error)
	go func() {
		initErrCh <- store.init()
	}()

	select {
	case err := <-initErrCh:
		// Database initialized in time.
		if err != nil {
			return nil, err
		}
		return store, nil
	case <-time.After(30 * time.Second):
		// Database is likely doing a full-reindex after a crash which can take a long time (multiple hours).
		// Continue without cache while the database is reindexing in the background. Once it's done,
		// the cache will be used.
		// NOTE: A failure during the database reindex will be ignored (but logged), therefore in that scenario
		// the cache will never be initialized.
		logger.Warn("KVStore initialization timed out, continuing without cache while the database is reindexing in the background")
		return store, nil
	}
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
		return fmt.Errorf("failed to unmarshal the value for key %s from cache into %T: %v; raw value was %x", key.Pretty(), value, err, raw)
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
		loggingCache, ok := cache.(*pogrebKVStore)
		if ok {
			loggingCache.logger.Warn(fmt.Sprintf("error fetching %s from cache: %v", key.Pretty(), err))
		}
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
