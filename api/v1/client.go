package v1

import (
	"context"
	"fmt"
	"math/big"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/oasisprotocol/oasis-core/go/common/crypto/signature"
	governance "github.com/oasisprotocol/oasis-core/go/governance/api"
	staking "github.com/oasisprotocol/oasis-core/go/staking/api"

	analyzerApi "github.com/oasisprotocol/oasis-indexer/analyzer"
	apiCommon "github.com/oasisprotocol/oasis-indexer/api/common"
	apiTypes "github.com/oasisprotocol/oasis-indexer/api/v1/types"
	"github.com/oasisprotocol/oasis-indexer/common"
	"github.com/oasisprotocol/oasis-indexer/log"
	storage "github.com/oasisprotocol/oasis-indexer/storage/client"
)

// storageClient is a wrapper around a storage.StorageClient
// with knowledge of network semantics.
type storageClient struct {
	chainID string
	storage *storage.StorageClient
	logger  *log.Logger
}

// newStorageClient creates a new storage client.
func newStorageClient(chainID string, s *storage.StorageClient, l *log.Logger) *storageClient {
	return &storageClient{chainID, s, l}
}

// validateInt32 parses an int32 url parameter.
func validateInt32(param string) (int32, error) {
	i, err := strconv.ParseInt(param, 10, 32)
	if err != nil {
		return 0, apiCommon.ErrBadRequest
	}
	return int32(i), nil
}

// validateInt64 parses an int64 url parameter.
func validateInt64(param string) (int64, error) {
	return strconv.ParseInt(param, 10, 64)
}

// validateUint64 parses an int64 url parameter.
func validateUint64(param string) (uint64, error) {
	return strconv.ParseUint(param, 10, 64)
}

// validateBigInt parses a big.Int url parameter.
func validateBigInt(param string) (*common.BigInt, error) {
	i := big.NewInt(0)
	i, ok := i.SetString(param, 10)
	if !ok {
		return nil, fmt.Errorf("invalid big.Int: %+v", param)
	}
	return &common.BigInt{Int: *i}, nil
}

// validateDatetime parses a datetime url parameter.
func validateDatetime(param string) (time.Time, error) {
	return time.Parse("2006-01-02T15:04:05Z-0700", param)
}

// validateConsensusAddress parses a consensus oasis address url parameter.
func validateConsensusAddress(param string) (*staking.Address, error) {
	var sender staking.Address
	err := sender.UnmarshalText([]byte(param))
	if err != nil || !sender.IsValid() {
		return nil, apiCommon.ErrBadRequest
	}
	return &sender, nil
}

// validateEventType parses a consensus event type url parameter.
func validateEventType(param string) (*string, error) {
	_, ok := analyzerApi.StringToEvent[param]
	if ok {
		return &param, nil
	}
	return nil, apiCommon.ErrBadRequest
}

// validateEntityID parses a governance entity ID url parameter.
func validateEntityID(param string) (*signature.PublicKey, error) {
	var pk signature.PublicKey
	if err := pk.UnmarshalText([]byte(param)); err != nil || !pk.IsValid() {
		return nil, err
	}
	return &pk, nil
}

// validateNodeID parses a node ID url parameter.
func validateNodeID(param string) (*signature.PublicKey, error) {
	var nid signature.PublicKey
	if err := nid.UnmarshalText([]byte(param)); err != nil || !nid.IsValid() {
		return nil, err
	}
	return &nid, nil
}

// Status returns status information for the Oasis Indexer.
func (c *storageClient) Status(ctx context.Context) (*apiTypes.Status, error) {
	return c.storage.Status(ctx)
}

// Blocks returns a list of consensus blocks.
func (c *storageClient) Blocks(ctx context.Context, r *http.Request) (*apiTypes.BlockList, error) {
	var q storage.BlocksRequest
	params := r.URL.Query()
	if v := params.Get("from"); v != "" {
		from, err := validateInt64(v)
		if err != nil {
			return nil, apiCommon.ErrBadRequest
		}
		q.From = &from
	}
	if v := params.Get("to"); v != "" {
		to, err := validateInt64(v)
		if err != nil {
			return nil, apiCommon.ErrBadRequest
		}
		q.To = &to
	}
	if v := params.Get("after"); v != "" {
		after, err := validateDatetime(v)
		if err != nil {
			return nil, apiCommon.ErrBadRequest
		}
		q.After = &after
	}
	if v := params.Get("before"); v != "" {
		before, err := validateDatetime(v)
		if err != nil {
			return nil, apiCommon.ErrBadRequest
		}
		q.Before = &before
	}

	p, err := apiCommon.NewPagination(r)
	if err != nil {
		c.logger.Info("pagination failed",
			"request_id", ctx.Value(common.RequestIDContextKey),
			"err", err.Error(),
		)
		return nil, apiCommon.ErrBadRequest
	}

	return c.storage.Blocks(ctx, &q, &p)
}

