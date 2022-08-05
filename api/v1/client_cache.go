package v1

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/dgraph-io/ristretto"
	"github.com/go-chi/chi/v5"
	"github.com/oasislabs/oasis-indexer/api/common"
	"github.com/oasislabs/oasis-indexer/log"
)

const (
	blockCost = 1
	txCost    = 1
)

type cachingStorageClient struct {
	inner storageClient

	blockCache *ristretto.Cache
	txCache    *ristretto.Cache

	logger *log.Logger
}

func newCachingStorageClient(sc storageClient, logger *log.Logger) (*cachingStorageClient, error) {
	blockCache, err := ristretto.NewCache(&ristretto.Config{
		NumCounters:        1024 * 10,
		MaxCost:            1024,
		BufferItems:        64,
		IgnoreInternalCost: true,
	})
	if err != nil {
		return nil, fmt.Errorf("cache client: failed to create block cache: %w", err)
	}
	txCache, err := ristretto.NewCache(&ristretto.Config{
		NumCounters:        1024 * 10,
		MaxCost:            1024,
		BufferItems:        64,
		IgnoreInternalCost: true,
	})
	if err != nil {
		return nil, fmt.Errorf("cache client: failed to create tx cache: %w", err)
	}

	return &cachingStorageClient{sc, blockCache, txCache, logger}, nil
}

func (c *cachingStorageClient) Block(ctx context.Context, r *http.Request) (*Block, error) {
	height, err := c.heightFromRequest(r)
	if err != nil {
		c.logger.Info("cache client: input validation failed",
			"request_id", ctx.Value(RequestIDContextKey),
			"err", err.Error(),
		)
		return nil, err
	}
	untypedBlock, ok := c.blockCache.Get(height)
	if ok {
		return untypedBlock.(*Block), nil
	}
	blk, err := c.inner.Block(ctx, r)
	if err != nil {
		return nil, err
	}
	c.cacheBlock(blk)
	return blk, nil
}

func (c *cachingStorageClient) Transaction(ctx context.Context, r *http.Request) (*Transaction, error) {
	hash, err := c.hashFromRequest(r)
	if err != nil {
		c.logger.Info("cache client: input validation failed",
			"request_id", ctx.Value(RequestIDContextKey),
			"err", err.Error(),
		)
		return nil, err
	}
	untypedTx, ok := c.txCache.Get(hash)
	if ok {
		return untypedTx.(*Transaction), nil
	}
	tx, err := c.inner.Transaction(ctx, r)
	if err != nil {
		return nil, err
	}
	c.cacheTx(tx)
	return tx, nil
}

func (c *cachingStorageClient) heightFromRequest(r *http.Request) (int64, error) {
	height, err := strconv.Atoi(chi.URLParam(r, "height"))
	if err != nil {
		return 0, common.ErrBadRequest
	}

	return int64(height), nil
}

func (c *cachingStorageClient) hashFromRequest(r *http.Request) (string, error) {
	hash := chi.URLParam(r, "txn_hash")
	if len(hash) == 0 {
		return "", common.ErrBadRequest
	}

	return hash, nil
}

func (c *cachingStorageClient) cacheBlock(blk *Block) {
	c.blockCache.Set(blk.Height, blk, blockCost)
}

func (c *cachingStorageClient) cacheTx(tx *Transaction) {
	c.txCache.Set(tx.Hash, tx, txCost)
}

func (c *cachingStorageClient) Status(ctx context.Context) (*Status, error) {
	return c.inner.Status(ctx)
}

func (c *cachingStorageClient) Blocks(ctx context.Context, r *http.Request) (*BlockList, error) {
	return c.inner.Blocks(ctx, r)
}

func (c *cachingStorageClient) Transactions(ctx context.Context, r *http.Request) (*TransactionList, error) {
	return c.inner.Transactions(ctx, r)
}

func (c *cachingStorageClient) Entities(ctx context.Context, r *http.Request) (*EntityList, error) {
	return c.inner.Entities(ctx, r)
}

func (c *cachingStorageClient) Entity(ctx context.Context, r *http.Request) (*Entity, error) {
	return c.inner.Entity(ctx, r)
}

func (c *cachingStorageClient) EntityNodes(ctx context.Context, r *http.Request) (*NodeList, error) {
	return c.inner.EntityNodes(ctx, r)
}

func (c *cachingStorageClient) EntityNode(ctx context.Context, r *http.Request) (*Node, error) {
	return c.inner.EntityNode(ctx, r)
}

func (c *cachingStorageClient) Accounts(ctx context.Context, r *http.Request) (*AccountList, error) {
	return c.inner.Accounts(ctx, r)
}

func (c *cachingStorageClient) Account(ctx context.Context, r *http.Request) (*Account, error) {
	return c.inner.Account(ctx, r)
}

func (c *cachingStorageClient) Delegations(ctx context.Context, r *http.Request) (*DelegationList, error) {
	return c.inner.Delegations(ctx, r)
}

func (c *cachingStorageClient) DebondingDelegations(ctx context.Context, r *http.Request) (*DebondingDelegationList, error) {
	return c.inner.DebondingDelegations(ctx, r)
}

func (c *cachingStorageClient) Epochs(ctx context.Context, r *http.Request) (*EpochList, error) {
	return c.inner.Epochs(ctx, r)
}

func (c *cachingStorageClient) Epoch(ctx context.Context, r *http.Request) (*Epoch, error) {
	return c.inner.Epoch(ctx, r)
}

func (c *cachingStorageClient) Proposals(ctx context.Context, r *http.Request) (*ProposalList, error) {
	return c.inner.Proposals(ctx, r)
}

func (c *cachingStorageClient) Proposal(ctx context.Context, r *http.Request) (*Proposal, error) {
	return c.inner.Proposal(ctx, r)
}

func (c *cachingStorageClient) ProposalVotes(ctx context.Context, r *http.Request) (*ProposalVotes, error) {
	return c.inner.ProposalVotes(ctx, r)
}

func (c *cachingStorageClient) Validators(ctx context.Context, r *http.Request) (*ValidatorList, error) {
	return c.inner.Validators(ctx, r)
}

func (c *cachingStorageClient) Validator(ctx context.Context, r *http.Request) (*Validator, error) {
	return c.inner.Validator(ctx, r)
}
