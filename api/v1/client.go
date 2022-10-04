package v1

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/dgraph-io/ristretto"
	"github.com/go-chi/chi/v5"

	governance "github.com/oasisprotocol/oasis-core/go/governance/api"
	staking "github.com/oasisprotocol/oasis-core/go/staking/api"
	"github.com/oasisprotocol/oasis-indexer/api/common"
	"github.com/oasisprotocol/oasis-indexer/log"
	storage "github.com/oasisprotocol/oasis-indexer/storage/client"
)

const (
	blockCost = 1
	txCost    = 1
)

// storageClient is a wrapper around a storage.TargetStorage
// with knowledge of network semantics.
type storageClient struct {
	chainID string
	storage *storage.StorageClient

	blockCache *ristretto.Cache
	txCache    *ristretto.Cache

	logger *log.Logger
}

// newStorageClient creates a new storage client.
func newStorageClient(chainID string, s *storage.StorageClient, l *log.Logger) (*storageClient, error) {
	blockCache, err := ristretto.NewCache(&ristretto.Config{
		NumCounters:        1024 * 10,
		MaxCost:            1024,
		BufferItems:        64,
		IgnoreInternalCost: true,
	})
	if err != nil {
		l.Error("api client: failed to create block cache: %w", err)
		return nil, err
	}
	txCache, err := ristretto.NewCache(&ristretto.Config{
		NumCounters:        1024 * 10,
		MaxCost:            1024,
		BufferItems:        64,
		IgnoreInternalCost: true,
	})
	if err != nil {
		l.Error("api client: failed to create tx cache: %w", err)
		return nil, err
	}
	return &storageClient{chainID, s, blockCache, txCache, l}, nil
}

// Status returns status information for the Oasis Indexer.
func (c *storageClient) Status(ctx context.Context) (*storage.Status, error) {
	return c.Status(ctx)
}

// Blocks returns a list of consensus blocks.
func (c *storageClient) Blocks(ctx context.Context, r *http.Request) (*storage.BlockList, error) {
	var q storage.BlocksRequest
	params := r.URL.Query()
	if v := params.Get("from"); v != "" {
		from, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return nil, common.ErrBadRequest
		}
		q.From = &from
	}
	if v := params.Get("to"); v != "" {
		to, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return nil, common.ErrBadRequest
		}
		q.To = &to
	}
	if v := params.Get("after"); v != "" {
		after, err := time.Parse("2006-01-02T15:04:05Z-0700", v)
		if err != nil {
			return nil, common.ErrBadRequest
		}
		q.After = &after
	}
	if v := params.Get("before"); v != "" {
		before, err := time.Parse("2006-01-02T15:04:05Z-0700", v)
		if err != nil {
			return nil, common.ErrBadRequest
		}
		q.Before = &before
	}

	p, err := common.NewPagination(r)
	if err != nil {
		c.logger.Info("pagination failed",
			"request_id", ctx.Value(RequestIDContextKey),
			"err", err.Error(),
		)
		return nil, common.ErrBadRequest
	}

	return c.storage.Blocks(ctx, &q, &p)
}

// Block returns a consensus block. This endpoint's responses are cached.
func (c *storageClient) Block(ctx context.Context, r *http.Request) (*storage.Block, error) {
	var q storage.BlockRequest

	v := chi.URLParam(r, "height")
	if v == "" {
		c.logger.Info("missing request parameter(s)")
		return nil, common.ErrBadRequest
	}
	height, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return nil, common.ErrBadRequest
	}
	q.Height = &height

	// Check cache
	untypedBlock, ok := c.blockCache.Get(height)
	if ok {
		return untypedBlock.(*storage.Block), nil
	}

	blk, err := c.storage.Block(ctx, &q)
	if err != nil {
		return nil, err
	}
	c.cacheBlock(blk)
	return blk, nil
}

// cacheBlock adds a block to the client's block cache.
func (c *storageClient) cacheBlock(blk *storage.Block) {
	c.blockCache.Set(blk.Height, blk, blockCost)
}

