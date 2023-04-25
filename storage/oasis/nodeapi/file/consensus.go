package file

import (
	"context"

	"github.com/akrylysov/pogreb"
	beacon "github.com/oasisprotocol/oasis-core/go/beacon/api"
	coreCommon "github.com/oasisprotocol/oasis-core/go/common"
	"github.com/oasisprotocol/oasis-core/go/common/cbor"
	consensus "github.com/oasisprotocol/oasis-core/go/consensus/api"
	genesis "github.com/oasisprotocol/oasis-core/go/genesis/api"

	"github.com/oasisprotocol/oasis-indexer/storage/oasis/nodeapi"
)

// FileConsensusApiLite provides access to the consensus API of an Oasis node.
// Since FileConsensusApiLite is backed by a file containing the cached responses
// to `ConsensusApiLite` calls, this data is inherently compatible with the
// current indexer and can thus handle heights from both Cobalt/Damask.
type FileConsensusApiLite struct {
	db           pogreb.DB
	consensusApi nodeapi.ConsensusApiLite
}

var _ nodeapi.ConsensusApiLite = (*FileConsensusApiLite)(nil)

func NewFileConsensusApiLite(filename string, consensusApi nodeapi.ConsensusApiLite) (*FileConsensusApiLite, error) {
	db, err := pogreb.Open(filename, &pogreb.Options{BackgroundSyncInterval: -1})
	if err != nil {
		return nil, err
	}
	return &FileConsensusApiLite{
		db:           *db,
		consensusApi: consensusApi,
	}, nil
}

// getFromCacheOrCall fetches the value of `cacheKey` from the cache if it exists,
// interpreted as a `Value`. If it does not exist, it calls `valueFunc` to get the
// value, and caches it before returning it.
// `height` is taken as an explicit parameter to catch non-cacheable calls. If `valueFunc`
// is not height-based, `height` should be set to `nil`.
func getFromCacheOrCall[Value any](cache pogreb.DB, height *int64, cacheKey []byte, valueFunc func() (*Value, error)) (*Value, error) {
	// If the latest height was requested, the response is not cacheable, so we have to hit the backing API.
	if height != nil && *height == consensus.HeightLatest {
		return valueFunc()
	}

	// If the value is cached, return it.
	isCached, err := cache.Has(cacheKey)
	if err != nil {
		return nil, err
	}
	if isCached {
		raw, err := cache.Get(cacheKey)
		if err != nil {
			return nil, err
		}
		var result *Value
		err = cbor.Unmarshal(raw, &result)
		return result, err
	}

	// Otherwise, the value is not cached. Call the backing API to get it.
	result, err := valueFunc()
	if err != nil {
		return nil, err
	}

	// Store value in cache for later use.
	return result, cache.Put(cacheKey, cbor.Marshal(result))
}

// Like getFromCacheOrCall, but for slice-typed return values.
func getSliceFromCacheOrCall[Response any](cache pogreb.DB, height *int64, cacheKey []byte, valueFunc func() ([]Response, error)) ([]Response, error) {
	// Use `getFromCacheOrCall()` to avoid duplicating the cache update logic.
	responsePtr, err := getFromCacheOrCall(cache, height, cacheKey, func() (*[]Response, error) {
		response, err := valueFunc()
		if response == nil {
			return nil, err
		}
		// Return the response wrapped in a pointer to conform to the signature of `getFromCacheOrCall()`.
		return &response, err
	})
	if responsePtr == nil {
		return nil, err
	}
	// Undo the pointer wrapping.
	return *responsePtr, err
}

func (c *FileConsensusApiLite) GetGenesisDocument(ctx context.Context) (*genesis.Document, error) {
	return getFromCacheOrCall(
		c.db, nil, /* height */
		generateCacheKey("GetGenesisDocument"),
		func() (*genesis.Document, error) { return c.consensusApi.GetGenesisDocument(ctx) },
	)
}

