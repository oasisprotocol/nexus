package file

import (
	"context"

	"github.com/akrylysov/pogreb"
	"github.com/oasisprotocol/oasis-core/go/common/cbor"
	roothash "github.com/oasisprotocol/oasis-core/go/roothash/api"

	"github.com/oasisprotocol/oasis-indexer/storage/oasis/nodeapi"
)

type FileRuntimeApiLite struct {
	runtime    string
	db         pogreb.DB
	runtimeApi nodeapi.RuntimeApiLite
}

type RuntimeApiMethod func() (interface{}, error)

var _ nodeapi.RuntimeApiLite = (*FileRuntimeApiLite)(nil)

func NewFileRuntimeApiLite(runtime string, filename string, runtimeApi nodeapi.RuntimeApiLite) (*FileRuntimeApiLite, error) {
	db, err := pogreb.Open(filename, &pogreb.Options{BackgroundSyncInterval: -1})
	if err != nil {
		return nil, err
	}
	return &FileRuntimeApiLite{
		runtime:    runtime,
		db:         *db,
		runtimeApi: runtimeApi,
	}, nil
}

func (r *FileRuntimeApiLite) updateCache(key []byte, method NodeApiMethod) error {
	exists, err := r.db.Has(key)
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

	return r.db.Put(key, cbor.Marshal(val))
}

func (r *FileRuntimeApiLite) GetBlockHeader(ctx context.Context, round uint64) (*nodeapi.RuntimeBlockHeader, error) {
	if round == roothash.RoundLatest {
		if r.runtimeApi != nil {
			return r.runtimeApi.GetBlockHeader(ctx, round)
		}
		return nil, ErrUnstableRPCMethod
	}
	key := generateCacheKey("GetBlockHeader", r.runtime, round)
	if r.runtimeApi != nil {
		if err := r.updateCache(key, func() (interface{}, error) { return r.runtimeApi.GetBlockHeader(ctx, round) }); err != nil {
			return nil, err
		}
	}
	var blockHeader nodeapi.RuntimeBlockHeader
	raw, err := r.db.Get(key)
	if err != nil {
		return nil, err
	}
	err = cbor.Unmarshal(raw, &blockHeader)
	if err != nil {
		return nil, err
	}
	return &blockHeader, nil
}

func (r *FileRuntimeApiLite) GetTransactionsWithResults(ctx context.Context, round uint64) ([]nodeapi.RuntimeTransactionWithResults, error) {
	if round == roothash.RoundLatest {
		if r.runtimeApi != nil {
			return r.runtimeApi.GetTransactionsWithResults(ctx, round)
		}
		return nil, ErrUnstableRPCMethod
	}
	key := generateCacheKey("GetTransactionsWithResults", r.runtime, round)
	if r.runtimeApi != nil {
		if err := r.updateCache(key, func() (interface{}, error) { return r.runtimeApi.GetTransactionsWithResults(ctx, round) }); err != nil {
			return nil, err
		}
	}
	txrs := []nodeapi.RuntimeTransactionWithResults{}
	raw, err := r.db.Get(key)
	if err != nil {
		return nil, err
	}
	err = cbor.Unmarshal(raw, &txrs)
	if err != nil {
		return nil, err
	}
	return txrs, nil
}

func (r *FileRuntimeApiLite) GetEventsRaw(ctx context.Context, round uint64) ([]nodeapi.RuntimeEvent, error) {
	if round == roothash.RoundLatest {
		if r.runtimeApi != nil {
			return r.runtimeApi.GetEventsRaw(ctx, round)
		}
		return nil, ErrUnstableRPCMethod
	}
	key := generateCacheKey("GetEventsRaw", r.runtime, round)
	if r.runtimeApi != nil {
		if err := r.updateCache(key, func() (interface{}, error) { return r.runtimeApi.GetEventsRaw(ctx, round) }); err != nil {
			return nil, err
		}
	}
	events := []nodeapi.RuntimeEvent{}
	raw, err := r.db.Get(key)
	if err != nil {
		return nil, err
	}
	err = cbor.Unmarshal(raw, events)
	if err != nil {
		return nil, err
	}
	return events, nil
}

func (r *FileRuntimeApiLite) EVMSimulateCall(ctx context.Context, round uint64, gasPrice []byte, gasLimit uint64, caller []byte, address []byte, value []byte, data []byte) ([]byte, error) {
	if round == roothash.RoundLatest {
		if r.runtimeApi != nil {
			return r.runtimeApi.EVMSimulateCall(ctx, round, gasPrice, gasLimit, caller, address, value, data)
		}
		return nil, ErrUnstableRPCMethod
	}
	key := generateCacheKey("EVMSimulateCall", r.runtime, round, gasPrice, gasLimit, caller, address, value, data)
	if r.runtimeApi != nil {
		if err := r.updateCache(key, func() (interface{}, error) {
			return r.runtimeApi.EVMSimulateCall(ctx, round, gasPrice, gasLimit, caller, address, value, data)
		}); err != nil {
			return nil, err
		}
	}
	res := []byte{}
	raw, err := r.db.Get(key)
	if err != nil {
		return nil, err
	}
	err = cbor.Unmarshal(raw, res)
	if err != nil {
		return nil, err
	}
	return res, nil
}
