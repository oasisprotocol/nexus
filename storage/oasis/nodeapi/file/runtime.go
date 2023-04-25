package file

import (
	"context"

	"github.com/akrylysov/pogreb"
	roothash "github.com/oasisprotocol/oasis-core/go/roothash/api"

	"github.com/oasisprotocol/oasis-indexer/common"
	"github.com/oasisprotocol/oasis-indexer/storage/oasis/nodeapi"
)

type FileRuntimeApiLite struct {
	runtime    common.Runtime
	db         KVStore
	runtimeApi nodeapi.RuntimeApiLite
}

type RuntimeApiMethod func() (interface{}, error)

var _ nodeapi.RuntimeApiLite = (*FileRuntimeApiLite)(nil)

func NewFileRuntimeApiLite(runtime common.Runtime, cacheDir string, runtimeApi nodeapi.RuntimeApiLite) (*FileRuntimeApiLite, error) {
	db, err := pogreb.Open(cacheDir, &pogreb.Options{BackgroundSyncInterval: -1})
	if err != nil {
		return nil, err
	}
	return &FileRuntimeApiLite{
		runtime:    runtime,
		db:         KVStore{db},
		runtimeApi: runtimeApi,
	}, nil
}

func (r *FileRuntimeApiLite) Close() error {
	return r.db.Close()
}

func (r *FileRuntimeApiLite) GetBlockHeader(ctx context.Context, round uint64) (*nodeapi.RuntimeBlockHeader, error) {
	return GetFromCacheOrCall(
		r.db, round == roothash.RoundLatest,
		generateCacheKey("GetBlockHeader", r.runtime, round),
		func() (*nodeapi.RuntimeBlockHeader, error) { return r.runtimeApi.GetBlockHeader(ctx, round) },
	)
}

func (r *FileRuntimeApiLite) GetTransactionsWithResults(ctx context.Context, round uint64) ([]nodeapi.RuntimeTransactionWithResults, error) {
	return GetSliceFromCacheOrCall(
		r.db, round == roothash.RoundLatest,
		generateCacheKey("GetTransactionsWithResults", r.runtime, round),
		func() ([]nodeapi.RuntimeTransactionWithResults, error) {
			return r.runtimeApi.GetTransactionsWithResults(ctx, round)
		},
	)
}

func (r *FileRuntimeApiLite) GetEventsRaw(ctx context.Context, round uint64) ([]nodeapi.RuntimeEvent, error) {
	return GetSliceFromCacheOrCall(
		r.db, round == roothash.RoundLatest,
		generateCacheKey("GetEventsRaw", r.runtime, round),
		func() ([]nodeapi.RuntimeEvent, error) { return r.runtimeApi.GetEventsRaw(ctx, round) },
	)
}

func (r *FileRuntimeApiLite) EVMSimulateCall(ctx context.Context, round uint64, gasPrice []byte, gasLimit uint64, caller []byte, address []byte, value []byte, data []byte) ([]byte, error) {
	return GetSliceFromCacheOrCall(
		r.db, round == roothash.RoundLatest,
		generateCacheKey("EVMSimulateCall", r.runtime, round, gasPrice, gasLimit, caller, address, value, data),
		func() ([]byte, error) {
			return r.runtimeApi.EVMSimulateCall(ctx, round, gasPrice, gasLimit, caller, address, value, data)
		},
	)
}
