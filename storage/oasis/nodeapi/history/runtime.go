package history

import (
	"context"
	"fmt"

	sdkConfig "github.com/oasisprotocol/oasis-sdk/client-sdk/go/config"
	sdkTypes "github.com/oasisprotocol/oasis-sdk/client-sdk/go/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/oasisprotocol/nexus/common"
	"github.com/oasisprotocol/nexus/config"
	"github.com/oasisprotocol/nexus/storage/oasis/connections"
	"github.com/oasisprotocol/nexus/storage/oasis/nodeapi"
)

var _ nodeapi.RuntimeApiLite = (*HistoryRuntimeApiLite)(nil)

type HistoryRuntimeApiLite struct {
	Runtime common.Runtime
	History *config.History
	APIs    map[string]nodeapi.RuntimeApiLite
}

func NewHistoryRuntimeApiLite(ctx context.Context, history *config.History, sdkPT *sdkConfig.ParaTime, nodes map[string]*config.ArchiveConfig, fastStartup bool, runtime common.Runtime) (*HistoryRuntimeApiLite, error) {
	if history == nil {
		return nil, fmt.Errorf("history config not provided")
	}
	apis := map[string]nodeapi.RuntimeApiLite{}
	for _, record := range history.Records {
		if archiveConfig, ok := nodes[record.ArchiveName]; ok {
			// If sdkPT is nil, the subsequent sdkConn.Runtime() call will panic
			if sdkPT == nil {
				return nil, fmt.Errorf("no paratime specified")
			}
			sdkConn, err := connections.SDKConnect(ctx, record.ChainContext, archiveConfig.ResolvedRuntimeNode(runtime), fastStartup)
			if err != nil {
				return nil, err
			}
			sdkClient := sdkConn.Runtime(sdkPT)
			rawConn := connections.NewLazyGrpcConn(*archiveConfig.ResolvedRuntimeNode(runtime))
			apis[record.ArchiveName] = nodeapi.NewUniversalRuntimeApiLite(sdkPT.Namespace(), rawConn, &sdkClient)
		}
	}
	return &HistoryRuntimeApiLite{
		Runtime: runtime,
		History: history,
		APIs:    apis,
	}, nil
}

