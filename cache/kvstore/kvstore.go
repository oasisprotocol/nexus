// Package kvstore implements a key-value store.
package kvstore

import (
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"

	"github.com/akrylysov/pogreb"
	"github.com/oasisprotocol/oasis-core/go/common/cbor"

	"github.com/oasisprotocol/nexus/log"
	"github.com/oasisprotocol/nexus/metrics"
)

// A key in the KVStore.
type CacheKey []byte

func GenerateCacheKey(methodName string, params ...interface{}) CacheKey {
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

	path    string
	logger  *log.Logger
	metrics *metrics.AnalysisMetrics // if nil, no metrics are emitted

	// Address of the atomic variable that indicates whether the store is initialized.
	// Synchronisation is required because the store is opened in background goroutine.
	initialized uint32
}

var _ KVStore = (*pogrebKVStore)(nil)

func (s *pogrebKVStore) isInitialized() bool {
	return atomic.LoadUint32(&s.initialized) == 1
}

// Get implements KVStore.
// NOTE: Cache hit/miss metrics are not captured if you call this method directly.
// Consider using the Get*FromCacheOrCall() methods instead.
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
	if !s.isInitialized() {
		// If pogreb is in the middle of recovery in the background, it will
		// die and have to start over next time.
		s.logger.Warn("skipping closing uninitialized KVStore")
		return nil
	}
	s.logger.Info("closing KVStore", "path", s.path)
	return s.db.Close()
}

// Returns true if path exists. Uses simplified error handling
// to match pogreb's behavior.
func pathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// Returns a list of files that match any of the patterns, but do not match any of the antipatterns.
func glob(patterns []string, antipatterns []string) ([]string, error) {
	files := map[string]struct{}{}
	for _, pattern := range patterns {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return nil, err
		}
		for _, match := range matches {
			files[match] = struct{}{}
		}
	}
	for _, antipattern := range antipatterns {
		matches, err := filepath.Glob(antipattern)
		if err != nil {
			return nil, err
		}
		for _, match := range matches {
			delete(files, match)
		}
	}
	filesArr := make([]string, 0, len(files))
	for k := range files {
		filesArr = append(filesArr, k)
	}
	return filesArr, nil
}

// Moves all files that match the src glob patterns to the destination directory.
// NOTE: If multiple source files have the same filename, one will clobber the others when moved!
func moveFiles(srcPatterns []string, srcAntipatters []string, dst string) error {
	files, err := glob(srcPatterns, srcAntipatters)
	if err != nil {
		return fmt.Errorf("unable to glob for files to move: %w", err)
	}

	// Create the destination directory.
	if err := os.MkdirAll(dst, 0o700); err != nil {
		return fmt.Errorf("unable to create destination directory %s: %w", dst, err)
	}

	for _, srcFile := range files {
		dstFile := filepath.Join(dst, filepath.Base(srcFile))
		if err := os.Rename(srcFile, dstFile); err != nil {
			return fmt.Errorf("unable to move file %s to %s: %w", srcFile, dstFile, err)
		}
	}
	return nil
}

// Deletes all files that match the glob pattern.
func deleteFiles(pattern string) error {
	files, err := filepath.Glob(pattern)
	if err != nil {
		return fmt.Errorf("unable to glob for files %s to delete: %w", pattern, err)
	}
	var lastErr error
	for _, f := range files {
		if err := os.Remove(f); err != nil {
			lastErr = fmt.Errorf("unable to delete file %s: %w", f, err)
		}
	}
	return lastErr
}

