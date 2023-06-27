package file

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/oasisprotocol/nexus/log"
)

// Returns an open KVStore, and a function to clean it up.
func openTestKVStore(t *testing.T) (KVStore, func()) {
	path, err := os.MkdirTemp("", "indexer-kv-test")
	require.NoError(t, err)
	kv, err := OpenKVStore(log.NewDefaultLogger("unit-test"), path)
	require.NoError(t, err)
	return kv, func() {
		err := kv.Close()
		os.RemoveAll(path)
		require.NoError(t, err)
	}
}

func TestHappyPath(t *testing.T) {
	kv, closer := openTestKVStore(t)
	defer closer()

	has, err := kv.Has([]byte("mykey"))
	require.NoError(t, err)
	require.False(t, has)

	err = kv.Put([]byte("mykey"), []byte("myval"))
	require.NoError(t, err)

	has, err = kv.Has([]byte("mykey"))
	require.NoError(t, err)
	require.True(t, has)

	val, err := kv.Get([]byte("mykey"))
	require.NoError(t, err)
	require.Equal(t, []byte("myval"), val)
}

func TestGetFromCacheOrCallSuccess(t *testing.T) {
	kv, closer := openTestKVStore(t)
	defer closer()

	var callCount int
	generator := func() (*string, error) {
		callCount++
		val := "myval"
		return &val, nil
	}

	// First call should call the function.
	val, err := GetFromCacheOrCall(kv, false /*volatile*/, generateCacheKey("mykey"), generator)
	require.NoError(t, err)
	require.Equal(t, "myval", *val)
	require.Equal(t, 1, callCount)

	// Second call should not call the function.
	val, err = GetFromCacheOrCall(kv, false /*volatile*/, generateCacheKey("mykey"), generator)
	require.NoError(t, err)
	require.Equal(t, "myval", *val)
	require.Equal(t, 1, callCount)

	// If a key is marked volatile, the function should be called every time.
	val, err = GetFromCacheOrCall(kv, true /*volatile*/, generateCacheKey("mykey"), generator)
	require.NoError(t, err)
	require.Equal(t, "myval", *val)
	require.Equal(t, 2, callCount)
}

func TestGetFromCacheOrCallError(t *testing.T) {
	kv, closer := openTestKVStore(t)
	defer closer()

	generator := func() (*int, error) {
		one := 1
		return &one, fmt.Errorf("myerr")
	}

	// The call should propagate the generator function error.
	val, err := GetFromCacheOrCall(kv, false /*volatile*/, generateCacheKey("mykey"), generator)
	require.Error(t, err)
	require.Nil(t, val)
}

func TestGetFromCacheOrCallTypeMismatch(t *testing.T) {
	kv, closer := openTestKVStore(t)
	defer closer()

	stringGenerator := func() (*string, error) {
		val := "myval"
		return &val, nil
	}
	intGenerator := func() (*int, error) {
		val := 123
		return &val, nil
	}

	// Put a string into the cache.
	_, err := GetFromCacheOrCall(kv, false /*volatile*/, generateCacheKey("mykey"), stringGenerator)
	require.NoError(t, err)

	// Try to fetch it and interpret it as an int.
	// It should log a warning but recover by calling the generator function.
	myInt, err := GetFromCacheOrCall(kv, false /*volatile*/, generateCacheKey("mykey"), intGenerator)
	require.NoError(t, err)
	require.Equal(t, 123, *myInt)
}