func (rc *HistoryRuntimeApiLite) Close() error {
	var firstErr error
	for _, api := range rc.APIs {
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

func (rc *HistoryRuntimeApiLite) APIForRound(round uint64) (nodeapi.RuntimeApiLite, error) {
	record, err := rc.History.RecordForRuntimeRound(rc.Runtime, round)
	if err != nil {
		return nil, fmt.Errorf("determining archive: %w", err)
	}
	api, ok := rc.APIs[record.ArchiveName]
	if !ok {
		return nil, fmt.Errorf("archive %s has no node configured", record.ArchiveName)
	}
	return api, nil
}

func (rc *HistoryRuntimeApiLite) GetEventsRaw(ctx context.Context, round uint64) ([]nodeapi.RuntimeEvent, error) {
	api, err := rc.APIForRound(round)
	if err != nil {
		return nil, fmt.Errorf("getting api for runtime %s round %d: %w", rc.Runtime, round, err)
	}
	return api.GetEventsRaw(ctx, round)
}

func (rc *HistoryRuntimeApiLite) EVMSimulateCall(ctx context.Context, round uint64, gasPrice []byte, gasLimit uint64, caller []byte, address []byte, value []byte, data []byte) (*nodeapi.FallibleResponse, error) {
	api, err := rc.APIForRound(round)
	if err != nil {
		return nil, fmt.Errorf("getting api for runtime %s round %d: %w", rc.Runtime, round, err)
	}
	return api.EVMSimulateCall(ctx, round, gasPrice, gasLimit, caller, address, value, data)
}

func (rc *HistoryRuntimeApiLite) EVMGetCode(ctx context.Context, round uint64, address []byte) ([]byte, error) {
	api, err := rc.APIForRound(round)
	if err != nil {
		return nil, fmt.Errorf("getting api for runtime %s round %d: %w", rc.Runtime, round, err)
	}
	return api.EVMGetCode(ctx, round, address)
}

func (rc *HistoryRuntimeApiLite) GetBlockHeader(ctx context.Context, round uint64) (*nodeapi.RuntimeBlockHeader, error) {
	api, err := rc.APIForRound(round)
	if err != nil {
		return nil, fmt.Errorf("getting api for runtime %s round %d: %w", rc.Runtime, round, err)
	}
	return api.GetBlockHeader(ctx, round)
}

func (rc *HistoryRuntimeApiLite) GetBalances(ctx context.Context, round uint64, addr nodeapi.Address) (map[sdkTypes.Denomination]common.BigInt, error) {
	api, err := rc.APIForRound(round)
	if err != nil {
		return nil, fmt.Errorf("getting api for runtime %s round %d: %w", rc.Runtime, round, err)
	}
	return api.GetBalances(ctx, round, addr)
}

func (rc *HistoryRuntimeApiLite) GetMinGasPrice(ctx context.Context, round uint64) (map[sdkTypes.Denomination]common.BigInt, error) {
	api, err := rc.APIForRound(round)
	if err != nil {
		return nil, fmt.Errorf("getting api for runtime %s round %d: %w", rc.Runtime, round, err)
	}
	minGasPrice, err := api.GetMinGasPrice(ctx, round)
	switch {
	case status.Code(err) == codes.Unimplemented:
		// Some early emerald blocks did not yet have the min-gas price query, ignore possible errors here.
		if rc.Runtime == common.RuntimeEmerald {
			return map[sdkTypes.Denomination]common.BigInt{
				sdkTypes.NativeDenomination: common.NewBigInt(0),
			}, nil
		}
		fallthrough
	case err != nil:
		return nil, err
	default:
	}
	return minGasPrice, nil
}

func (rc *HistoryRuntimeApiLite) GetTransactionsWithResults(ctx context.Context, round uint64) ([]nodeapi.RuntimeTransactionWithResults, error) {
	api, err := rc.APIForRound(round)
	if err != nil {
		return nil, fmt.Errorf("getting api for runtime %s round %d: %w", rc.Runtime, round, err)
	}
	return api.GetTransactionsWithResults(ctx, round)
}

func (rc *HistoryRuntimeApiLite) RoflApp(ctx context.Context, round uint64, id nodeapi.AppID) (*nodeapi.AppConfig, error) {
	api, err := rc.APIForRound(round)
	if err != nil {
		return nil, fmt.Errorf("getting api for runtime %s round %d: %w", rc.Runtime, round, err)
	}
	return api.RoflApp(ctx, round, id)
}

func (rc *HistoryRuntimeApiLite) RoflApps(ctx context.Context, round uint64) ([]*nodeapi.AppConfig, error) {
	api, err := rc.APIForRound(round)
	if err != nil {
		return nil, fmt.Errorf("getting api for runtime %s round %d: %w", rc.Runtime, round, err)
	}
	return api.RoflApps(ctx, round)
}

func (rc *HistoryRuntimeApiLite) RoflAppInstance(ctx context.Context, round uint64, id nodeapi.AppID, rak nodeapi.PublicKey) (*nodeapi.Registration, error) {
	api, err := rc.APIForRound(round)
	if err != nil {
		return nil, fmt.Errorf("getting api for runtime %s round %d: %w", rc.Runtime, round, err)
	}
	return api.RoflAppInstance(ctx, round, id, rak)
}

func (rc *HistoryRuntimeApiLite) RoflAppInstances(ctx context.Context, round uint64, id nodeapi.AppID) ([]*nodeapi.Registration, error) {
	api, err := rc.APIForRound(round)
	if err != nil {
		return nil, fmt.Errorf("getting api for runtime %s round %d: %w", rc.Runtime, round, err)
	}
	return api.RoflAppInstances(ctx, round, id)
}

func (rc *HistoryRuntimeApiLite) RoflMarketProvider(ctx context.Context, round uint64, providerAddress sdkTypes.Address) (*nodeapi.Provider, error) {
	api, err := rc.APIForRound(round)
	if err != nil {
		return nil, fmt.Errorf("getting api for runtime %s round %d: %w", rc.Runtime, round, err)
	}
	return api.RoflMarketProvider(ctx, round, providerAddress)
}

func (rc *HistoryRuntimeApiLite) RoflMarketProviders(ctx context.Context, round uint64) ([]*nodeapi.Provider, error) {
	api, err := rc.APIForRound(round)
	if err != nil {
		return nil, fmt.Errorf("getting api for runtime %s round %d: %w", rc.Runtime, round, err)
	}
	return api.RoflMarketProviders(ctx, round)
}

func (rc *HistoryRuntimeApiLite) RoflMarketOffer(ctx context.Context, round uint64, providerAddress sdkTypes.Address, offerID nodeapi.OfferID) (*nodeapi.Offer, error) {
	api, err := rc.APIForRound(round)
	if err != nil {
		return nil, fmt.Errorf("getting api for runtime %s round %d: %w", rc.Runtime, round, err)
	}
	return api.RoflMarketOffer(ctx, round, providerAddress, offerID)
}

func (rc *HistoryRuntimeApiLite) RoflMarketOffers(ctx context.Context, round uint64, providerAddress sdkTypes.Address) ([]*nodeapi.Offer, error) {
	api, err := rc.APIForRound(round)
	if err != nil {
		return nil, fmt.Errorf("getting api for runtime %s round %d: %w", rc.Runtime, round, err)
	}
	return api.RoflMarketOffers(ctx, round, providerAddress)
}

func (rc *HistoryRuntimeApiLite) RoflMarketInstance(ctx context.Context, round uint64, providerAddress sdkTypes.Address, instanceID nodeapi.InstanceID) (*nodeapi.Instance, error) {
	api, err := rc.APIForRound(round)
	if err != nil {
		return nil, fmt.Errorf("getting api for runtime %s round %d: %w", rc.Runtime, round, err)
	}
	return api.RoflMarketInstance(ctx, round, providerAddress, instanceID)
}

func (rc *HistoryRuntimeApiLite) RoflMarketInstances(ctx context.Context, round uint64, providerAddress sdkTypes.Address) ([]*nodeapi.Instance, error) {
	api, err := rc.APIForRound(round)
	if err != nil {
		return nil, fmt.Errorf("getting api for runtime %s round %d: %w", rc.Runtime, round, err)
	}
	return api.RoflMarketInstances(ctx, round, providerAddress)
}

func (rc *HistoryRuntimeApiLite) RoflMarketInstanceCommands(ctx context.Context, round uint64, providerAddress sdkTypes.Address, instanceID nodeapi.InstanceID) ([]*nodeapi.QueuedCommand, error) {
	api, err := rc.APIForRound(round)
	if err != nil {
		return nil, fmt.Errorf("getting api for runtime %s round %d: %w", rc.Runtime, round, err)
	}
	return api.RoflMarketInstanceCommands(ctx, round, providerAddress, instanceID)
}
