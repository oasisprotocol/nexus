package analyzer

import (
	"context"
	"errors"
)

var (
	// ErrOutOfRange is returned if the current block does not fall within the
	// analyzer's analysis range.
	ErrOutOfRange = errors.New("range not found. no data source available")

	// ErrLatestBlockNotFound is returned if the analyzer has not indexed any
	// blocks yet. This indicates to begin from the start of its range.
	ErrLatestBlockNotFound = errors.New("latest block not found")
)

// Analyzer is a worker that analyzes a subset of the Oasis Network.
type Analyzer interface {
	// Start starts the analyzer. The method should return once the analyzer
	// is confident it has (and will have) no more work to do; that's possibly never.
	Start(ctx context.Context)

	// Name returns the name of the analyzer.
	Name() string
}

type BlockAnalysisMode string

const (
	FastSyncMode BlockAnalysisMode = "fast-sync"
	SlowSyncMode BlockAnalysisMode = "slow-sync"
)