// Transactions returns a list of consensus transactions.
func (c *storageClient) Transactions(ctx context.Context, r *http.Request) (*storage.TransactionList, error) {
	var q storage.TransactionsRequest
	params := r.URL.Query()
	if v := params.Get("block"); v != "" {
		block, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return nil, common.ErrBadRequest
		}
		q.Block = &block
	}
	if v := params.Get("method"); v != "" {
		q.Method = &v
	}
	if v := params.Get("sender"); v != "" {
		var sender *staking.Address
		err := sender.UnmarshalText([]byte(v))
		if err != nil || !sender.IsValid() {
			return nil, common.ErrBadRequest
		}
		q.Sender = sender
	}
	if v := params.Get("minFee"); v != "" {
		minFee, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return nil, common.ErrBadRequest
		}
		q.MinFee = &minFee
	}
	if v := params.Get("maxFee"); v != "" {
		maxFee, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return nil, common.ErrBadRequest
		}
		q.MaxFee = &maxFee
	}
	if v := params.Get("code"); v != "" {
		code, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return nil, common.ErrBadRequest
		}
		q.Code = &code
	}

	p, err := common.NewPagination(r)
	if err != nil {
		c.logger.Info("pagination failed",
			"request_id", ctx.Value(RequestIDContextKey),
			"err", err.Error(),
		)
		return nil, common.ErrBadRequest
	}

	return c.storage.Transactions(ctx, &q, &p)
}

// Transaction returns a consensus transaction.
func (c *storageClient) Transaction(ctx context.Context, r *http.Request) (*storage.Transaction, error) {
	var q storage.TransactionRequest

	txHash := chi.URLParam(r, "tx_hash")
	if txHash == "" {
		c.logger.Info("missing request parameters")
		return nil, common.ErrBadRequest
	}
	q.TxHash = &txHash

	// Check cache
	untypedTx, ok := c.txCache.Get(txHash)
	if ok {
		return untypedTx.(*storage.Transaction), nil
	}
	tx, err := c.storage.Transaction(ctx, &q)
	if err != nil {
		return nil, err
	}
	c.cacheTx(tx)
	return tx, nil
}

// cacheTx adds a transaction to the client's transaction cache.
func (c *storageClient) cacheTx(tx *storage.Transaction) {
	c.txCache.Set(tx.Hash, tx, txCost)
}

// Entities returns a list of registered entities.
func (c *storageClient) Entities(ctx context.Context, r *http.Request) (*storage.EntityList, error) {
	p, err := common.NewPagination(r)
	if err != nil {
		c.logger.Info("pagination failed",
			"request_id", ctx.Value(RequestIDContextKey),
			"err", err.Error(),
		)
		return nil, common.ErrBadRequest
	}

	return c.storage.Entities(ctx, &p)
}

// Entity returns a registered entity.
func (c *storageClient) Entity(ctx context.Context, r *http.Request) (*storage.Entity, error) {
	var q storage.EntityRequest

	entityId, err := url.PathUnescape(chi.URLParam(r, "entity_id"))
	if err != nil {
		return nil, common.ErrBadRequest
	}
	if entityId == "" {
		c.logger.Info("missing request parameters")
		return nil, common.ErrBadRequest
	}
	q.EntityId = &entityId

	return c.storage.Entity(ctx, &q)
}

// EntityNodes returns a list of nodes controlled by the provided entity.
func (c *storageClient) EntityNodes(ctx context.Context, r *http.Request) (*storage.NodeList, error) {
	var q storage.EntityNodesRequest

	entityId, err := url.PathUnescape(chi.URLParam(r, "entity_id"))
	if err != nil {
		return nil, common.ErrBadRequest
	}
	if entityId == "" {
		c.logger.Info("missing request parameters")
		return nil, common.ErrBadRequest
	}
	q.EntityId = &entityId

	p, err := common.NewPagination(r)
	if err != nil {
		c.logger.Info("pagination failed",
			"request_id", ctx.Value(RequestIDContextKey),
			"err", err.Error(),
		)
		return nil, common.ErrBadRequest
	}

	return c.storage.EntityNodes(ctx, &q, &p)
}

// EntityNode returns a node controlled by the provided entity.
func (c *storageClient) EntityNode(ctx context.Context, r *http.Request) (*storage.Node, error) {
	var q storage.EntityNodeRequest
	entityId, err := url.PathUnescape(chi.URLParam(r, "entity_id"))
	if err != nil {
		return nil, common.ErrBadRequest
	}
	nodeId, err := url.PathUnescape(chi.URLParam(r, "node_id"))
	if err != nil {
		return nil, common.ErrBadRequest
	}
	if entityId == "" || nodeId == "" {
		c.logger.Info("missing request parameters")
		return nil, common.ErrBadRequest
	}
	q.EntityId = &entityId
	q.NodeId = &nodeId

	return c.storage.EntityNode(ctx, &q)
}

