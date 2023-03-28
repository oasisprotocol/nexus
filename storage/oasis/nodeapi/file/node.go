package file

import (
	"bytes"
	"context"
	"encoding/gob"
	"fmt"

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
	db        pogreb.DB
	damaskApi *damask.DamaskConsensusApiLite
}

type ConsensusApiMethod func() (interface{}, error)

var _ nodeapi.ConsensusApiLite = (*FileConsensusApiLite)(nil)

func NewFileConsensusApiLite(filename string, client *consensus.ClientBackend) (*FileConsensusApiLite, error) {
	db, err := pogreb.Open(filename, &pogreb.Options{BackgroundSyncInterval: -1})
	if err != nil {
		return nil, err
	}
	var damaskApi *damask.DamaskConsensusApiLite
	if client != nil {
		damaskApi = damask.NewDamaskConsensusApiLite(*client)
	}
	return &FileConsensusApiLite{
		db:        *db,
		damaskApi: damaskApi,
	}, nil
}

func generateCacheKey(methodName string, params ...interface{}) ([]byte, error) {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	err := enc.Encode(methodName)
	if err != nil {
		return nil, err
	}
	for _, p := range params {
		err = enc.Encode(p)
		if err != nil {
			return nil, err
		}
	}
	return buf.Bytes(), nil
}

func (c *FileConsensusApiLite) get(key []byte, result interface{}) error {
	res, err := c.db.Get(key)
	if err != nil {
		return err
	}
	dec := gob.NewDecoder(bytes.NewReader(res))
	return dec.Decode(result)
}

func (c *FileConsensusApiLite) put(key []byte, val interface{}) error {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	err := enc.Encode(val)
	if err != nil {
		fmt.Printf("file backend: error encoding value for put " + err.Error())
		return err
	}
	return c.db.Put(key, buf.Bytes())
}

func (c *FileConsensusApiLite) updateCache(key []byte, method ConsensusApiMethod) error {
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
	key, err := generateCacheKey("GetGenesisDocument")
	if err != nil {
		return nil, err
	}
	if c.damaskApi != nil {
		fmt.Printf("file backend: fetching genesis")
		if err := c.updateCache(key, func() (interface{}, error) { return c.damaskApi.GetGenesisDocument(ctx) }); err != nil {
			fmt.Printf("file backend: error fetching genesis")
			return nil, err
		}
	}
	var genesisDocument genesis.Document
	fmt.Printf("file backend: about to get")
	err = c.get(key, &genesisDocument)
	if err != nil {
		fmt.Printf("file backend: error getting genesis back from pogreb")
		return nil, err
	}
	return &genesisDocument, nil
}

func (c *FileConsensusApiLite) StateToGenesis(ctx context.Context, height int64) (*genesis.Document, error) {
	key, err := generateCacheKey("StateToGenesis", height)
	if err != nil {
		return nil, err
	}
	if c.damaskApi != nil {
		c.updateCache(key, func() (interface{}, error) { return c.damaskApi.StateToGenesis(ctx, height) })
	}
	var genesisDocument genesis.Document
	err = c.get(key, &genesisDocument)
	if err != nil {
		return nil, err
	}
	return &genesisDocument, nil
}

func (c *FileConsensusApiLite) GetBlock(ctx context.Context, height int64) (*consensus.Block, error) {
	key, err := generateCacheKey("GetBlock", height)
	if err != nil {
		return nil, err
	}
	if c.damaskApi != nil {
		c.updateCache(key, func() (interface{}, error) { return c.damaskApi.GetBlock(ctx, height) })
	}
	var block consensus.Block
	err = c.get(key, &block)
	if err != nil {
		return nil, err
	}
	return &block, nil
}

func (c *FileConsensusApiLite) GetTransactionsWithResults(ctx context.Context, height int64) ([]nodeapi.TransactionWithResults, error) {
	key, err := generateCacheKey("GetTransactionsWithResults", height)
	if err != nil {
		return nil, err
	}
	if c.damaskApi != nil {
		c.updateCache(key, func() (interface{}, error) { return c.damaskApi.GetTransactionsWithResults(ctx, height) })
	}
	txs := []nodeapi.TransactionWithResults{}
	err = c.get(key, &txs) // TODO: is the & necessary?
	if err != nil {
		return nil, err
	}
	return txs, nil
}