// Block returns a consensus block. This endpoint's responses are cached.
func (c *storageClient) Block(ctx context.Context, r *http.Request) (*apiTypes.Block, error) {
	var q storage.BlockRequest

	v := chi.URLParam(r, "height")
	if v == "" {
		c.logger.Info("missing request parameter(s)")
		return nil, apiCommon.ErrBadRequest
	}
	height, err := validateInt64(v)
	if err != nil {
		return nil, apiCommon.ErrBadRequest
	}
	q.Height = &height

	return c.storage.Block(ctx, &q)
}

// Transactions returns a list of consensus transactions.
func (c *storageClient) Transactions(ctx context.Context, r *http.Request) (*apiTypes.TransactionList, error) {
	var q storage.TransactionsRequest
	params := r.URL.Query()
	if v := params.Get("block"); v != "" {
		block, err := validateInt64(v)
		if err != nil {
			return nil, apiCommon.ErrBadRequest
		}
		q.Block = &block
	}
	if v := params.Get("method"); v != "" {
		q.Method = &v
	}
	if v := params.Get("sender"); v != "" {
		sender, err := validateConsensusAddress(v)
		if err != nil {
			c.logger.Info("failed to validate address", "error", err)
			return nil, apiCommon.ErrBadRequest
		}
		q.Sender = sender
	}
	if v := params.Get("rel"); v != "" {
		rel, err := validateConsensusAddress(v)
		if err != nil {
			c.logger.Info("failed to validate address", "error", err)
			return nil, apiCommon.ErrBadRequest
		}
		q.Rel = rel
	}
	if v := params.Get("minFee"); v != "" {
		minFee, err := validateBigInt(v)
		if err != nil {
			return nil, apiCommon.ErrBadRequest
		}
		q.MinFee = minFee
	}
	if v := params.Get("maxFee"); v != "" {
		maxFee, err := validateBigInt(v)
		if err != nil {
			return nil, apiCommon.ErrBadRequest
		}
		q.MaxFee = maxFee
	}
	if v := params.Get("code"); v != "" {
		code, err := validateInt64(v)
		if err != nil {
			return nil, apiCommon.ErrBadRequest
		}
		q.Code = &code
	}

	p, err := apiCommon.NewPagination(r)
	if err != nil {
		c.logger.Info("pagination failed",
			"request_id", ctx.Value(common.RequestIDContextKey),
			"err", err.Error(),
		)
		return nil, apiCommon.ErrBadRequest
	}

	return c.storage.Transactions(ctx, &q, &p)
}

// Transaction returns a consensus transaction.
func (c *storageClient) Transaction(ctx context.Context, r *http.Request) (*apiTypes.Transaction, error) {
	var q storage.TransactionRequest

	txHash := chi.URLParam(r, "tx_hash")
	if txHash == "" {
		c.logger.Info("missing request parameters")
		return nil, apiCommon.ErrBadRequest
	}
	q.TxHash = &txHash

	return c.storage.Transaction(ctx, &q)
}