// Accounts returns a list of consensus accounts.
func (c *storageClient) Accounts(ctx context.Context, r *http.Request) (*storage.AccountList, error) {
	var q storage.AccountsRequest
	params := r.URL.Query()

	if v := params.Get("minAvailable"); v != "" {
		minAvailable, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return nil, common.ErrBadRequest
		}
		q.MinAvailable = &minAvailable
	}
	if v := params.Get("maxAvailable"); v != "" {
		maxAvailable, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return nil, common.ErrBadRequest
		}
		q.MaxAvailable = &maxAvailable
	}
	if v := params.Get("minEscrow"); v != "" {
		minEscrow, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return nil, common.ErrBadRequest
		}
		q.MinEscrow = &minEscrow
	}
	if v := params.Get("maxEscrow"); v != "" {
		maxEscrow, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return nil, common.ErrBadRequest
		}
		q.MaxEscrow = &maxEscrow
	}
	if v := params.Get("minDebonding"); v != "" {
		minDebonding, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return nil, common.ErrBadRequest
		}
		q.MinDebonding = &minDebonding
	}
	if v := params.Get("maxDebonding"); v != "" {
		maxDebonding, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return nil, common.ErrBadRequest
		}
		q.MaxDebonding = &maxDebonding
	}
	if v := params.Get("minTotalBalance"); v != "" {
		minTotalBalance, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return nil, common.ErrBadRequest
		}
		q.MinTotalBalance = &minTotalBalance
	}
	if v := params.Get("maxTotalBalance"); v != "" {
		maxTotalBalance, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return nil, common.ErrBadRequest
		}
		q.MaxTotalBalance = &maxTotalBalance
	}

	p, err := common.NewPagination(r)
	if err != nil {
		c.logger.Info("pagination failed",
			"request_id", ctx.Value(RequestIDContextKey),
			"err", err.Error(),
		)
		return nil, common.ErrBadRequest
	}

	return c.storage.Accounts(ctx, &q, &p)
}

// Account returns a consensus account.
func (c *storageClient) Account(ctx context.Context, r *http.Request) (*storage.Account, error) {
	var q storage.AccountRequest

	v := chi.URLParam(r, "address")
	if v == "" {
		c.logger.Info("missing required parameters")
		return nil, common.ErrBadRequest
	}
	var address *staking.Address
	err := address.UnmarshalText([]byte(v))
	if err != nil || !address.IsValid() {
		return nil, common.ErrBadRequest
	}
	q.Address = address

	return c.storage.Account(ctx, &q)
}

// Delegations returns a list of delegations.
func (c *storageClient) Delegations(ctx context.Context, r *http.Request) (*storage.DelegationList, error) {
	var q storage.DelegationsRequest

	v := chi.URLParam(r, "address")
	if v == "" {
		c.logger.Info("missing required parameters")
		return nil, common.ErrBadRequest
	}
	var address *staking.Address
	err := address.UnmarshalText([]byte(v))
	if err != nil || !address.IsValid() {
		return nil, common.ErrBadRequest
	}
	q.Address = address

	p, err := common.NewPagination(r)
	if err != nil {
		c.logger.Info("pagination failed",
			"request_id", ctx.Value(RequestIDContextKey),
			"err", err.Error(),
		)
		return nil, common.ErrBadRequest
	}

	return c.storage.Delegations(ctx, &q, &p)
}

// DebondingDelegations returns a list of debonding delegations.
func (c *storageClient) DebondingDelegations(ctx context.Context, r *http.Request) (*storage.DebondingDelegationList, error) {
	var q storage.DebondingDelegationsRequest

	v := chi.URLParam(r, "address")
	if v == "" {
		c.logger.Info("missing required parameters")
		return nil, common.ErrBadRequest
	}
	var address *staking.Address
	err := address.UnmarshalText([]byte(v))
	if err != nil || !address.IsValid() {
		return nil, common.ErrBadRequest
	}
	q.Address = address

	p, err := common.NewPagination(r)
	if err != nil {
		c.logger.Info("pagination failed",
			"request_id", ctx.Value(RequestIDContextKey),
			"err", err.Error(),
		)
		return nil, common.ErrBadRequest
	}

	return c.storage.DebondingDelegations(ctx, &q, &p)
}

// Epochs returns a list of consensus epochs.
func (c *storageClient) Epochs(ctx context.Context, r *http.Request) (*storage.EpochList, error) {
	p, err := common.NewPagination(r)
	if err != nil {
		c.logger.Info("pagination failed",
			"request_id", ctx.Value(RequestIDContextKey),
			"err", err.Error(),
		)
		return nil, common.ErrBadRequest
	}

	return c.storage.Epochs(ctx, &p)
}