func (c *FileConsensusApiLite) GetEpoch(ctx context.Context, height int64) (beacon.EpochTime, error) {
	key, err := generateCacheKey("GetEpoch", height)
	if err != nil {
		return beacon.EpochInvalid, err
	}
	if c.damaskApi != nil {
		c.updateCache(key, func() (interface{}, error) { return c.damaskApi.GetEpoch(ctx, height) })
	}
	var epoch beacon.EpochTime
	err = c.get(key, &epoch)
	if err != nil {
		return beacon.EpochInvalid, err
	}
	return epoch, nil
}

func (c *FileConsensusApiLite) RegistryEvents(ctx context.Context, height int64) ([]nodeapi.Event, error) {
	key, err := generateCacheKey("RegistryEvents", height)
	if err != nil {
		return nil, err
	}
	if c.damaskApi != nil {
		c.updateCache(key, func() (interface{}, error) { return c.damaskApi.RegistryEvents(ctx, height) })
	}
	events := []nodeapi.Event{}
	err = c.get(key, &events)
	if err != nil {
		return nil, err
	}
	return events, nil
}

func (c *FileConsensusApiLite) StakingEvents(ctx context.Context, height int64) ([]nodeapi.Event, error) {
	key, err := generateCacheKey("StakingEvents", height)
	if err != nil {
		return nil, err
	}
	if c.damaskApi != nil {
		c.updateCache(key, func() (interface{}, error) { return c.damaskApi.StakingEvents(ctx, height) })
	}
	events := []nodeapi.Event{}
	err = c.get(key, &events)
	if err != nil {
		return nil, err
	}
	return events, nil
}

func (c *FileConsensusApiLite) GovernanceEvents(ctx context.Context, height int64) ([]nodeapi.Event, error) {
	key, err := generateCacheKey("GovernanceEvents", height)
	if err != nil {
		return nil, err
	}
	if c.damaskApi != nil {
		c.updateCache(key, func() (interface{}, error) { return c.damaskApi.GovernanceEvents(ctx, height) })
	}
	events := []nodeapi.Event{}
	err = c.get(key, &events)
	if err != nil {
		return nil, err
	}
	return events, nil
}

func (c *FileConsensusApiLite) RoothashEvents(ctx context.Context, height int64) ([]nodeapi.Event, error) {
	key, err := generateCacheKey("RoothashEvents", height)
	if err != nil {
		return nil, err
	}
	if c.damaskApi != nil {
		c.updateCache(key, func() (interface{}, error) { return c.damaskApi.RoothashEvents(ctx, height) })
	}
	events := []nodeapi.Event{}
	err = c.get(key, &events)
	if err != nil {
		return nil, err
	}
	return events, nil
}

func (c *FileConsensusApiLite) GetValidators(ctx context.Context, height int64) ([]nodeapi.Validator, error) {
	key, err := generateCacheKey("GetValidators", height)
	if err != nil {
		return nil, err
	}
	if c.damaskApi != nil {
		c.updateCache(key, func() (interface{}, error) { return c.damaskApi.GetValidators(ctx, height) })
	}
	validators := []nodeapi.Validator{}
	err = c.get(key, &validators)
	if err != nil {
		return nil, err
	}
	return validators, nil
}

func (c *FileConsensusApiLite) GetCommittees(ctx context.Context, height int64, runtimeID coreCommon.Namespace) ([]nodeapi.Committee, error) {
	key, err := generateCacheKey("GetCommittees", height, runtimeID)
	if err != nil {
		return nil, err
	}
	if c.damaskApi != nil {
		c.updateCache(key, func() (interface{}, error) { return c.damaskApi.GetCommittees(ctx, height, runtimeID) })
	}
	committees := []nodeapi.Committee{}
	err = c.get(key, &committees)
	if err != nil {
		return nil, err
	}
	return committees, nil
}

func (c *FileConsensusApiLite) GetProposal(ctx context.Context, height int64, proposalID uint64) (*nodeapi.Proposal, error) {
	key, err := generateCacheKey("GetProposal", height, proposalID)
	if err != nil {
		return nil, err
	}
	if c.damaskApi != nil {
		c.updateCache(key, func() (interface{}, error) { return c.damaskApi.GetProposal(ctx, height, proposalID) })
	}
	var proposal nodeapi.Proposal
	err = c.get(key, &proposal)
	if err != nil {
		return nil, err
	}
	return &proposal, nil
}
