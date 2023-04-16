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

func (c *FileConsensusApiLite) updateCache(key []byte, method NodeApiMethod) error {
	exists, err := c.db.Has(key)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	val, err := method()
	if err != nil {
		return err
	}

	return c.db.Put(key, cbor.Marshal(val))
}

func (c *FileConsensusApiLite) GetGenesisDocument(ctx context.Context) (*genesis.Document, error) {
	key := generateCacheKey("GetGenesisDocument")
	if c.consensusApi != nil {
		if err := c.updateCache(key, func() (interface{}, error) { return c.consensusApi.GetGenesisDocument(ctx) }); err != nil {
			return nil, err
		}
	}
	var genesisDocument genesis.Document
	raw, err := c.db.Get(key)
	if err != nil {
		return nil, err
	}
	err = cbor.Unmarshal(raw, &genesisDocument)
	if err != nil {
		return nil, err
	}
	return &genesisDocument, nil
}

func (c *FileConsensusApiLite) StateToGenesis(ctx context.Context, height int64) (*genesis.Document, error) {
	key := generateCacheKey("StateToGenesis", height)
	if c.consensusApi != nil {
		if err := c.updateCache(key, func() (interface{}, error) { return c.consensusApi.StateToGenesis(ctx, height) }); err != nil {
			return nil, err
		}
	}
	var genesisDocument genesis.Document
	raw, err := c.db.Get(key)
	if err != nil {
		return nil, err
	}
	err = cbor.Unmarshal(raw, &genesisDocument)
	if err != nil {
		return nil, err
	}
	return &genesisDocument, nil
}

func (c *FileConsensusApiLite) GetBlock(ctx context.Context, height int64) (*consensus.Block, error) {
	key := generateCacheKey("GetBlock", height)
	if c.consensusApi != nil {
		if err := c.updateCache(key, func() (interface{}, error) { return c.consensusApi.GetBlock(ctx, height) }); err != nil {
			return nil, err
		}
	}
	var block consensus.Block
	raw, err := c.db.Get(key)
	if err != nil {
		return nil, err
	}
	err = cbor.Unmarshal(raw, &block)
	if err != nil {
		return nil, err
	}
	return &block, nil
}

func (c *FileConsensusApiLite) GetTransactionsWithResults(ctx context.Context, height int64) ([]nodeapi.TransactionWithResults, error) {
	key := generateCacheKey("GetTransactionsWithResults", height)
	if c.consensusApi != nil {
		if err := c.updateCache(key, func() (interface{}, error) { return c.consensusApi.GetTransactionsWithResults(ctx, height) }); err != nil {
			return nil, err
		}
	}
	txs := []nodeapi.TransactionWithResults{}
	raw, err := c.db.Get(key)
	if err != nil {
		return nil, err
	}
	err = cbor.Unmarshal(raw, &txs)
	if err != nil {
		return nil, err
	}
	return txs, nil
}

func (c *FileConsensusApiLite) GetEpoch(ctx context.Context, height int64) (beacon.EpochTime, error) {
	key := generateCacheKey("GetEpoch", height)
	if c.consensusApi != nil {
		if err := c.updateCache(key, func() (interface{}, error) { return c.consensusApi.GetEpoch(ctx, height) }); err != nil {
			return beacon.EpochInvalid, err
		}
	}
	var epoch beacon.EpochTime
	raw, err := c.db.Get(key)
	if err != nil {
		return beacon.EpochInvalid, err
	}
	err = cbor.Unmarshal(raw, &epoch)
	if err != nil {
		return beacon.EpochInvalid, err
	}
	return epoch, nil
}

func (c *FileConsensusApiLite) RegistryEvents(ctx context.Context, height int64) ([]nodeapi.Event, error) {
	key := generateCacheKey("RegistryEvents", height)
	if c.consensusApi != nil {
		if err := c.updateCache(key, func() (interface{}, error) { return c.consensusApi.RegistryEvents(ctx, height) }); err != nil {
			return nil, err
		}
	}
	events := []nodeapi.Event{}
	raw, err := c.db.Get(key)
	if err != nil {
		return nil, err
	}
	err = cbor.Unmarshal(raw, &events)
	if err != nil {
		return nil, err
	}
	return events, nil
}

