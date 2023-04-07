package file

import (
	"context"
	"encoding/json"

	"github.com/akrylysov/pogreb"
	beacon "github.com/oasisprotocol/oasis-core/go/beacon/api"
	coreCommon "github.com/oasisprotocol/oasis-core/go/common"
	consensus "github.com/oasisprotocol/oasis-core/go/consensus/api"
	genesis "github.com/oasisprotocol/oasis-core/go/genesis/api"

	"github.com/oasisprotocol/oasis-indexer/storage/oasis/nodeapi"
	"github.com/oasisprotocol/oasis-indexer/storage/oasis/nodeapi/damask"
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

func NewFileConsensusApiLite(filename string, client *consensus.ClientBackend) (*FileConsensusApiLite, error) {
	db, err := pogreb.Open(filename, &pogreb.Options{BackgroundSyncInterval: -1})
	if err != nil {
		return nil, err
	}
	var consensusApi nodeapi.ConsensusApiLite
	if client != nil {
		consensusApi = damask.NewDamaskConsensusApiLite(*client)
	}
	return &FileConsensusApiLite{
		db:           *db,
		consensusApi: consensusApi,
	}, nil
}

func (c *FileConsensusApiLite) get(key []byte, result interface{}) error {
	res, err := c.db.Get(key)
	if err != nil {
		return err
	}
	return json.Unmarshal(res, result)
}

func (c *FileConsensusApiLite) put(key []byte, val interface{}) error {
	valBytes, err := json.Marshal(val)
	if err != nil {
		return err
	}
	return c.db.Put(key, valBytes)
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

	return c.put(key, val)
}

func (c *FileConsensusApiLite) GetGenesisDocument(ctx context.Context) (*genesis.Document, error) {
	key := generateCacheKey("GetGenesisDocument")
	if c.consensusApi != nil {
		if err := c.updateCache(key, func() (interface{}, error) { return c.consensusApi.GetGenesisDocument(ctx) }); err != nil {
			return nil, err
		}
	}
	var genesisDocument genesis.Document
	err := c.get(key, &genesisDocument)
	if err != nil {
		return nil, err
	}
	return &genesisDocument, nil
}

func (c *FileConsensusApiLite) StateToGenesis(ctx context.Context, height int64) (*genesis.Document, error) {
	key := generateCacheKey("StateToGenesis", height)
	if c.consensusApi != nil {
		c.updateCache(key, func() (interface{}, error) { return c.consensusApi.StateToGenesis(ctx, height) })
	}
	var genesisDocument genesis.Document
	err := c.get(key, &genesisDocument)
	if err != nil {
		return nil, err
	}
	return &genesisDocument, nil
}

func (c *FileConsensusApiLite) GetBlock(ctx context.Context, height int64) (*consensus.Block, error) {
	key := generateCacheKey("GetBlock", height)
	if c.consensusApi != nil {
		c.updateCache(key, func() (interface{}, error) { return c.consensusApi.GetBlock(ctx, height) })
	}
	var block consensus.Block
	err := c.get(key, &block)
	if err != nil {
		return nil, err
	}
	return &block, nil
}

func (c *FileConsensusApiLite) GetTransactionsWithResults(ctx context.Context, height int64) ([]nodeapi.TransactionWithResults, error) {
	key := generateCacheKey("GetTransactionsWithResults", height)
	if c.consensusApi != nil {
		c.updateCache(key, func() (interface{}, error) { return c.consensusApi.GetTransactionsWithResults(ctx, height) })
	}
	txs := []nodeapi.TransactionWithResults{}
	err := c.get(key, &txs) // TODO: is the & necessary?
	if err != nil {
		return nil, err
	}
	return txs, nil
}

func (c *FileConsensusApiLite) GetEpoch(ctx context.Context, height int64) (beacon.EpochTime, error) {
	key := generateCacheKey("GetEpoch", height)
	if c.consensusApi != nil {
		c.updateCache(key, func() (interface{}, error) { return c.consensusApi.GetEpoch(ctx, height) })
	}
	var epoch beacon.EpochTime
	err := c.get(key, &epoch)
	if err != nil {
		return beacon.EpochInvalid, err
	}
	return epoch, nil
}

func (c *FileConsensusApiLite) RegistryEvents(ctx context.Context, height int64) ([]nodeapi.Event, error) {
	key := generateCacheKey("RegistryEvents", height)
	if c.consensusApi != nil {
		c.updateCache(key, func() (interface{}, error) { return c.consensusApi.RegistryEvents(ctx, height) })
	}
	events := []nodeapi.Event{}
	err := c.get(key, &events)
	if err != nil {
		return nil, err
	}
	return events, nil
}

func (c *FileConsensusApiLite) StakingEvents(ctx context.Context, height int64) ([]nodeapi.Event, error) {
	key := generateCacheKey("StakingEvents", height)
	if c.consensusApi != nil {
		c.updateCache(key, func() (interface{}, error) { return c.consensusApi.StakingEvents(ctx, height) })
	}
	events := []nodeapi.Event{}
	err := c.get(key, &events)
	if err != nil {
		return nil, err
	}
	return events, nil
}

func (c *FileConsensusApiLite) GovernanceEvents(ctx context.Context, height int64) ([]nodeapi.Event, error) {
	key := generateCacheKey("GovernanceEvents", height)
	if c.consensusApi != nil {
		c.updateCache(key, func() (interface{}, error) { return c.consensusApi.GovernanceEvents(ctx, height) })
	}
	events := []nodeapi.Event{}
	err := c.get(key, &events)
	if err != nil {
		return nil, err
	}
	return events, nil
}

func (c *FileConsensusApiLite) RoothashEvents(ctx context.Context, height int64) ([]nodeapi.Event, error) {
	key := generateCacheKey("RoothashEvents", height)
	if c.consensusApi != nil {
		c.updateCache(key, func() (interface{}, error) { return c.consensusApi.RoothashEvents(ctx, height) })
	}
	events := []nodeapi.Event{}
	err := c.get(key, &events)
	if err != nil {
		return nil, err
	}
	return events, nil
}

func (c *FileConsensusApiLite) GetValidators(ctx context.Context, height int64) ([]nodeapi.Validator, error) {
	key := generateCacheKey("GetValidators", height)
	if c.consensusApi != nil {
		c.updateCache(key, func() (interface{}, error) { return c.consensusApi.GetValidators(ctx, height) })
	}
	validators := []nodeapi.Validator{}
	err := c.get(key, &validators)
	if err != nil {
		return nil, err
	}
	return validators, nil
}

func (c *FileConsensusApiLite) GetCommittees(ctx context.Context, height int64, runtimeID coreCommon.Namespace) ([]nodeapi.Committee, error) {
	key := generateCacheKey("GetCommittees", height, runtimeID)
	if c.consensusApi != nil {
		c.updateCache(key, func() (interface{}, error) { return c.consensusApi.GetCommittees(ctx, height, runtimeID) })
	}
	committees := []nodeapi.Committee{}
	err := c.get(key, &committees)
	if err != nil {
		return nil, err
	}
	return committees, nil
}

func (c *FileConsensusApiLite) GetProposal(ctx context.Context, height int64, proposalID uint64) (*nodeapi.Proposal, error) {
	key := generateCacheKey("GetProposal", height, proposalID)
	if c.consensusApi != nil {
		c.updateCache(key, func() (interface{}, error) { return c.consensusApi.GetProposal(ctx, height, proposalID) })
	}
	var proposal nodeapi.Proposal
	err := c.get(key, &proposal)
	if err != nil {
		return nil, err
	}
	return &proposal, nil
}
