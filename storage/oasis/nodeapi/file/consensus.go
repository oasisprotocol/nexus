package file

import (
	"context"

	coreCommon "github.com/oasisprotocol/oasis-core/go/common"

	cmdCommon "github.com/oasisprotocol/nexus/cmd/common"

	beacon "github.com/oasisprotocol/nexus/coreapi/v22.2.11/beacon/api"
	consensus "github.com/oasisprotocol/nexus/coreapi/v22.2.11/consensus/api"
	genesis "github.com/oasisprotocol/nexus/coreapi/v22.2.11/genesis/api"
	roothash "github.com/oasisprotocol/nexus/coreapi/v22.2.11/roothash/api"

	"github.com/oasisprotocol/nexus/cache/kvstore"
	"github.com/oasisprotocol/nexus/common"
	"github.com/oasisprotocol/nexus/metrics"
	"github.com/oasisprotocol/nexus/storage/oasis/connections"
	"github.com/oasisprotocol/nexus/storage/oasis/nodeapi"
)

// FileConsensusApiLite provides access to the consensus API of an Oasis node.
// Since FileConsensusApiLite is backed by a file containing the cached responses
// to `ConsensusApiLite` calls, this data is inherently compatible with the
// current Nexus and can thus handle heights from both Cobalt/Damask.
type FileConsensusApiLite struct {
	db           kvstore.KVStore
	consensusApi nodeapi.ConsensusApiLite
}

var _ nodeapi.ConsensusApiLite = (*FileConsensusApiLite)(nil)

func NewFileConsensusApiLite(cacheDir string, consensusApi nodeapi.ConsensusApiLite) (*FileConsensusApiLite, error) {
	db, err := kvstore.OpenKVStore(
		cmdCommon.RootLogger().WithModule("file-consensus-api-lite").With("runtime", "consensus"),
		cacheDir,
		common.Ptr(metrics.NewDefaultAnalysisMetrics("consensus")),
	)
	if err != nil {
		return nil, err
	}
	return &FileConsensusApiLite{
		db:           db,
		consensusApi: consensusApi,
	}, nil
}

func (c *FileConsensusApiLite) Close() error {
	// Close all resources and return the first encountered error, if any.
	var firstErr error
	if c.consensusApi != nil {
		firstErr = c.consensusApi.Close()
	}
	if err := c.db.Close(); err != nil && firstErr == nil {
		firstErr = err
	}
	return firstErr
}

func (c *FileConsensusApiLite) GetGenesisDocument(ctx context.Context, chainContext string) (*genesis.Document, error) {
	return kvstore.GetFromCacheOrCall(
		c.db, false,
		// v3: Added proposal invalid votes.
		kvstore.GenerateCacheKey("GetGenesisDocument.v3", chainContext),
		func() (*genesis.Document, error) { return c.consensusApi.GetGenesisDocument(ctx, chainContext) },
	)
}

func (c *FileConsensusApiLite) StateToGenesis(ctx context.Context, height int64) (*genesis.Document, error) {
	return kvstore.GetFromCacheOrCall(
		c.db, height == consensus.HeightLatest,
		// v3: Added proposal invalid votes.
		kvstore.GenerateCacheKey("StateToGenesis.v3", height),
		func() (*genesis.Document, error) { return c.consensusApi.StateToGenesis(ctx, height) },
	)
}

func (c *FileConsensusApiLite) GetConsensusParameters(ctx context.Context, height int64) (*nodeapi.ConsensusParameters, error) {
	return kvstore.GetFromCacheOrCall(
		c.db, height == consensus.HeightLatest,
		// v2: Added max block size.
		kvstore.GenerateCacheKey("ConsensusParameters.v2", height),
		func() (*nodeapi.ConsensusParameters, error) {
			return c.consensusApi.GetConsensusParameters(ctx, height)
		},
	)
}

func (c *FileConsensusApiLite) GetBlock(ctx context.Context, height int64) (*consensus.Block, error) {
	return kvstore.GetFromCacheOrCall(
		c.db, height == consensus.HeightLatest,
		kvstore.GenerateCacheKey("GetBlock", height),
		func() (*consensus.Block, error) { return c.consensusApi.GetBlock(ctx, height) },
	)
}

func (c *FileConsensusApiLite) GetTransactionsWithResults(ctx context.Context, height int64) ([]nodeapi.TransactionWithResults, error) {
	return kvstore.GetSliceFromCacheOrCall(
		c.db, height == consensus.HeightLatest,
		// v3: Updated roothash events conversion to retain roothash messages.
		kvstore.GenerateCacheKey("GetTransactionsWithResults.v3", height),
		func() ([]nodeapi.TransactionWithResults, error) {
			return c.consensusApi.GetTransactionsWithResults(ctx, height)
		},
	)
}