// Epoch returns a consensus epoch.
func (c *storageClient) Epoch(ctx context.Context, r *http.Request) (*storage.Epoch, error) {
	var q storage.EpochRequest

	v := chi.URLParam(r, "epoch")
	if v == "" {
		c.logger.Info("missing required parameters")
		return nil, common.ErrBadRequest
	}
	epoch, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return nil, common.ErrBadRequest
	}
	q.Epoch = &epoch

	return c.storage.Epoch(ctx, &q)
}

// Proposals returns a list of governance proposals.
func (c *storageClient) Proposals(ctx context.Context, r *http.Request) (*storage.ProposalList, error) {
	var q storage.ProposalsRequest
	params := r.URL.Query()

	if v := params.Get("submitter"); v != "" {
		var submitter *staking.Address
		err := submitter.UnmarshalText([]byte(v))
		if err != nil || !submitter.IsValid() {
			return nil, common.ErrBadRequest
		}
		q.Submitter = submitter
	}
	if v := params.Get("state"); v != "" {
		var state *governance.ProposalState
		if err := state.UnmarshalText([]byte(v)); err != nil {
			return nil, common.ErrBadRequest
		}
		q.State = state
	}

	p, err := common.NewPagination(r)
	if err != nil {
		c.logger.Info("pagination failed",
			"request_id", ctx.Value(RequestIDContextKey),
			"err", err.Error(),
		)
		return nil, common.ErrBadRequest
	}

	return c.storage.Proposals(ctx, &q, &p)
}

// Proposal returns a governance proposal.
func (c *storageClient) Proposal(ctx context.Context, r *http.Request) (*storage.Proposal, error) {
	var q storage.ProposalRequest

	v := chi.URLParam(r, "proposal_id")
	if v == "" {
		c.logger.Info("missing required parameters")
		return nil, common.ErrBadRequest
	}
	proposalId, err := strconv.ParseUint(v, 10, 64)
	if err != nil {
		return nil, common.ErrBadRequest
	}
	q.ProposalId = &proposalId

	return c.storage.Proposal(ctx, &q)
}

// ProposalVotes returns votes for a governance proposal.
func (c *storageClient) ProposalVotes(ctx context.Context, r *http.Request) (*storage.ProposalVotes, error) {
	var q storage.ProposalVotesRequest

	v := chi.URLParam(r, "proposal_id")
	if v == "" {
		c.logger.Info("missing required parameters")
		return nil, common.ErrBadRequest
	}
	proposalId, err := strconv.ParseUint(v, 10, 64)
	if err != nil {
		return nil, common.ErrBadRequest
	}
	q.ProposalId = &proposalId

	p, err := common.NewPagination(r)
	if err != nil {
		c.logger.Info("pagination failed",
			"request_id", ctx.Value(RequestIDContextKey),
			"err", err.Error(),
		)
		return nil, common.ErrBadRequest
	}

	return c.storage.ProposalVotes(ctx, &q, &p)
}

// Validators returns a list of validators.
func (c *storageClient) Validators(ctx context.Context, r *http.Request) (*storage.ValidatorList, error) {
	order := "voting_power"
	p := common.Pagination{
		Order:  &order,
		Limit:  1000,
		Offset: 0,
	}

	return c.storage.Validators(ctx, &p)
}

// Validator returns a single validator.
func (c *storageClient) Validator(ctx context.Context, r *http.Request) (*storage.Validator, error) {
	var q storage.ValidatorRequest

	entityId := chi.URLParam(r, "entity_id")
	if entityId == "" {
		c.logger.Info("missing required parameters")
		return nil, common.ErrBadRequest
	}
	q.EntityId = &entityId

	return c.storage.Validator(ctx, &q)

}

// TransactionsPerSecond returns a list of tps checkpoint values.
func (c *storageClient) TransactionsPerSecond(ctx context.Context, r *http.Request) (*storage.TpsCheckpointList, error) {
	p, err := common.NewPagination(r)
	if err != nil {
		c.logger.Info("pagination failed",
			"request_id", ctx.Value(RequestIDContextKey),
			"err", err.Error(),
		)
		return nil, common.ErrBadRequest
	}

	return c.storage.TransactionsPerSecond(ctx, &p)
}

// DailyVolumes returns a list of daily transaction volumes.
func (c *storageClient) DailyVolumes(ctx context.Context, r *http.Request) (*storage.VolumeList, error) {
	p, err := common.NewPagination(r)
	if err != nil {
		c.logger.Info("pagination failed",
			"request_id", ctx.Value(RequestIDContextKey),
			"err", err.Error(),
		)
		return nil, common.ErrBadRequest
	}

	return c.storage.DailyVolumes(ctx, &p)
}
