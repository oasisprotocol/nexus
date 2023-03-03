package analyzer

import (
	"errors"
	"strings"

	oasisConfig "github.com/oasisprotocol/oasis-sdk/client-sdk/go/config"

	"github.com/oasisprotocol/oasis-indexer/storage"
)

var (
	// ErrOutOfRange is returned if the current block does not fall within the
	// analyzer's analysis range.
	ErrOutOfRange = errors.New("range not found. no data source available")

	// ErrLatestBlockNotFound is returned if the analyzer has not indexed any
	// blocks yet. This indicates to begin from the start of its range.
	ErrLatestBlockNotFound = errors.New("latest block not found")

	// ErrNetworkUnknown is returned if a chain context does not correspond
	// to a known network identifier.
	ErrNetworkUnknown = errors.New("network unknown")

	// ErrRuntimeUnknown is returned if a chain context does not correspond
	// to a known runtime identifier.
	ErrRuntimeUnknown = errors.New("runtime unknown")
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
	// ChainID is the chain ID for the underlying network.
	ChainID string

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

// ChainID is the ID of a chain.
type ChainID string

// String returns the string representation of a ChainID.
func (c ChainID) String() string {
	return string(c)
}

// Network is an instance of the Oasis Network.
type Network uint8

const (
	// NetworkTestnet is the identifier for testnet.
	NetworkTestnet Network = iota
	// NetworkMainnet is the identifier for mainnet.
	NetworkMainnet
	// NetworkUnknown is the identifier for an unknown network.
	NetworkUnknown = 255
)

// FromChainContext identifies a Network using its ChainContext.
func FromChainContext(chainContext string) (Network, error) {
	var network Network
	for name, nw := range oasisConfig.DefaultNetworks.All {
		if nw.ChainContext == chainContext {
			if err := network.Set(name); err != nil {
				return NetworkUnknown, err
			}
			return network, nil
		}
	}

	return NetworkUnknown, ErrNetworkUnknown
}

// Set sets the Network to the value specified by the provided string.
func (n *Network) Set(s string) error {
	switch strings.ToLower(s) {
	case "mainnet":
		*n = NetworkMainnet
	case "testnet":
		*n = NetworkTestnet
	default:
		return ErrNetworkUnknown
	}

	return nil
}

// String returns the string representation of a network.
func (n Network) String() string {
	switch n {
	case NetworkTestnet:
		return "testnet"
	case NetworkMainnet:
		return "mainnet"
	default:
		return "unknown"
	}
}

// Runtime is an identifier for a runtime on the Oasis Network.
type Runtime string

const (
	RuntimeEmerald  Runtime = "emerald"
	RuntimeCipher   Runtime = "cipher"
	RuntimeSapphire Runtime = "sapphire"
	RuntimeUnknown  Runtime = "unknown"
)

// String returns the string representation of a runtime.
func (r Runtime) String() string {
	return string(r)
}

// ID returns the ID for a Runtime on the provided network.
func (r Runtime) ID(n Network) (string, error) {
	for nname, nw := range oasisConfig.DefaultNetworks.All {
		if nname == n.String() {
			for pname, pt := range nw.ParaTimes.All {
				if pname == r.String() {
					return pt.ID, nil
				}
			}

			return "", ErrRuntimeUnknown
		}
	}

	return "", ErrRuntimeUnknown
}
