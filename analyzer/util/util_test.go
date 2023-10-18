package util

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/oasisprotocol/oasis-core/go/common/quantity"

	beacon "github.com/oasisprotocol/nexus/coreapi/v22.2.11/beacon/api"
	staking "github.com/oasisprotocol/nexus/coreapi/v22.2.11/staking/api"
)

// TestBackoffWait tests if the backoff time is
// updated correctly.
func TestBackoffWait(t *testing.T) {
	backoff, err := NewBackoff(time.Millisecond, 10*time.Second)
	require.Nil(t, err)

	for i := 0; i < 10; i++ {
		backoff.Failure()
	}
	require.Equal(t, 512*time.Millisecond, backoff.Timeout())
}

// TestBackoffReset tests if the backoff time is
// reset correctly.
func TestBackoffReset(t *testing.T) {
	backoff, err := NewBackoff(time.Millisecond, 10*time.Second)
	require.Nil(t, err)

	backoff.Failure()
	backoff.Reset()
	require.Equal(t, 0*time.Second, backoff.Timeout())
}

// TestBackoffMaximum tests if the backoff time is
// appropriately upper bounded.
func TestBackoffMaximum(t *testing.T) {
	backoff, err := NewBackoff(time.Millisecond, 10*time.Millisecond)
	require.Nil(t, err)

	for i := 0; i < 10; i++ {
		backoff.Failure()
	}
	require.Equal(t, backoff.Timeout(), 10*time.Millisecond)
}

// TestBackoffMinimum tests if the backoff time is
// appropriately lower bounded.
func TestBackoffMinimum(t *testing.T) {
	backoff, err := NewBackoff(time.Millisecond, 10*time.Millisecond)
	require.Nil(t, err)

	backoff.Failure()
	for i := 0; i < 100; i++ {
		backoff.Success()
	}
	require.Equal(t, 0*time.Second, backoff.Timeout())
}

// TestBackoff tests the backoff logic.
func TestBackoff(t *testing.T) {
	backoff, err := NewBackoff(100*time.Millisecond, 6*time.Second)
	require.Nil(t, err)

	for i := 0; i < 2; i++ {
		backoff.Failure()
	}
	require.Equal(t, 200*time.Millisecond, backoff.Timeout())
	backoff.Success()
	require.Equal(t, (200*9/10)*time.Millisecond, backoff.Timeout())
	for i := 0; i < 6; i++ {
		backoff.Success()
	}
	require.Equal(t, 0*time.Second, backoff.Timeout())
	backoff.Success()
	require.Equal(t, 0*time.Second, backoff.Timeout())
}

// TestMinimumBelowMaximum tests that the configured minimum timeout step must be less that the maximum.
func TestMinimumGreaterThanMaximum(t *testing.T) {
	_, err := NewBackoff(time.Millisecond*2, time.Millisecond)
	require.NotNil(t, err)
}

// TestMaximumTimeoutUpperBound tests that the maximum timeout upper
// bound is respected.
func TestMaximumTimeoutUpperBound(t *testing.T) {
	_, err := NewBackoff(time.Millisecond, timeoutUpperBound+1*time.Second)
	require.NotNil(t, err)
}

// TestMinTimeoutStepBound tests that the minimum timeout step is positive.
func TestMinTimeoutStepBound(t *testing.T) {
	_, err := NewBackoff(0, 10*time.Millisecond)
	require.NotNil(t, err)
}

func TestCurrentBound(t *testing.T) {
	commissionSchedule := staking.CommissionSchedule{
		Rates: []staking.CommissionRateStep{},
		Bounds: []staking.CommissionRateBoundStep{
			{
				Start:   1,
				RateMin: *quantity.NewFromUint64(0),
				RateMax: *quantity.NewFromUint64(1000),
			},
			{
				Start:   5,
				RateMin: *quantity.NewFromUint64(5),
				RateMax: *quantity.NewFromUint64(1000),
			},
			{
				Start:   10,
				RateMin: *quantity.NewFromUint64(10),
				RateMax: *quantity.NewFromUint64(1000),
			},
		},
	}
	bound, epochEnd := CurrentBound(commissionSchedule, beacon.EpochTime(4))
	require.Equal(t, bound, &staking.CommissionRateBoundStep{
		Start:   1,
		RateMin: *quantity.NewFromUint64(0),
		RateMax: *quantity.NewFromUint64(1000),
	})
	require.Equal(t, epochEnd, uint64(4))
	bound, epochEnd = CurrentBound(commissionSchedule, beacon.EpochTime(5))
	require.Equal(t, bound, &staking.CommissionRateBoundStep{
		Start:   5,
		RateMin: *quantity.NewFromUint64(5),
		RateMax: *quantity.NewFromUint64(1000),
	})
	require.Equal(t, epochEnd, uint64(9))
	bound, epochEnd = CurrentBound(commissionSchedule, beacon.EpochTime(10))
	require.Equal(t, bound, &staking.CommissionRateBoundStep{
		Start:   10,
		RateMin: *quantity.NewFromUint64(10),
		RateMax: *quantity.NewFromUint64(1000),
	})
	require.Equal(t, epochEnd, uint64(0))
}
