package util

import (
	"testing"
	"time"

	beacon "github.com/oasisprotocol/oasis-core/go/beacon/api"
	"github.com/oasisprotocol/oasis-core/go/common/quantity"
	staking "github.com/oasisprotocol/oasis-core/go/staking/api"
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
