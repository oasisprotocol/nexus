package analyzer

import (
	"github.com/oasislabs/oasis-block-indexer/go/storage"
)

// Analyzer is a worker that analyzes a subset of the Oasis Network.
type Analyzer interface {
	// AddRange adds configuration for processing a range of blocks
	// to the analyzer.
	AddRange(RangeConfig) error

	// Start starts the analyzer.
	Start()

	// Name returns the name of the analyzer.
	Name() string
}

// RangeConfig specifies configuration parameters
// for processing a range of blocks.
type RangeConfig struct {
	From   int64
	To     int64
	Source storage.SourceStorage
}