// Events returns a list of events.
func (c *storageClient) Events(ctx context.Context, r *http.Request) (*apiTypes.EventsList, error) {
	var q storage.EventsRequest

	params := r.URL.Query()
	if v := params.Get("height"); v != "" {
		height, err := validateInt64(v)
		if err != nil {
			return nil, apiCommon.ErrBadRequest
		}
		q.Height = &height
	}
	if v := params.Get("tx_index"); v != "" {
		// If tx_index is provided, height must also be provided.
		if q.Height == nil {
			return nil, apiCommon.ErrBadRequest
		}

		txIndex, err := validateInt32(v)
		if err != nil {
			return nil, apiCommon.ErrBadRequest
		}
		q.TxIndex = &txIndex
	}
	if v := params.Get("tx_hash"); v != "" {
		q.TxHash = &v
	}
	if v := params.Get("rel"); v != "" {
		addr, err := validateConsensusAddress(v)
		if err != nil {
			return nil, apiCommon.ErrBadRequest
		}
		q.Rel = addr
	}
	if v := params.Get("type"); v != "" {
		ty, err := validateEventType(v)
		if err != nil {
			return nil, apiCommon.ErrBadRequest
		}
		q.Type = ty
	}

	p, err := apiCommon.NewPagination(r)
	if err != nil {
		c.logger.Info("pagination failed",
			"request_id", ctx.Value(common.RequestIDContextKey),
			"err", err.Error(),
		)
		return nil, apiCommon.ErrBadRequest
	}

	return c.storage.Events(ctx, &q, &p)
}

// Entities returns a list of registered entities.
func (c *storageClient) Entities(ctx context.Context, r *http.Request) (*apiTypes.EntityList, error) {
	p, err := apiCommon.NewPagination(r)
	if err != nil {
		c.logger.Info("pagination failed",
			"request_id", ctx.Value(common.RequestIDContextKey),
			"err", err.Error(),
		)
		return nil, apiCommon.ErrBadRequest
	}

	return c.storage.Entities(ctx, &p)
}

// Entity returns a registered entity.
func (c *storageClient) Entity(ctx context.Context, r *http.Request) (*apiTypes.Entity, error) {
	var q storage.EntityRequest

	v, err := url.PathUnescape(chi.URLParam(r, "entity_id"))
	if err != nil {
		return nil, apiCommon.ErrBadRequest
	}
	entityID, err := validateEntityID(v)
	if err != nil {
		c.logger.Info("failed to validate entity id", "error", err)
		return nil, apiCommon.ErrBadRequest
	}
	q.EntityID = entityID

	return c.storage.Entity(ctx, &q)
}

// EntityNodes returns a list of nodes controlled by the provided entity.
func (c *storageClient) EntityNodes(ctx context.Context, r *http.Request) (*apiTypes.NodeList, error) {
	var q storage.EntityNodesRequest

	v, err := url.PathUnescape(chi.URLParam(r, "entity_id"))
	if err != nil {
		return nil, apiCommon.ErrBadRequest
	}
	entityID, err := validateEntityID(v)
	if err != nil {
		c.logger.Info("failed to validate entity id", "error", err)
		return nil, apiCommon.ErrBadRequest
	}
	q.EntityID = entityID

	p, err := apiCommon.NewPagination(r)
	if err != nil {
		c.logger.Info("pagination failed",
			"request_id", ctx.Value(common.RequestIDContextKey),
			"err", err.Error(),
		)
		return nil, apiCommon.ErrBadRequest
	}

	return c.storage.EntityNodes(ctx, &q, &p)
}

// EntityNode returns a node controlled by the provided entity.
func (c *storageClient) EntityNode(ctx context.Context, r *http.Request) (*apiTypes.Node, error) {
	var q storage.EntityNodeRequest
	v, err := url.PathUnescape(chi.URLParam(r, "entity_id"))
	if err != nil {
		return nil, apiCommon.ErrBadRequest
	}
	entityID, err := validateEntityID(v)
	if err != nil {
		c.logger.Info("failed to validate entity id", "error", err)
		return nil, apiCommon.ErrBadRequest
	}
	q.EntityID = entityID
	v, err = url.PathUnescape(chi.URLParam(r, "node_id"))
	if err != nil {
		return nil, apiCommon.ErrBadRequest
	}
	nodeID, err := validateNodeID(v)
	if err != nil {
		c.logger.Info("failed to validate node_id", "error", err)
		return nil, apiCommon.ErrBadRequest
	}
	q.NodeID = nodeID

	return c.storage.EntityNode(ctx, &q)
}

