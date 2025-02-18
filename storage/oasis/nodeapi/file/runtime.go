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
