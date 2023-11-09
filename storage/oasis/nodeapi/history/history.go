package history

import (
	"context"
	"fmt"

	"github.com/oasisprotocol/oasis-core/go/common"

	beacon "github.com/oasisprotocol/nexus/coreapi/v22.2.11/beacon/api"
	consensus "github.com/oasisprotocol/nexus/coreapi/v22.2.11/consensus/api"
	genesis "github.com/oasisprotocol/nexus/coreapi/v22.2.11/genesis/api"

	"github.com/oasisprotocol/nexus/config"
	"github.com/oasisprotocol/nexus/storage/oasis/connections"
	"github.com/oasisprotocol/nexus/storage/oasis/nodeapi"
	"github.com/oasisprotocol/nexus/storage/oasis/nodeapi/cobalt"
	"github.com/oasisprotocol/nexus/storage/oasis/nodeapi/damask"
	"github.com/oasisprotocol/nexus/storage/oasis/nodeapi/eden"
)

var _ nodeapi.ConsensusApiLite = (*HistoryConsensusApiLite)(nil)

type APIConstructor func(ctx context.Context, chainContext string, archiveConfig *config.ArchiveConfig, fastStartup bool) (nodeapi.ConsensusApiLite, error)

func damaskAPIConstructor(ctx context.Context, chainContext string, archiveConfig *config.ArchiveConfig, fastStartup bool) (nodeapi.ConsensusApiLite, error) {
	rawConn := connections.NewLazyGrpcConn(*archiveConfig.ResolvedConsensusNode())
	return damask.NewConsensusApiLite(rawConn), nil
}

func cobaltAPIConstructor(ctx context.Context, chainContext string, archiveConfig *config.ArchiveConfig, fastStartup bool) (nodeapi.ConsensusApiLite, error) {
	rawConn := connections.NewLazyGrpcConn(*archiveConfig.ResolvedConsensusNode())
	return cobalt.NewConsensusApiLite(rawConn), nil
}

func edenAPIConstructor(ctx context.Context, chainContext string, archiveConfig *config.ArchiveConfig, fastStartup bool) (nodeapi.ConsensusApiLite, error) {
	rawConn := connections.NewLazyGrpcConn(*archiveConfig.ResolvedConsensusNode())
	return eden.NewConsensusApiLite(rawConn), nil
}

// APIConstructors map each (nexus-internal) archive name to the API constructor
// that can talk to that archive. The namespace of archive names is shared
// between mainnet and testnet for simplicity.
// The supported archive names come from `config.DefaultChains`. If you want to use
// a custom set of archives (`custom_chain` in the yaml config), you must reuse
// a suitable archive name.
var APIConstructors = map[string]APIConstructor{
	// mainnet
	"damask": damaskAPIConstructor,
	"cobalt": cobaltAPIConstructor,
	"eden":   edenAPIConstructor,
	// testnet
	"2022-03-03": damaskAPIConstructor,
	"2023-10-12": edenAPIConstructor,
}

type HistoryConsensusApiLite struct {
	history *config.History
	// apis. Keys are "archive names," which are named after mainnet releases,
	// in lowercase e.g. "cobalt" and "damask."
	apis map[string]nodeapi.ConsensusApiLite
}

func NewHistoryConsensusApiLite(ctx context.Context, history *config.History, nodes map[string]*config.ArchiveConfig, fastStartup bool) (*HistoryConsensusApiLite, error) {
	apis := map[string]nodeapi.ConsensusApiLite{}
	for _, record := range history.Records {
		if archiveConfig, ok := nodes[record.ArchiveName]; ok {
			apiConstructor := APIConstructors[record.ArchiveName]
			if apiConstructor == nil {
				return nil, fmt.Errorf("historical API for archive %s not implemented", record.ArchiveName)
			}
			api, err := apiConstructor(ctx, record.ChainContext, archiveConfig, fastStartup)
			if err != nil {
				return nil, fmt.Errorf("connecting to archive %s: %w", record.ArchiveName, err)
			}
			apis[record.ArchiveName] = api
		}
	}
	return &HistoryConsensusApiLite{
		history: history,
		apis:    apis,
	}, nil
}

func (c *HistoryConsensusApiLite) Close() error {
	var firstErr error
	for _, api := range c.apis {
		if err := api.Close(); err != nil && firstErr == nil {
			firstErr = err
			// Do not return yet; keep closing others.
		}
	}
	if firstErr != nil {
		return fmt.Errorf("closing apis failed, first encountered error was: %w", firstErr)
	}
	return nil
}

func (c *HistoryConsensusApiLite) APIForHeight(height int64) (nodeapi.ConsensusApiLite, error) {
	record, err := c.history.RecordForHeight(height)
	if err != nil {
		return nil, fmt.Errorf("dertermining archive: %w", err)
	}
	api, ok := c.apis[record.ArchiveName]
	if !ok {
		return nil, fmt.Errorf("archive %s has no node configured", record.ArchiveName)
	}
	return api, nil
}