// Accounts returns a list of consensus accounts.
func (c *storageClient) Accounts(ctx context.Context, r *http.Request) (*apiTypes.AccountList, error) {
	var q storage.AccountsRequest
	params := r.URL.Query()

	if v := params.Get("minAvailable"); v != "" {
		minAvailable, err := validateBigInt(v)
		if err != nil {
			return nil, apiCommon.ErrBadRequest
		}
		q.MinAvailable = minAvailable
	}
	if v := params.Get("maxAvailable"); v != "" {
		maxAvailable, err := validateBigInt(v)
		if err != nil {
			return nil, apiCommon.ErrBadRequest
		}
		q.MaxAvailable = maxAvailable
	}
	if v := params.Get("minEscrow"); v != "" {
		minEscrow, err := validateBigInt(v)
		if err != nil {
			return nil, apiCommon.ErrBadRequest
		}
		q.MinEscrow = minEscrow
	}
	if v := params.Get("maxEscrow"); v != "" {
		maxEscrow, err := validateBigInt(v)
		if err != nil {
			return nil, apiCommon.ErrBadRequest
		}
		q.MaxEscrow = maxEscrow
	}
	if v := params.Get("minDebonding"); v != "" {
		minDebonding, err := validateBigInt(v)
		if err != nil {
			return nil, apiCommon.ErrBadRequest
		}
		q.MinDebonding = minDebonding
	}
	if v := params.Get("maxDebonding"); v != "" {
		maxDebonding, err := validateBigInt(v)
		if err != nil {
			return nil, apiCommon.ErrBadRequest
		}
		q.MaxDebonding = maxDebonding
	}
	if v := params.Get("minTotalBalance"); v != "" {
		minTotalBalance, err := validateBigInt(v)
		if err != nil {
			return nil, apiCommon.ErrBadRequest
		}
		q.MinTotalBalance = minTotalBalance
	}
	if v := params.Get("maxTotalBalance"); v != "" {
		maxTotalBalance, err := validateBigInt(v)
		if err != nil {
			return nil, apiCommon.ErrBadRequest
		}
		q.MaxTotalBalance = maxTotalBalance
	}

	p, err := apiCommon.NewPagination(r)
	if err != nil {
		c.logger.Info("pagination failed",
			"request_id", ctx.Value(common.RequestIDContextKey),
			"err", err.Error(),
		)
		return nil, apiCommon.ErrBadRequest
	}

	return c.storage.Accounts(ctx, &q, &p)
}

// Account returns a consensus account.
func (c *storageClient) Account(ctx context.Context, r *http.Request) (*apiTypes.Account, error) {
	var q storage.AccountRequest

	v := chi.URLParam(r, "address")
	if v == "" {
		c.logger.Info("missing required parameters")
		return nil, apiCommon.ErrBadRequest
	}
	address, err := validateConsensusAddress(v)
	if err != nil {
		c.logger.Info("failed to validate address", "error", err)
		return nil, apiCommon.ErrBadRequest
	}
	q.Address = address

	return c.storage.Account(ctx, &q)
}

// Delegations returns a list of delegations.
func (c *storageClient) Delegations(ctx context.Context, r *http.Request) (*apiTypes.DelegationList, error) {
	var q storage.DelegationsRequest

	v := chi.URLParam(r, "address")
	if v == "" {
		c.logger.Info("missing required parameters")
		return nil, apiCommon.ErrBadRequest
	}
	address, err := validateConsensusAddress(v)
	if err != nil {
		c.logger.Info("failed to validate address", "error", err)
		return nil, apiCommon.ErrBadRequest
	}
	q.Address = address

	p, err := apiCommon.NewPagination(r)
	if err != nil {
		c.logger.Info("pagination failed",
			"request_id", ctx.Value(common.RequestIDContextKey),
			"err", err.Error(),
		)
		return nil, apiCommon.ErrBadRequest
	}

	return c.storage.Delegations(ctx, &q, &p)
}

// DebondingDelegations returns a list of debonding delegations.
func (c *storageClient) DebondingDelegations(ctx context.Context, r *http.Request) (*apiTypes.DebondingDelegationList, error) {
	var q storage.DebondingDelegationsRequest

	v := chi.URLParam(r, "address")
	if v == "" {
		c.logger.Info("missing required parameters")
		return nil, apiCommon.ErrBadRequest
	}
	address, err := validateConsensusAddress(v)
	if err != nil {
		c.logger.Info("failed to validate address", "error", err)
		return nil, apiCommon.ErrBadRequest
	}
	q.Address = address

	p, err := apiCommon.NewPagination(r)
	if err != nil {
		c.logger.Info("pagination failed",
			"request_id", ctx.Value(common.RequestIDContextKey),
			"err", err.Error(),
		)
		return nil, apiCommon.ErrBadRequest
	}

	return c.storage.DebondingDelegations(ctx, &q, &p)
}