func (c *FileConsensusApiLite) GetEpoch(ctx context.Context, height int64) (beacon.EpochTime, error) {
	time, err := kvstore.GetFromCacheOrCall(
		c.db, height == consensus.HeightLatest,
		kvstore.GenerateCacheKey("GetEpoch", height),
		func() (*beacon.EpochTime, error) {
			time, err := c.consensusApi.GetEpoch(ctx, height)
			return &time, err
		},
	)
	if err != nil {
		return beacon.EpochInvalid, err
	}
	return *time, nil
}

func (c *FileConsensusApiLite) RegistryEvents(ctx context.Context, height int64) ([]nodeapi.Event, error) {
	return kvstore.GetSliceFromCacheOrCall(
		c.db, height == consensus.HeightLatest,
		kvstore.GenerateCacheKey("RegistryEvents", height),
		func() ([]nodeapi.Event, error) { return c.consensusApi.RegistryEvents(ctx, height) },
	)
}

func (c *FileConsensusApiLite) StakingEvents(ctx context.Context, height int64) ([]nodeapi.Event, error) {
	return kvstore.GetSliceFromCacheOrCall(
		c.db, height == consensus.HeightLatest,
		kvstore.GenerateCacheKey("StakingEvents", height),
		func() ([]nodeapi.Event, error) { return c.consensusApi.StakingEvents(ctx, height) },
	)
}

func (c *FileConsensusApiLite) GovernanceEvents(ctx context.Context, height int64) ([]nodeapi.Event, error) {
	return kvstore.GetSliceFromCacheOrCall(
		c.db, height == consensus.HeightLatest,
		kvstore.GenerateCacheKey("GovernanceEvents", height),
		func() ([]nodeapi.Event, error) { return c.consensusApi.GovernanceEvents(ctx, height) },
	)
}

func (c *FileConsensusApiLite) RoothashEvents(ctx context.Context, height int64) ([]nodeapi.Event, error) {
	return kvstore.GetSliceFromCacheOrCall(
		c.db, height == consensus.HeightLatest,
		// v3: Updated roothash events conversion to retain roothash messages.
		kvstore.GenerateCacheKey("RoothashEvents.v3", height),
		func() ([]nodeapi.Event, error) { return c.consensusApi.RoothashEvents(ctx, height) },
	)
}

func (c *FileConsensusApiLite) RoothashLastRoundResults(ctx context.Context, height int64, runtimeID coreCommon.Namespace) (*roothash.RoundResults, error) {
	return kvstore.GetFromCacheOrCall(
		c.db, height == consensus.HeightLatest,
		kvstore.GenerateCacheKey("RoothashLastRoundResults", height, runtimeID),
		func() (*roothash.RoundResults, error) {
			return c.consensusApi.RoothashLastRoundResults(ctx, height, runtimeID)
		},
	)
}

func (c *FileConsensusApiLite) GetNodes(ctx context.Context, height int64) ([]nodeapi.Node, error) {
	return kvstore.GetSliceFromCacheOrCall(
		c.db, height == consensus.HeightLatest,
		kvstore.GenerateCacheKey("GetNodes", height),
		func() ([]nodeapi.Node, error) { return c.consensusApi.GetNodes(ctx, height) },
	)
}

func (c *FileConsensusApiLite) GetValidators(ctx context.Context, height int64) ([]nodeapi.Validator, error) {
	return kvstore.GetSliceFromCacheOrCall(
		c.db, height == consensus.HeightLatest,
		kvstore.GenerateCacheKey("GetValidators", height),
		func() ([]nodeapi.Validator, error) { return c.consensusApi.GetValidators(ctx, height) },
	)
}

func (c *FileConsensusApiLite) GetCommittees(ctx context.Context, height int64, runtimeID coreCommon.Namespace) ([]nodeapi.Committee, error) {
	return kvstore.GetSliceFromCacheOrCall(
		c.db, height == consensus.HeightLatest,
		kvstore.GenerateCacheKey("GetCommittee", height, runtimeID),
		func() ([]nodeapi.Committee, error) { return c.consensusApi.GetCommittees(ctx, height, runtimeID) },
	)
}

func (c *FileConsensusApiLite) GetProposal(ctx context.Context, height int64, proposalID uint64) (*nodeapi.Proposal, error) {
	return kvstore.GetFromCacheOrCall(
		c.db, height == consensus.HeightLatest,
		kvstore.GenerateCacheKey("GetProposal", height, proposalID),
		func() (*nodeapi.Proposal, error) { return c.consensusApi.GetProposal(ctx, height, proposalID) },
	)
}

func (c *FileConsensusApiLite) GrpcConn() connections.GrpcConn {
	return c.consensusApi.GrpcConn()
}
