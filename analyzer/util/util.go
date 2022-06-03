// Package util contains utility analyzer functionality.
package util

import (
	"time"
)

// Backoff implements retry backoff on failure.
type Backoff struct {
	initialTimeout time.Duration
	currentTimeout time.Duration
	maximumTimeout time.Duration
}

// NewBackoff returns a new backoff.
func NewBackoff(initialTimeout time.Duration, maximumTimeout time.Duration) *Backoff {
	return &Backoff{initialTimeout, initialTimeout, maximumTimeout}
}

// Wait waits for the appropriate backoff interval.
func (b *Backoff) Wait() {
	time.Sleep(b.currentTimeout)
	b.currentTimeout *= 2
	if b.currentTimeout > b.maximumTimeout {
		b.currentTimeout = b.maximumTimeout
	}
}

// Reset resets the backoff.
func (b *Backoff) Reset() {
	b.currentTimeout = b.initialTimeout
}

// Timeout returns the backoff timeout.
func (b *Backoff) Timeout() time.Duration {
	return b.currentTimeout
}