// Epochs returns a list of consensus epochs.
func (c *storageClient) Epochs(ctx context.Context, r *http.Request) (*apiTypes.EpochList, error) {
	p, err := apiCommon.NewPagination(r)
	if err != nil {
		c.logger.Info("pagination failed",
			"request_id", ctx.Value(common.RequestIDContextKey),
			"err", err.Error(),
		)
		return nil, apiCommon.ErrBadRequest
	}

	return c.storage.Epochs(ctx, &p)
}

// Epoch returns a consensus epoch.
func (c *storageClient) Epoch(ctx context.Context, r *http.Request) (*apiTypes.Epoch, error) {
	var q storage.EpochRequest

	v := chi.URLParam(r, "epoch")
	if v == "" {
		c.logger.Info("missing required parameters")
		return nil, apiCommon.ErrBadRequest
	}
	epoch, err := validateInt64(v)
	if err != nil {
		return nil, apiCommon.ErrBadRequest
	}
	q.Epoch = &epoch

	return c.storage.Epoch(ctx, &q)
}

// Proposals returns a list of governance proposals.
func (c *storageClient) Proposals(ctx context.Context, r *http.Request) (*apiTypes.ProposalList, error) {
	var q storage.ProposalsRequest
	params := r.URL.Query()

	if v := params.Get("submitter"); v != "" {
		submitter, err := validateConsensusAddress(v)
		if err != nil {
			c.logger.Info("failed to validate address", "error", err)
			return nil, apiCommon.ErrBadRequest
		}
		q.Submitter = submitter
	}
	if v := params.Get("state"); v != "" {
		var state *governance.ProposalState
		if err := state.UnmarshalText([]byte(v)); err != nil {
			return nil, apiCommon.ErrBadRequest
		}
		q.State = state
	}

	p, err := apiCommon.NewPagination(r)
	if err != nil {
		c.logger.Info("pagination failed",
			"request_id", ctx.Value(common.RequestIDContextKey),
			"err", err.Error(),
		)
		return nil, apiCommon.ErrBadRequest
	}

	return c.storage.Proposals(ctx, &q, &p)
}

// Proposal returns a governance proposal.
func (c *storageClient) Proposal(ctx context.Context, r *http.Request) (*apiTypes.Proposal, error) {
	var q storage.ProposalRequest

	v := chi.URLParam(r, "proposal_id")
	if v == "" {
		c.logger.Info("missing required parameters")
		return nil, apiCommon.ErrBadRequest
	}
	proposalID, err := validateUint64(v)
	if err != nil {
		return nil, apiCommon.ErrBadRequest
	}
	q.ProposalID = &proposalID

	return c.storage.Proposal(ctx, &q)
}

// ProposalVotes returns votes for a governance proposal.
func (c *storageClient) ProposalVotes(ctx context.Context, r *http.Request) (*apiTypes.ProposalVotes, error) {
	var q storage.ProposalVotesRequest

	v := chi.URLParam(r, "proposal_id")
	if v == "" {
		c.logger.Info("missing required parameters")
		return nil, apiCommon.ErrBadRequest
	}
	proposalID, err := validateUint64(v)
	if err != nil {
		return nil, apiCommon.ErrBadRequest
	}
	q.ProposalID = &proposalID

	p, err := apiCommon.NewPagination(r)
	if err != nil {
		c.logger.Info("pagination failed",
			"request_id", ctx.Value(common.RequestIDContextKey),
			"err", err.Error(),
		)
		return nil, apiCommon.ErrBadRequest
	}

	return c.storage.ProposalVotes(ctx, &q, &p)
}

// Validators returns a list of validators.
func (c *storageClient) Validators(ctx context.Context, r *http.Request) (*apiTypes.ValidatorList, error) {
	order := "voting_power"
	p := apiCommon.Pagination{
		Order:  &order,
		Limit:  1000,
		Offset: 0,
	}

	return c.storage.Validators(ctx, &p)
}

// Validator returns a single validator.
func (c *storageClient) Validator(ctx context.Context, r *http.Request) (*apiTypes.Validator, error) {
	var q storage.ValidatorRequest

	v, err := url.PathUnescape(chi.URLParam(r, "entity_id"))
	if err != nil {
		return nil, apiCommon.ErrBadRequest
	}
	entityID, err := validateEntityID(v)
	if err != nil {
		c.logger.Info("failed to validate entity id", "error", err)
		return nil, apiCommon.ErrBadRequest
	}
	q.EntityID = entityID

	return c.storage.Validator(ctx, &q)
}

