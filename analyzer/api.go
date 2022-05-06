package analyzer

import (
	"github.com/oasislabs/oasis-block-indexer/go/storage"
)

// Analyzer is a worker that analyzes a subset of the Oasis Network.
type Analyzer interface {
	// SetRange sets configuration for the range of blocks to process
	// to the analyzer.
	SetRange(RangeConfig) error

	// Start starts the analyzer.
	Start()

	// Name returns the name of the analyzer.
	Name() string
}

// RangeConfig specifies configuration parameters
// for processing a range of blocks.
type RangeConfig struct {
	// From is the first block to process in this range, inclusive.
	From int64

	// To is the last block to process in this range, inclusive.
	To int64

	// Source is the storage source from which to fetch block data
	// when processing blocks in this range.
	Source storage.SourceStorage
}
