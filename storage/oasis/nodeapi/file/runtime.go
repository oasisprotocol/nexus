package file

import (
	"context"

	sdkTypes "github.com/oasisprotocol/oasis-sdk/client-sdk/go/types"

	"github.com/oasisprotocol/nexus/cache/kvstore"
	roothash "github.com/oasisprotocol/nexus/coreapi/v22.2.11/roothash/api"

	"github.com/oasisprotocol/nexus/common"
	"github.com/oasisprotocol/nexus/log"
	"github.com/oasisprotocol/nexus/metrics"
	"github.com/oasisprotocol/nexus/storage/oasis/nodeapi"
)

type FileRuntimeApiLite struct {
	runtime    common.Runtime
	db         kvstore.KVStore
	runtimeApi nodeapi.RuntimeApiLite
}

type RuntimeApiMethod func() (interface{}, error)

var _ nodeapi.RuntimeApiLite = (*FileRuntimeApiLite)(nil)

func NewFileRuntimeApiLite(runtime common.Runtime, cacheDir string, runtimeApi nodeapi.RuntimeApiLite) (*FileRuntimeApiLite, error) {
	db, err := kvstore.OpenKVStore(
		log.NewDefaultLogger("cached-node-api").With("runtime", runtime),
		cacheDir,
		common.Ptr(metrics.NewDefaultAnalysisMetrics(string(runtime))),
	)
	if err != nil {
		return nil, err
	}
	return &FileRuntimeApiLite{
		runtime:    runtime,
		db:         db,
		runtimeApi: runtimeApi,
	}, nil
}

func (r *FileRuntimeApiLite) Close() error {
	// Close all resources and return the first encountered error, if any.
	var firstErr error
	if r.runtimeApi != nil {
		firstErr = r.runtimeApi.Close()
	}
	if err := r.db.Close(); err != nil && firstErr == nil {
		firstErr = err
	}
	return firstErr
}

func (r *FileRuntimeApiLite) GetBlockHeader(ctx context.Context, round uint64) (*nodeapi.RuntimeBlockHeader, error) {
	return kvstore.GetFromCacheOrCall(
		r.db, round == roothash.RoundLatest,
		kvstore.GenerateCacheKey("GetBlockHeader", r.runtime, round),
		func() (*nodeapi.RuntimeBlockHeader, error) { return r.runtimeApi.GetBlockHeader(ctx, round) },
	)
}

func (r *FileRuntimeApiLite) GetTransactionsWithResults(ctx context.Context, round uint64) ([]nodeapi.RuntimeTransactionWithResults, error) {
	return kvstore.GetSliceFromCacheOrCall(
		r.db, round == roothash.RoundLatest,
		kvstore.GenerateCacheKey("GetTransactionsWithResults", r.runtime, round),
		func() ([]nodeapi.RuntimeTransactionWithResults, error) {
			return r.runtimeApi.GetTransactionsWithResults(ctx, round)
		},
	)
}

func (r *FileRuntimeApiLite) GetEventsRaw(ctx context.Context, round uint64) ([]nodeapi.RuntimeEvent, error) {
	return kvstore.GetSliceFromCacheOrCall(
		r.db, round == roothash.RoundLatest,
		kvstore.GenerateCacheKey("GetEventsRaw", r.runtime, round),
		func() ([]nodeapi.RuntimeEvent, error) { return r.runtimeApi.GetEventsRaw(ctx, round) },
	)
}

func (r *FileRuntimeApiLite) GetBalances(ctx context.Context, round uint64, addr nodeapi.Address) (map[sdkTypes.Denomination]common.BigInt, error) {
	return kvstore.GetMapFromCacheOrCall[sdkTypes.Denomination, common.BigInt](
		r.db, round == roothash.RoundLatest,
		kvstore.GenerateCacheKey("GetBalances", r.runtime, round, addr),
		func() (map[sdkTypes.Denomination]common.BigInt, error) {
			return r.runtimeApi.GetBalances(ctx, round, addr)
		},
	)
}

func (r *FileRuntimeApiLite) GetMinGasPrice(ctx context.Context, round uint64) (map[sdkTypes.Denomination]common.BigInt, error) {
	return kvstore.GetMapFromCacheOrCall[sdkTypes.Denomination, common.BigInt](
		r.db, round == roothash.RoundLatest,
		kvstore.GenerateCacheKey("GetMinGasPrice", r.runtime, round),
		func() (map[sdkTypes.Denomination]common.BigInt, error) {
			return r.runtimeApi.GetMinGasPrice(ctx, round)
		},
	)
}