// RuntimeBlocks returns a list of a runtime's blocks.
func (c *storageClient) RuntimeBlocks(ctx context.Context, r *http.Request) (*apiTypes.RuntimeBlockList, error) {
	var q storage.RuntimeBlocksRequest
	params := r.URL.Query()
	if v := params.Get("from"); v != "" {
		from, err := validateInt64(v)
		if err != nil {
			return nil, apiCommon.ErrBadRequest
		}
		q.From = &from
	}
	if v := params.Get("to"); v != "" {
		to, err := validateInt64(v)
		if err != nil {
			return nil, apiCommon.ErrBadRequest
		}
		q.To = &to
	}
	if v := params.Get("after"); v != "" {
		after, err := validateDatetime(v)
		if err != nil {
			return nil, apiCommon.ErrBadRequest
		}
		q.After = &after
	}
	if v := params.Get("before"); v != "" {
		before, err := validateDatetime(v)
		if err != nil {
			return nil, apiCommon.ErrBadRequest
		}
		q.Before = &before
	}

	p, err := apiCommon.NewPagination(r)
	if err != nil {
		c.logger.Info("pagination failed",
			"request_id", ctx.Value(common.RequestIDContextKey),
			"err", err.Error(),
		)
		return nil, apiCommon.ErrBadRequest
	}

	return c.storage.RuntimeBlocks(ctx, &q, &p)
}

// RuntimeTransactions returns a list of runtime transactions.
func (c *storageClient) RuntimeTransactions(ctx context.Context, r *http.Request) (*apiTypes.RuntimeTransactionList, error) {
	var q storage.RuntimeTransactionsRequest
	params := r.URL.Query()
	if v := params.Get("block"); v != "" {
		block, err := validateInt64(v)
		if err != nil {
			return nil, apiCommon.ErrBadRequest
		}
		q.Block = &block
	}

	p, err := apiCommon.NewPagination(r)
	if err != nil {
		c.logger.Info("pagination failed",
			"request_id", ctx.Value(common.RequestIDContextKey),
			"err", err.Error(),
		)
		return nil, apiCommon.ErrBadRequest
	}

	storageTransactions, err := c.storage.RuntimeTransactions(ctx, &q, &p)
	if err != nil {
		return nil, err
	}

	var apiTransactions apiTypes.RuntimeTransactionList
	for _, storageTransaction := range storageTransactions.Transactions {
		apiTransaction, err2 := renderRuntimeTransaction(storageTransaction)
		if err2 != nil {
			return nil, fmt.Errorf("round %d tx %d: %w", storageTransaction.Round, storageTransaction.Index, err2)
		}
		apiTransactions.Transactions = append(apiTransactions.Transactions, apiTransaction)
	}

	return &apiTransactions, err
}

func (c *storageClient) RuntimeTokens(ctx context.Context, r *http.Request) (*apiTypes.RuntimeTokenList, error) {
	var q storage.RuntimeTokensRequest

	p, err := apiCommon.NewPagination(r)
	if err != nil {
		c.logger.Info("pagination failed",
			"request_id", ctx.Value(common.RequestIDContextKey),
			"err", err.Error(),
		)
		return nil, apiCommon.ErrBadRequest
	}

	return c.storage.RuntimeTokens(ctx, &q, &p)
}

// TxVolumes returns a list of transaction volumes grouped into fine-grained buckets.
func (c *storageClient) TxVolumes(ctx context.Context, r *http.Request) (*apiTypes.TxVolumeList, error) {
	p, err := apiCommon.NewPagination(r)
	if err != nil {
		c.logger.Info("pagination failed",
			"request_id", ctx.Value(common.RequestIDContextKey),
			"err", err.Error(),
		)
		return nil, apiCommon.ErrBadRequest
	}

	q, err := apiCommon.NewBucketedStatsParams(r)
	if err != nil {
		c.logger.Info("bucket param extraction failed",
			"request_id", ctx.Value(common.RequestIDContextKey),
			"err", err.Error(),
		)
		return nil, apiCommon.ErrBadRequest
	}

	return c.storage.TxVolumes(ctx, &p, &q)
}