func (c *FileConsensusApiLite) StateToGenesis(ctx context.Context, height int64) (*genesis.Document, error) {
	return getFromCacheOrCall(
		c.db, &height,
		generateCacheKey("StateToGenesis", height),
		func() (*genesis.Document, error) { return c.consensusApi.StateToGenesis(ctx, height) },
	)
}

func (c *FileConsensusApiLite) GetBlock(ctx context.Context, height int64) (*consensus.Block, error) {
	return getFromCacheOrCall(
		c.db, &height,
		generateCacheKey("GetBlock", height),
		func() (*consensus.Block, error) { return c.consensusApi.GetBlock(ctx, height) },
	)
}

func (c *FileConsensusApiLite) GetTransactionsWithResults(ctx context.Context, height int64) ([]nodeapi.TransactionWithResults, error) {
	return getSliceFromCacheOrCall(
		c.db, &height,
		generateCacheKey("GetTransactionsWithResults", height),
		func() ([]nodeapi.TransactionWithResults, error) {
			return c.consensusApi.GetTransactionsWithResults(ctx, height)
		},
	)
}

func (c *FileConsensusApiLite) GetEpoch(ctx context.Context, height int64) (beacon.EpochTime, error) {
	time, err := getFromCacheOrCall(
		c.db, &height,
		generateCacheKey("GetEpoch", height),
		func() (*beacon.EpochTime, error) {
			time, err := c.consensusApi.GetEpoch(ctx, height)
			return &time, err
		},
	)
	return *time, err
}

func (c *FileConsensusApiLite) RegistryEvents(ctx context.Context, height int64) ([]nodeapi.Event, error) {
	return getSliceFromCacheOrCall(
		c.db, &height,
		generateCacheKey("RegistryEvents", height),
		func() ([]nodeapi.Event, error) { return c.consensusApi.RegistryEvents(ctx, height) },
	)
}

func (c *FileConsensusApiLite) StakingEvents(ctx context.Context, height int64) ([]nodeapi.Event, error) {
	return getSliceFromCacheOrCall(
		c.db, &height,
		generateCacheKey("StakingEvents", height),
		func() ([]nodeapi.Event, error) { return c.consensusApi.StakingEvents(ctx, height) },
	)
}

func (c *FileConsensusApiLite) GovernanceEvents(ctx context.Context, height int64) ([]nodeapi.Event, error) {
	return getSliceFromCacheOrCall(
		c.db, &height,
		generateCacheKey("GovernanceEvents", height),
		func() ([]nodeapi.Event, error) { return c.consensusApi.GovernanceEvents(ctx, height) },
	)
}

func (c *FileConsensusApiLite) RoothashEvents(ctx context.Context, height int64) ([]nodeapi.Event, error) {
	return getSliceFromCacheOrCall(
		c.db, &height,
		generateCacheKey("RoothashEvents", height),
		func() ([]nodeapi.Event, error) { return c.consensusApi.RoothashEvents(ctx, height) },
	)
}

func (c *FileConsensusApiLite) GetValidators(ctx context.Context, height int64) ([]nodeapi.Validator, error) {
	return getSliceFromCacheOrCall(
		c.db, &height,
		generateCacheKey("GetValidators", height),
		func() ([]nodeapi.Validator, error) { return c.consensusApi.GetValidators(ctx, height) },
	)
}

func (c *FileConsensusApiLite) GetCommittees(ctx context.Context, height int64, runtimeID coreCommon.Namespace) ([]nodeapi.Committee, error) {
	return getSliceFromCacheOrCall(
		c.db, &height,
		generateCacheKey("GetCommittee", height, runtimeID),
		func() ([]nodeapi.Committee, error) { return c.consensusApi.GetCommittees(ctx, height, runtimeID) },
	)
}

func (c *FileConsensusApiLite) GetProposal(ctx context.Context, height int64, proposalID uint64) (*nodeapi.Proposal, error) {
	return getFromCacheOrCall(
		c.db, &height,
		generateCacheKey("GetProposal", height, proposalID),
		func() (*nodeapi.Proposal, error) { return c.consensusApi.GetProposal(ctx, height, proposalID) },
	)
}