func (r *FileRuntimeApiLite) EVMSimulateCall(ctx context.Context, round uint64, gasPrice []byte, gasLimit uint64, caller []byte, address []byte, value []byte, data []byte) (*nodeapi.FallibleResponse, error) {
	return kvstore.GetFromCacheOrCall(
		r.db, round == roothash.RoundLatest,
		kvstore.GenerateCacheKey("EVMSimulateCall", r.runtime, round, gasPrice, gasLimit, caller, address, value, data),
		func() (*nodeapi.FallibleResponse, error) {
			return r.runtimeApi.EVMSimulateCall(ctx, round, gasPrice, gasLimit, caller, address, value, data)
		},
	)
}

func (r *FileRuntimeApiLite) EVMGetCode(ctx context.Context, round uint64, address []byte) ([]byte, error) {
	return kvstore.GetSliceFromCacheOrCall(
		r.db, round == roothash.RoundLatest,
		kvstore.GenerateCacheKey("EVMGetCode", r.runtime, round, address),
		func() ([]byte, error) {
			return r.runtimeApi.EVMGetCode(ctx, round, address)
		},
	)
}

func (r *FileRuntimeApiLite) GetDelegation(ctx context.Context, round uint64, from nodeapi.Address, to nodeapi.Address) (*nodeapi.DelegationInfo, error) {
	return kvstore.GetFromCacheOrCall(
		r.db, round == roothash.RoundLatest,
		kvstore.GenerateCacheKey("GetDelegation", r.runtime, round, from, to),
		func() (*nodeapi.DelegationInfo, error) { return r.runtimeApi.GetDelegation(ctx, round, from, to) },
	)
}

func (r *FileRuntimeApiLite) GetAllDelegations(ctx context.Context, round uint64) ([]*nodeapi.CompleteDelegationInfo, error) {
	return kvstore.GetSliceFromCacheOrCall(
		r.db, round == roothash.RoundLatest,
		kvstore.GenerateCacheKey("GetAllDelegations", r.runtime, round),
		func() ([]*nodeapi.CompleteDelegationInfo, error) { return r.runtimeApi.GetAllDelegations(ctx, round) },
	)
}

func (r *FileRuntimeApiLite) GetAllUndelegations(ctx context.Context, round uint64) ([]*nodeapi.CompleteUndelegationInfo, error) {
	return kvstore.GetSliceFromCacheOrCall(
		r.db, round == roothash.RoundLatest,
		kvstore.GenerateCacheKey("GetAllUndelegations", r.runtime, round),
		func() ([]*nodeapi.CompleteUndelegationInfo, error) {
			return r.runtimeApi.GetAllUndelegations(ctx, round)
		},
	)
}

func (r *FileRuntimeApiLite) RoflApps(ctx context.Context, round uint64) ([]*nodeapi.AppConfig, error) {
	return kvstore.GetSliceFromCacheOrCall(
		r.db, round == roothash.RoundLatest,
		kvstore.GenerateCacheKey("RoflApps", r.runtime, round),
		func() ([]*nodeapi.AppConfig, error) { return r.runtimeApi.RoflApps(ctx, round) },
	)
}

func (r *FileRuntimeApiLite) RoflApp(ctx context.Context, round uint64, id nodeapi.AppID) (*nodeapi.AppConfig, error) {
	return kvstore.GetFromCacheOrCall(
		r.db, round == roothash.RoundLatest,
		kvstore.GenerateCacheKey("RoflApp", r.runtime, round, id),
		func() (*nodeapi.AppConfig, error) { return r.runtimeApi.RoflApp(ctx, round, id) },
	)
}

func (r *FileRuntimeApiLite) RoflAppInstances(ctx context.Context, round uint64, id nodeapi.AppID) ([]*nodeapi.Registration, error) {
	return kvstore.GetSliceFromCacheOrCall(
		r.db, round == roothash.RoundLatest,
		kvstore.GenerateCacheKey("RoflAppInstances", r.runtime, round, id),
		func() ([]*nodeapi.Registration, error) { return r.runtimeApi.RoflAppInstances(ctx, round, id) },
	)
}

