package analyzer

import (
	"errors"

	sdkConfig "github.com/oasisprotocol/oasis-sdk/client-sdk/go/config"

	"github.com/oasisprotocol/oasis-indexer/common"
	"github.com/oasisprotocol/oasis-indexer/storage"
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
	// Start starts the analyzer.
	Start()

	// Name returns the name of the analyzer.
	Name() string
}

// ConsensusConfig specifies configuration parameters for
// for processing the consensus layer.
type ConsensusConfig struct {
	// GenesisChainContext is the chain context that specifies which genesis
	// file to analyze.
	GenesisChainContext string

	// Range is the range of blocks to process.
	// If this is set, the analyzer analyzes blocks in the provided range.
	Range BlockRange

	// Source is the storage source from which to fetch block data
	// when processing blocks in this range.
	Source storage.ConsensusSourceStorage
}

// BlockRange is a range of blocks.
type BlockRange struct {
	// From is the first block to process in this range, inclusive.
	From int64

	// To is the last block to process in this range, inclusive.
	To int64
}

// RuntimeConfig specifies configuration parameters for
// processing the runtime layer.
type RuntimeConfig struct {
	// RuntimeName is which runtime to analyze.
	RuntimeName common.Runtime

	// ParaTime is the SDK ParaTime structure describing the runtime.
	ParaTime *sdkConfig.ParaTime

	// Range is the range of rounds to process.
	// If this is set, the analyzer analyzes rounds in the provided range.
	Range RoundRange

	// Source is the storage source from which to fetch block data
	// when processing blocks in this range.
	Source storage.RuntimeSourceStorage
}

// RoundRange is a range of blocks.
type RoundRange struct {
	// From is the first block to process in this range, inclusive.
	From uint64

	// To is the last block to process in this range, inclusive.
	To uint64
}