func (c *HistoryConsensusApiLite) APIForChainContext(chainContext string) (nodeapi.ConsensusApiLite, error) {
	record, err := c.history.RecordForChainContext(chainContext)
	if err != nil {
		return nil, fmt.Errorf("determining archive: %w", err)
	}
	api, ok := c.apis[record.ArchiveName]
	if !ok {
		return nil, fmt.Errorf("archive %s has no node configured", record.ArchiveName)
	}
	return api, nil
}

func (c *HistoryConsensusApiLite) GetGenesisDocument(ctx context.Context, chainContext string) (*genesis.Document, error) {
	api, err := c.APIForChainContext(chainContext)
	if err != nil {
		return nil, fmt.Errorf("getting api for chain context %s: %w", chainContext, err)
	}
	return api.GetGenesisDocument(ctx, chainContext)
}

func (c *HistoryConsensusApiLite) StateToGenesis(ctx context.Context, height int64) (*genesis.Document, error) {
	api, err := c.APIForHeight(height)
	if err != nil {
		return nil, fmt.Errorf("getting api for height %d: %w", height, err)
	}
	return api.StateToGenesis(ctx, height)
}

func (c *HistoryConsensusApiLite) GetBlock(ctx context.Context, height int64) (*consensus.Block, error) {
	api, err := c.APIForHeight(height)
	if err != nil {
		return nil, fmt.Errorf("getting api for height %d: %w", height, err)
	}
	return api.GetBlock(ctx, height)
}

func (c *HistoryConsensusApiLite) GetTransactionsWithResults(ctx context.Context, height int64) ([]nodeapi.TransactionWithResults, error) {
	api, err := c.APIForHeight(height)
	if err != nil {
		return nil, fmt.Errorf("getting api for height %d: %w", height, err)
	}
	return api.GetTransactionsWithResults(ctx, height)
}

func (c *HistoryConsensusApiLite) GetEpoch(ctx context.Context, height int64) (beacon.EpochTime, error) {
	api, err := c.APIForHeight(height)
	if err != nil {
		return 0, fmt.Errorf("getting api for height %d: %w", height, err)
	}
	return api.GetEpoch(ctx, height)
}

func (c *HistoryConsensusApiLite) RegistryEvents(ctx context.Context, height int64) ([]nodeapi.Event, error) {
	api, err := c.APIForHeight(height)
	if err != nil {
		return nil, fmt.Errorf("getting api for height %d: %w", height, err)
	}
	return api.RegistryEvents(ctx, height)
}

func (c *HistoryConsensusApiLite) StakingEvents(ctx context.Context, height int64) ([]nodeapi.Event, error) {
	api, err := c.APIForHeight(height)
	if err != nil {
		return nil, fmt.Errorf("getting api for height %d: %w", height, err)
	}
	return api.StakingEvents(ctx, height)
}

func (c *HistoryConsensusApiLite) GovernanceEvents(ctx context.Context, height int64) ([]nodeapi.Event, error) {
	api, err := c.APIForHeight(height)
	if err != nil {
		return nil, fmt.Errorf("getting api for height %d: %w", height, err)
	}
	return api.GovernanceEvents(ctx, height)
}

func (c *HistoryConsensusApiLite) RoothashEvents(ctx context.Context, height int64) ([]nodeapi.Event, error) {
	api, err := c.APIForHeight(height)
	if err != nil {
		return nil, fmt.Errorf("getting api for height %d: %w", height, err)
	}
	return api.RoothashEvents(ctx, height)
}

func (c *HistoryConsensusApiLite) GetNodes(ctx context.Context, height int64) ([]nodeapi.Node, error) {
	api, err := c.APIForHeight(height)
	if err != nil {
		return nil, fmt.Errorf("getting api for height %d: %w", height, err)
	}
	return api.GetNodes(ctx, height)
}

func (c *HistoryConsensusApiLite) GetValidators(ctx context.Context, height int64) ([]nodeapi.Validator, error) {
	api, err := c.APIForHeight(height)
	if err != nil {
		return nil, fmt.Errorf("getting api for height %d: %w", height, err)
	}
	return api.GetValidators(ctx, height)
}

func (c *HistoryConsensusApiLite) GetCommittees(ctx context.Context, height int64, runtimeID common.Namespace) ([]nodeapi.Committee, error) {
	api, err := c.APIForHeight(height)
	if err != nil {
		return nil, fmt.Errorf("getting api for height %d: %w", height, err)
	}
	return api.GetCommittees(ctx, height, runtimeID)
}

func (c *HistoryConsensusApiLite) GetProposal(ctx context.Context, height int64, proposalID uint64) (*nodeapi.Proposal, error) {
	api, err := c.APIForHeight(height)
	if err != nil {
		return nil, fmt.Errorf("getting api for height %d: %w", height, err)
	}
	return api.GetProposal(ctx, height, proposalID)
}
