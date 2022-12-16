// Package util contains utility analyzer functionality.
package util

import (
	"fmt"
	"time"

	beacon "github.com/oasisprotocol/oasis-core/go/beacon/api"
	staking "github.com/oasisprotocol/oasis-core/go/staking/api"
)

const (
	InitialTimeoutLowerBoundSeconds = 0
	MaximumTimeoutUpperBoundSeconds = 60
	// SHA256 of empty string.
	// oasis-core tags event that are not associated with any real transactions.
	// For example, the DebondingStart escrow event.
	EmptyTxHash = "c672b8d1ef56ed28ab87c3622c5114069bdd3ad7b8f9737498d0c01ecef0967a"
)

// Backoff implements retry backoff on failure.
type Backoff struct {
	initialTimeout time.Duration
	currentTimeout time.Duration
	maximumTimeout time.Duration
}

// NewBackoff returns a new backoff.
func NewBackoff(initialTimeout time.Duration, maximumTimeout time.Duration) (*Backoff, error) {
	if initialTimeout <= InitialTimeoutLowerBoundSeconds {
		return nil, fmt.Errorf(
			"initial timeout %fs less than lower bound %ds",
			initialTimeout.Seconds(),
			InitialTimeoutLowerBoundSeconds,
		)
	}
	if maximumTimeout.Seconds() >= MaximumTimeoutUpperBoundSeconds {
		return nil, fmt.Errorf(
			"maximum timeout %fs greater than upper bound %ds",
			maximumTimeout.Seconds(),
			MaximumTimeoutUpperBoundSeconds,
		)
	}
	return &Backoff{initialTimeout, initialTimeout, maximumTimeout}, nil
}

// Wait waits for the appropriate backoff interval.
func (b *Backoff) Wait() {
	time.Sleep(b.currentTimeout)
}

// Success slightly decreases the backoff interval.
func (b *Backoff) Success() {
	b.currentTimeout *= 9
	b.currentTimeout /= 10
	if b.currentTimeout < b.initialTimeout {
		b.currentTimeout = b.initialTimeout
	}
}

// Failure increases the backoff interval.
func (b *Backoff) Failure() {
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

// CurrentBound returns the bound at the latest rate step that has started or nil if no step has started.
func CurrentBound(cs staking.CommissionSchedule, now beacon.EpochTime) (currentBound *staking.CommissionRateBoundStep, epochEnd uint64) {
	var latestStartedStep *staking.CommissionRateBoundStep
	i := 0
	for ; i < len(cs.Bounds); i++ {
		bound := &cs.Bounds[i]
		if bound.Start > now {
			break
		}
		latestStartedStep = bound
	}
	if latestStartedStep == nil {
		return nil, 0
	}

	if i >= len(cs.Bounds) {
		return latestStartedStep, 0
	}

	return latestStartedStep, uint64(cs.Bounds[i].Start - 1)
}

func SanitizeTxHash(hash string) *string {
	if hash == EmptyTxHash {
		return nil
	}

	return &hash
}
