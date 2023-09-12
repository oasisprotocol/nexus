// Package util contains utility analyzer functionality.
package util

import (
	"fmt"
	"sync"
	"time"

	beacon "github.com/oasisprotocol/oasis-core/go/beacon/api"
	staking "github.com/oasisprotocol/oasis-core/go/staking/api"
)

const (
	timeoutUpperBound = 60 * time.Second
	// SHA256 of empty string.
	// oasis-core tags event that are not associated with any real transactions.
	// For example, the DebondingStart escrow event.
	EmptyTxHash = "c672b8d1ef56ed28ab87c3622c5114069bdd3ad7b8f9737498d0c01ecef0967a"
	// The zero-value transaction hash.
	// oasis-core tags runtime events that are not associated with any real transactions.
	// See https://github.com/oasisprotocol/oasis-core/blob/b6c20732c43f95b51d31e42968acd118cf0ac1b6/go/runtime/transaction/tags.go#L5-L7
	ZeroTxHash = "0000000000000000000000000000000000000000000000000000000000000000"
)

// Backoff implements retry backoff on failure.
type Backoff struct {
	currentTimeout time.Duration
	minTimeoutStep time.Duration
	maximumTimeout time.Duration
}

// NewBackoff returns a new exponential backoff.
//
// Initial timeout is always 0.
// The minTimeoutStep is the timeout step on first failure. A call to `Success()` that
// lowers the timeout below the minTimeoutStep, reduces the timeout back to 0.
// The maximum timeout is the maximum possible timeout to use.
func NewBackoff(minTimeoutStep time.Duration, maximumTimeout time.Duration) (*Backoff, error) {
	if minTimeoutStep > maximumTimeout {
		return nil, fmt.Errorf(
			"minimum timeout step %d greater than maximum timeout %d",
			minTimeoutStep,
			maximumTimeout,
		)
	}
	if minTimeoutStep <= 0 {
		return nil, fmt.Errorf(
			"minimum timeout %d step must be greater than 0",
			minTimeoutStep,
		)
	}
	if maximumTimeout >= timeoutUpperBound {
		return nil, fmt.Errorf(
			"maximum timeout %ds greater than upper bound %ds",
			maximumTimeout,
			timeoutUpperBound,
		)
	}
	return &Backoff{0, minTimeoutStep, maximumTimeout}, nil
}

// Wait waits for the appropriate backoff interval.
func (b *Backoff) Wait() {
	time.Sleep(b.currentTimeout)
}

// Success slightly decreases the backoff interval.
func (b *Backoff) Success() {
	b.currentTimeout *= 9
	b.currentTimeout /= 10
	// If we go below the minimum timeout step on success, reset the timeout back to 0.
	if b.currentTimeout < b.minTimeoutStep {
		b.currentTimeout = 0
	}
}

// Failure increases the backoff interval.
func (b *Backoff) Failure() {
	b.currentTimeout *= 2
	// If we are below the minimum timeout step on failure, increase the timeout to the minimum.
	if b.currentTimeout < b.minTimeoutStep {
		b.currentTimeout = b.minTimeoutStep
	}
	// Cap the timeout at the maximum.
	if b.currentTimeout > b.maximumTimeout {
		b.currentTimeout = b.maximumTimeout
	}
}

// Reset resets the backoff.
func (b *Backoff) Reset() {
	b.currentTimeout = 0
}

// Timeout returns the backoff timeout.
func (b *Backoff) Timeout() time.Duration {
	return b.currentTimeout
}

// closingChannel returns a channel that closes when the wait group `wg` is done.
func ClosingChannel(wg *sync.WaitGroup) <-chan struct{} {
	c := make(chan struct{})
	go func() {
		wg.Wait()
		close(c)
	}()
	return c
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
