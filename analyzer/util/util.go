// Package util contains utility analyzer functionality.
package util

import (
	"fmt"
	"math"
	"time"

	beacon "github.com/oasisprotocol/oasis-core/go/beacon/api"
	staking "github.com/oasisprotocol/oasis-core/go/staking/api"
)

const (
	initialTimeoutLowerBound = 0
	maximumTimeoutUpperBound = math.MaxInt64 / 2
)

// Backoff implements retry backoff on failure.
type Backoff struct {
	initialTimeout time.Duration
	currentTimeout time.Duration
	maximumTimeout time.Duration
}

// NewBackoff returns a new backoff.
func NewBackoff(initialTimeout time.Duration, maximumTimeout time.Duration) (*Backoff, error) {
	if initialTimeout <= initialTimeoutLowerBound {
		return nil, fmt.Errorf(
			"initial timeout %fs less than lower bound %ds",
			initialTimeout.Seconds(),
			initialTimeoutLowerBound,
		)
	}
	if maximumTimeout.Seconds() >= math.MaxInt64/2 {
		return nil, fmt.Errorf(
			"maximum timeout %fs greater than upper bound %ds",
			maximumTimeout.Seconds(),
			maximumTimeoutUpperBound,
		)
	}
	return &Backoff{initialTimeout, initialTimeout, maximumTimeout}, nil
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