func (r *FileRuntimeApiLite) RoflAppInstance(ctx context.Context, round uint64, id nodeapi.AppID, rak nodeapi.PublicKey) (*nodeapi.Registration, error) {
	return kvstore.GetFromCacheOrCall(
		r.db, round == roothash.RoundLatest,
		kvstore.GenerateCacheKey("RoflAppInstance", r.runtime, round, id, rak),
		func() (*nodeapi.Registration, error) { return r.runtimeApi.RoflAppInstance(ctx, round, id, rak) },
	)
}

func (r *FileRuntimeApiLite) RoflMarketProvider(ctx context.Context, round uint64, providerAddress sdkTypes.Address) (*nodeapi.Provider, error) {
	return kvstore.GetFromCacheOrCall(
		r.db, round == roothash.RoundLatest,
		kvstore.GenerateCacheKey("RoflMarketProvider", r.runtime, round, providerAddress),
		func() (*nodeapi.Provider, error) { return r.runtimeApi.RoflMarketProvider(ctx, round, providerAddress) },
	)
}

func (r *FileRuntimeApiLite) RoflMarketProviders(ctx context.Context, round uint64) ([]*nodeapi.Provider, error) {
	return kvstore.GetSliceFromCacheOrCall(
		r.db, round == roothash.RoundLatest,
		kvstore.GenerateCacheKey("RoflMarketProviders", r.runtime, round),
		func() ([]*nodeapi.Provider, error) { return r.runtimeApi.RoflMarketProviders(ctx, round) },
	)
}

func (r *FileRuntimeApiLite) RoflMarketOffer(ctx context.Context, round uint64, providerAddress sdkTypes.Address, offerID nodeapi.OfferID) (*nodeapi.Offer, error) {
	return kvstore.GetFromCacheOrCall(
		r.db, round == roothash.RoundLatest,
		kvstore.GenerateCacheKey("RoflMarketOffer", r.runtime, round, providerAddress, offerID),
		func() (*nodeapi.Offer, error) {
			return r.runtimeApi.RoflMarketOffer(ctx, round, providerAddress, offerID)
		},
	)
}

func (r *FileRuntimeApiLite) RoflMarketOffers(ctx context.Context, round uint64, providerAddress sdkTypes.Address) ([]*nodeapi.Offer, error) {
	return kvstore.GetSliceFromCacheOrCall(
		r.db, round == roothash.RoundLatest,
		kvstore.GenerateCacheKey("RoflMarketOffers", r.runtime, round, providerAddress),
		func() ([]*nodeapi.Offer, error) { return r.runtimeApi.RoflMarketOffers(ctx, round, providerAddress) },
	)
}

func (r *FileRuntimeApiLite) RoflMarketInstance(ctx context.Context, round uint64, providerAddress sdkTypes.Address, instanceID nodeapi.InstanceID) (*nodeapi.Instance, error) {
	return kvstore.GetFromCacheOrCall(
		r.db, round == roothash.RoundLatest,
		kvstore.GenerateCacheKey("RoflMarketInstance", r.runtime, round, providerAddress, instanceID),
		func() (*nodeapi.Instance, error) {
			return r.runtimeApi.RoflMarketInstance(ctx, round, providerAddress, instanceID)
		},
	)
}

func (r *FileRuntimeApiLite) RoflMarketInstances(ctx context.Context, round uint64, providerAddress sdkTypes.Address) ([]*nodeapi.Instance, error) {
	return kvstore.GetSliceFromCacheOrCall(
		r.db, round == roothash.RoundLatest,
		kvstore.GenerateCacheKey("RoflMarketInstances", r.runtime, round, providerAddress),
		func() ([]*nodeapi.Instance, error) {
			return r.runtimeApi.RoflMarketInstances(ctx, round, providerAddress)
		},
	)
}

func (r *FileRuntimeApiLite) RoflMarketInstanceCommands(ctx context.Context, round uint64, providerAddress sdkTypes.Address, instanceID nodeapi.InstanceID) ([]*nodeapi.QueuedCommand, error) {
	return kvstore.GetSliceFromCacheOrCall(
		r.db, round == roothash.RoundLatest,
		kvstore.GenerateCacheKey("RoflMarketInstanceCommands", r.runtime, round, providerAddress, instanceID),
		func() ([]*nodeapi.QueuedCommand, error) {
			return r.runtimeApi.RoflMarketInstanceCommands(ctx, round, providerAddress, instanceID)
		},
	)
}