// Gets rid of excessively backed-up pogreb index files.
// If we know pogreb will reindex, this deletes or possibly backs up the old index files.
func (s *pogrebKVStore) preBackup() {
	backupNeeded := pathExists(filepath.Join(s.path, "lock"))
	backupDir := filepath.Join(filepath.Dir(s.path), filepath.Base(s.path)+".backup")
	if backupNeeded {
		// pogreb is sure to try to back up its indexes and build new ones.
		s.logger.Info("pogreb lock file found; preemptively deleting or backing up indexes", "path", s.path, "backup_path", backupDir)
		if !pathExists(backupDir) { // If an older backup exists, keep that one.
			err := moveFiles(
				[]string{filepath.Join(s.path, "*")},
				[]string{
					filepath.Join(s.path, "*.psg"), // the data that needs to be reindexed
					filepath.Join(s.path, "lock"),  // will trigger a reindex
				},
				backupDir,
			)
			if err != nil {
				s.logger.Warn("failed to move pogreb index files to backup directory", "err", err, "path", s.path, "backup_path", backupDir)
			}
		}
	}
	// In rare cases, pogreb might still back up indexes even if "lock" is not present.
	// Also, our moving operation might have left files behind, e.g. because older backups existed.
	// Prevent build-up of extensively-long .bac.bac.bac.... filenames.
	if err := deleteFiles(filepath.Join(s.path, "*.bac.bac")); err != nil {
		s.logger.Warn("failed to delete excessively backed-up pogreb index files", "err", err)
	}
}

func (s *pogrebKVStore) init() error {
	// Pogreb backs up its indices into <oldname>.bac. ".bac" becomes ".bac.bac", etc.
	// If nexus loop-crashes in k8s, the filenames grow too long for the filesystem
	// and pogreb is then unable to initialize without manual intervention.
	// Prevent this by cleaning up the backup files before opening the store.
	s.preBackup()

	// Open the DB. If a reindex is needed, this can take hours.
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

// Initializes a new KVStore backed by a database at `path`, or opens an existing one.
// `metrics` can be `nil`, in which case no metrics are emitted during operation.
func OpenKVStore(logger *log.Logger, path string, metrics *metrics.AnalysisMetrics) (KVStore, error) {
	store := &pogrebKVStore{
		logger:  logger,
		path:    path,
		metrics: metrics,
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
// Intended only for debugging. Not guaranteed to be a stable representation.
func (cacheKey CacheKey) Pretty() string {
	var pretty string
	var parsed interface{}
	err := cbor.Unmarshal(cacheKey, &parsed)
	if err == nil {
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

func increaseReadCounter(cache KVStore, status metrics.CacheReadStatus) {
	// Make sure the cache supports metric-gathering.
	if metricsCache, ok := cache.(*pogrebKVStore); ok && metricsCache.metrics != nil {
		// Increase the counter.
		metricsCache.metrics.LocalCacheReads(status).Inc()
	}
}

// fetchTypedValue fetches the value of `cacheKey` from the cache, interpreted as a `Value`.
func fetchTypedValue[Value any](cache KVStore, key CacheKey, value *Value) error {
	isCached, err := cache.Has(key)
	if err != nil {
		increaseReadCounter(cache, metrics.CacheReadStatusError)
		return err
	}
	if !isCached {
		increaseReadCounter(cache, metrics.CacheReadStatusMiss)
		return errNoSuchKey
	}
	raw, err := cache.Get(key)
	if err != nil {
		increaseReadCounter(cache, metrics.CacheReadStatusError)
		return fmt.Errorf("failed to fetch key %s from cache: %v", key.Pretty(), err)
	}
	err = cbor.Unmarshal(raw, value)
	if err != nil {
		increaseReadCounter(cache, metrics.CacheReadStatusBadValue)
		return fmt.Errorf("failed to unmarshal the value for key %s from cache into %T: %v; raw value was %x", key.Pretty(), value, err, raw)
	}
	increaseReadCounter(cache, metrics.CacheReadStatusHit)

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
		if loggingCache, ok := cache.(*pogrebKVStore); ok {
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

func GetMapFromCacheOrCall[Key comparable, Value any](cache KVStore, volatile bool, key CacheKey, valueFunc func() (map[Key]Value, error)) (map[Key]Value, error) {
	// Use `getFromCacheOrCall()` to avoid duplicating the cache update logic.
	responsePtr, err := GetFromCacheOrCall(cache, volatile, key, func() (*map[Key]Value, error) {
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
