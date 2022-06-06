package util

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestBackoffWait tests if the backoff time is
// updated correctly.
func TestBackoffWait(t *testing.T) {
	backoff := NewBackoff(time.Millisecond, 10*time.Second)
	for i := 0; i < 10; i++ {
		backoff.Wait()
	}
	require.Equal(t, backoff.Timeout(), 1024*time.Millisecond)
}

// TestBackoffReset tests if the backoff time is
// reset correctly.
func TestBackoffReset(t *testing.T) {
	backoff := NewBackoff(time.Millisecond, 10*time.Second)
	backoff.Wait()
	backoff.Reset()
	require.Equal(t, backoff.Timeout(), time.Millisecond)
}

// TestBackoffMaximum tests if the backoff time is
// appropriately upper bounded.
func TestBackoffMaximum(t *testing.T) {
	backoff := NewBackoff(time.Millisecond, 10*time.Millisecond)
	for i := 0; i < 10; i++ {
		backoff.Wait()
	}
	require.Equal(t, backoff.Timeout(), 10*time.Millisecond)
}
