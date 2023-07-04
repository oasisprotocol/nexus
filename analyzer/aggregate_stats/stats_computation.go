package aggregate_stats

import (
	"context"
	"fmt"
	"time"

	"github.com/oasisprotocol/nexus/storage"
)

type statsComputation struct {
	target storage.TargetStorage

	// Stats name.
	name string
	// Layer (e.g. consensus, emerald...) for which the stats are computed.
	layer string

	outputTable  string
	outputColumn string

	// Stat window configurations.
	windowSize time.Duration
	windowStep time.Duration

	// Return the latest timestamp up until the data is available.
	latestAvailableDataTs func(ctx context.Context, target storage.TargetStorage) (*time.Time, error)
	// Query that computes the stats. The query should expect 3 arguments:
	// $1 - layer name
	// $2 - start of window timestamp
	// $3 - end of window timestamp
	// The result should be a uint64 compatible number - the computed stat.
	statsQuery string
}

func (s *statsComputation) LatestComputedTs(ctx context.Context) (time.Time, error) {
	var latestStatsTs time.Time
	err := s.target.QueryRow(
		ctx,
		fmt.Sprintf(QueryLatestStatsComputation, s.outputTable),
		s.layer,
	).Scan(&latestStatsTs)
	return latestStatsTs, err
}

func (s *statsComputation) ComputeStats(ctx context.Context, windowStart time.Time, windowEnd time.Time) (*storage.QueryBatch, error) {
	batch := &storage.QueryBatch{}
	row := s.target.QueryRow(
		ctx,
		s.statsQuery,
		s.layer,
		windowStart,
		windowEnd,
	)
	var result uint64
	if err := row.Scan(&result); err != nil {
		return nil, err
	}

	// Insert computed stat.
	batch.Queue(fmt.Sprintf(QueryInsertStatsComputation, s.outputTable, s.outputColumn), s.layer, windowEnd.UTC(), result)

	return batch, nil
}