func (c *FileConsensusApiLite) StakingEvents(ctx context.Context, height int64) ([]nodeapi.Event, error) {
	key := generateCacheKey("StakingEvents", height)
	if c.consensusApi != nil {
		if err := c.updateCache(key, func() (interface{}, error) { return c.consensusApi.StakingEvents(ctx, height) }); err != nil {
			return nil, err
		}
	}
	events := []nodeapi.Event{}
	raw, err := c.db.Get(key)
	if err != nil {
		return nil, err
	}
	err = cbor.Unmarshal(raw, &events)
	if err != nil {
		return nil, err
	}
	return events, nil
}

func (c *FileConsensusApiLite) GovernanceEvents(ctx context.Context, height int64) ([]nodeapi.Event, error) {
	key := generateCacheKey("GovernanceEvents", height)
	if c.consensusApi != nil {
		if err := c.updateCache(key, func() (interface{}, error) { return c.consensusApi.GovernanceEvents(ctx, height) }); err != nil {
			return nil, err
		}
	}
	events := []nodeapi.Event{}
	raw, err := c.db.Get(key)
	if err != nil {
		return nil, err
	}
	err = cbor.Unmarshal(raw, &events)
	if err != nil {
		return nil, err
	}
	return events, nil
}

func (c *FileConsensusApiLite) RoothashEvents(ctx context.Context, height int64) ([]nodeapi.Event, error) {
	key := generateCacheKey("RoothashEvents", height)
	if c.consensusApi != nil {
		if err := c.updateCache(key, func() (interface{}, error) { return c.consensusApi.RoothashEvents(ctx, height) }); err != nil {
			return nil, err
		}
	}
	events := []nodeapi.Event{}
	raw, err := c.db.Get(key)
	if err != nil {
		return nil, err
	}
	err = cbor.Unmarshal(raw, &events)
	if err != nil {
		return nil, err
	}
	return events, nil
}

func (c *FileConsensusApiLite) GetValidators(ctx context.Context, height int64) ([]nodeapi.Validator, error) {
	key := generateCacheKey("GetValidators", height)
	if c.consensusApi != nil {
		if err := c.updateCache(key, func() (interface{}, error) { return c.consensusApi.GetValidators(ctx, height) }); err != nil {
			return nil, err
		}
	}
	validators := []nodeapi.Validator{}
	raw, err := c.db.Get(key)
	if err != nil {
		return nil, err
	}
	err = cbor.Unmarshal(raw, &validators)
	if err != nil {
		return nil, err
	}
	return validators, nil
}

func (c *FileConsensusApiLite) GetCommittees(ctx context.Context, height int64, runtimeID coreCommon.Namespace) ([]nodeapi.Committee, error) {
	key := generateCacheKey("GetCommittees", height, runtimeID)
	if c.consensusApi != nil {
		if err := c.updateCache(key, func() (interface{}, error) { return c.consensusApi.GetCommittees(ctx, height, runtimeID) }); err != nil {
			return nil, err
		}
	}
	committees := []nodeapi.Committee{}
	raw, err := c.db.Get(key)
	if err != nil {
		return nil, err
	}
	err = cbor.Unmarshal(raw, &committees)
	if err != nil {
		return nil, err
	}
	return committees, nil
}

func (c *FileConsensusApiLite) GetProposal(ctx context.Context, height int64, proposalID uint64) (*nodeapi.Proposal, error) {
	key := generateCacheKey("GetProposal", height, proposalID)
	if c.consensusApi != nil {
		if err := c.updateCache(key, func() (interface{}, error) { return c.consensusApi.GetProposal(ctx, height, proposalID) }); err != nil {
			return nil, err
		}
	}
	var proposal nodeapi.Proposal
	raw, err := c.db.Get(key)
	if err != nil {
		return nil, err
	}
	err = cbor.Unmarshal(raw, &proposal)
	if err != nil {
		return nil, err
	}
	return &proposal, nil
}
