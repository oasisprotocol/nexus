package v1

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/oasisprotocol/oasis-core/go/common/cbor"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/modules/accounts"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/modules/consensusaccounts"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/modules/evm"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/types"

	"github.com/oasisprotocol/oasis-core/go/common/crypto/signature"
	governance "github.com/oasisprotocol/oasis-core/go/governance/api"
	staking "github.com/oasisprotocol/oasis-core/go/staking/api"

	uncategorized "github.com/oasisprotocol/oasis-indexer/analyzer/uncategorized"
	"github.com/oasisprotocol/oasis-indexer/api/common"
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

// validateInt64 parses an int64 url parameter.
func validateInt64(param string) (int64, error) {
	return strconv.ParseInt(param, 10, 64)
}

// validateUint64 parses an int64 url parameter.
func validateUint64(param string) (uint64, error) {
	return strconv.ParseUint(param, 10, 64)
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
		return nil, common.ErrBadRequest
	}
	return &sender, nil
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
func (c *storageClient) Status(ctx context.Context) (*storage.Status, error) {
	return c.storage.Status(ctx)
}

// Blocks returns a list of consensus blocks.
func (c *storageClient) Blocks(ctx context.Context, r *http.Request) (*storage.BlockList, error) {
	var q storage.BlocksRequest
	params := r.URL.Query()
	if v := params.Get("from"); v != "" {
		from, err := validateInt64(v)
		if err != nil {
			return nil, common.ErrBadRequest
		}
		q.From = &from
	}
	if v := params.Get("to"); v != "" {
		to, err := validateInt64(v)
		if err != nil {
			return nil, common.ErrBadRequest
		}
		q.To = &to
	}
	if v := params.Get("after"); v != "" {
		after, err := validateDatetime(v)
		if err != nil {
			return nil, common.ErrBadRequest
		}
		q.After = &after
	}
	if v := params.Get("before"); v != "" {
		before, err := validateDatetime(v)
		if err != nil {
			return nil, common.ErrBadRequest
		}
		q.Before = &before
	}

	p, err := common.NewPagination(r)
	if err != nil {
		c.logger.Info("pagination failed",
			"request_id", ctx.Value(storage.RequestIDContextKey),
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
	height, err := validateInt64(v)
	if err != nil {
		return nil, common.ErrBadRequest
	}
	q.Height = &height

	return c.storage.Block(ctx, &q)
}

// Transactions returns a list of consensus transactions.
func (c *storageClient) Transactions(ctx context.Context, r *http.Request) (*storage.TransactionList, error) {
	var q storage.TransactionsRequest
	params := r.URL.Query()
	if v := params.Get("block"); v != "" {
		block, err := validateInt64(v)
		if err != nil {
			return nil, common.ErrBadRequest
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
			return nil, common.ErrBadRequest
		}
		q.Sender = sender
	}
	if v := params.Get("minFee"); v != "" {
		minFee, err := validateInt64(v)
		if err != nil {
			return nil, common.ErrBadRequest
		}
		q.MinFee = &minFee
	}
	if v := params.Get("maxFee"); v != "" {
		maxFee, err := validateInt64(v)
		if err != nil {
			return nil, common.ErrBadRequest
		}
		q.MaxFee = &maxFee
	}
	if v := params.Get("code"); v != "" {
		code, err := validateInt64(v)
		if err != nil {
			return nil, common.ErrBadRequest
		}
		q.Code = &code
	}

	p, err := common.NewPagination(r)
	if err != nil {
		c.logger.Info("pagination failed",
			"request_id", ctx.Value(storage.RequestIDContextKey),
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

	return c.storage.Transaction(ctx, &q)
}

// Entities returns a list of registered entities.
func (c *storageClient) Entities(ctx context.Context, r *http.Request) (*storage.EntityList, error) {
	p, err := common.NewPagination(r)
	if err != nil {
		c.logger.Info("pagination failed",
			"request_id", ctx.Value(storage.RequestIDContextKey),
			"err", err.Error(),
		)
		return nil, common.ErrBadRequest
	}

	return c.storage.Entities(ctx, &p)
}

// Entity returns a registered entity.
func (c *storageClient) Entity(ctx context.Context, r *http.Request) (*storage.Entity, error) {
	var q storage.EntityRequest

	v, err := url.PathUnescape(chi.URLParam(r, "entity_id"))
	if err != nil {
		return nil, common.ErrBadRequest
	}
	entityID, err := validateEntityID(v)
	if err != nil {
		c.logger.Info("failed to validate entity id", "error", err)
		return nil, common.ErrBadRequest
	}
	q.EntityID = entityID

	return c.storage.Entity(ctx, &q)
}

// EntityNodes returns a list of nodes controlled by the provided entity.
func (c *storageClient) EntityNodes(ctx context.Context, r *http.Request) (*storage.NodeList, error) {
	var q storage.EntityNodesRequest

	v, err := url.PathUnescape(chi.URLParam(r, "entity_id"))
	if err != nil {
		return nil, common.ErrBadRequest
	}
	entityID, err := validateEntityID(v)
	if err != nil {
		c.logger.Info("failed to validate entity id", "error", err)
		return nil, common.ErrBadRequest
	}
	q.EntityID = entityID

	p, err := common.NewPagination(r)
	if err != nil {
		c.logger.Info("pagination failed",
			"request_id", ctx.Value(storage.RequestIDContextKey),
			"err", err.Error(),
		)
		return nil, common.ErrBadRequest
	}

	return c.storage.EntityNodes(ctx, &q, &p)
}

// EntityNode returns a node controlled by the provided entity.
func (c *storageClient) EntityNode(ctx context.Context, r *http.Request) (*storage.Node, error) {
	var q storage.EntityNodeRequest
	v, err := url.PathUnescape(chi.URLParam(r, "entity_id"))
	if err != nil {
		return nil, common.ErrBadRequest
	}
	entityID, err := validateEntityID(v)
	if err != nil {
		c.logger.Info("failed to validate entity id", "error", err)
		return nil, common.ErrBadRequest
	}
	q.EntityID = entityID
	v, err = url.PathUnescape(chi.URLParam(r, "node_id"))
	if err != nil {
		return nil, common.ErrBadRequest
	}
	nodeID, err := validateNodeID(v)
	if err != nil {
		c.logger.Info("failed to validate node_id", "error", err)
		return nil, common.ErrBadRequest
	}
	q.NodeID = nodeID

	return c.storage.EntityNode(ctx, &q)
}

// Accounts returns a list of consensus accounts.
func (c *storageClient) Accounts(ctx context.Context, r *http.Request) (*storage.AccountList, error) {
	var q storage.AccountsRequest
	params := r.URL.Query()

	if v := params.Get("minAvailable"); v != "" {
		minAvailable, err := validateInt64(v)
		if err != nil {
			return nil, common.ErrBadRequest
		}
		q.MinAvailable = &minAvailable
	}
	if v := params.Get("maxAvailable"); v != "" {
		maxAvailable, err := validateInt64(v)
		if err != nil {
			return nil, common.ErrBadRequest
		}
		q.MaxAvailable = &maxAvailable
	}
	if v := params.Get("minEscrow"); v != "" {
		minEscrow, err := validateInt64(v)
		if err != nil {
			return nil, common.ErrBadRequest
		}
		q.MinEscrow = &minEscrow
	}
	if v := params.Get("maxEscrow"); v != "" {
		maxEscrow, err := validateInt64(v)
		if err != nil {
			return nil, common.ErrBadRequest
		}
		q.MaxEscrow = &maxEscrow
	}
	if v := params.Get("minDebonding"); v != "" {
		minDebonding, err := validateInt64(v)
		if err != nil {
			return nil, common.ErrBadRequest
		}
		q.MinDebonding = &minDebonding
	}
	if v := params.Get("maxDebonding"); v != "" {
		maxDebonding, err := validateInt64(v)
		if err != nil {
			return nil, common.ErrBadRequest
		}
		q.MaxDebonding = &maxDebonding
	}
	if v := params.Get("minTotalBalance"); v != "" {
		minTotalBalance, err := validateInt64(v)
		if err != nil {
			return nil, common.ErrBadRequest
		}
		q.MinTotalBalance = &minTotalBalance
	}
	if v := params.Get("maxTotalBalance"); v != "" {
		maxTotalBalance, err := validateInt64(v)
		if err != nil {
			return nil, common.ErrBadRequest
		}
		q.MaxTotalBalance = &maxTotalBalance
	}

	p, err := common.NewPagination(r)
	if err != nil {
		c.logger.Info("pagination failed",
			"request_id", ctx.Value(storage.RequestIDContextKey),
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
	address, err := validateConsensusAddress(v)
	if err != nil {
		c.logger.Info("failed to validate address", "error", err)
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
	address, err := validateConsensusAddress(v)
	if err != nil {
		c.logger.Info("failed to validate address", "error", err)
		return nil, common.ErrBadRequest
	}
	q.Address = address

	p, err := common.NewPagination(r)
	if err != nil {
		c.logger.Info("pagination failed",
			"request_id", ctx.Value(storage.RequestIDContextKey),
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
	address, err := validateConsensusAddress(v)
	if err != nil {
		c.logger.Info("failed to validate address", "error", err)
		return nil, common.ErrBadRequest
	}
	q.Address = address

	p, err := common.NewPagination(r)
	if err != nil {
		c.logger.Info("pagination failed",
			"request_id", ctx.Value(storage.RequestIDContextKey),
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
			"request_id", ctx.Value(storage.RequestIDContextKey),
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
	epoch, err := validateInt64(v)
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
		submitter, err := validateConsensusAddress(v)
		if err != nil {
			c.logger.Info("failed to validate address", "error", err)
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
			"request_id", ctx.Value(storage.RequestIDContextKey),
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
	proposalID, err := validateUint64(v)
	if err != nil {
		return nil, common.ErrBadRequest
	}
	q.ProposalID = &proposalID

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
	proposalID, err := validateUint64(v)
	if err != nil {
		return nil, common.ErrBadRequest
	}
	q.ProposalID = &proposalID

	p, err := common.NewPagination(r)
	if err != nil {
		c.logger.Info("pagination failed",
			"request_id", ctx.Value(storage.RequestIDContextKey),
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

	v, err := url.PathUnescape(chi.URLParam(r, "entity_id"))
	if err != nil {
		return nil, common.ErrBadRequest
	}
	entityID, err := validateEntityID(v)
	if err != nil {
		c.logger.Info("failed to validate entity id", "error", err)
		return nil, common.ErrBadRequest
	}
	q.EntityID = entityID

	return c.storage.Validator(ctx, &q)
}

// RuntimeBlocks returns a list of a runtime's blocks.
func (c *storageClient) RuntimeBlocks(ctx context.Context, r *http.Request) (*storage.RuntimeBlockList, error) {
	var q storage.RuntimeBlocksRequest
	params := r.URL.Query()
	if v := params.Get("from"); v != "" {
		from, err := validateInt64(v)
		if err != nil {
			return nil, common.ErrBadRequest
		}
		q.From = &from
	}
	if v := params.Get("to"); v != "" {
		to, err := validateInt64(v)
		if err != nil {
			return nil, common.ErrBadRequest
		}
		q.To = &to
	}
	if v := params.Get("after"); v != "" {
		after, err := validateDatetime(v)
		if err != nil {
			return nil, common.ErrBadRequest
		}
		q.After = &after
	}
	if v := params.Get("before"); v != "" {
		before, err := validateDatetime(v)
		if err != nil {
			return nil, common.ErrBadRequest
		}
		q.Before = &before
	}

	p, err := common.NewPagination(r)
	if err != nil {
		c.logger.Info("pagination failed",
			"request_id", ctx.Value(storage.RequestIDContextKey),
			"err", err.Error(),
		)
		return nil, common.ErrBadRequest
	}

	return c.storage.RuntimeBlocks(ctx, &q, &p)
}

// RuntimeTransactions returns a list of runtime transactions.
func (c *storageClient) RuntimeTransactions(ctx context.Context, r *http.Request) (*RuntimeTransactionList, error) {
	var q storage.RuntimeTransactionsRequest
	params := r.URL.Query()
	if v := params.Get("block"); v != "" {
		block, err := validateInt64(v)
		if err != nil {
			return nil, common.ErrBadRequest
		}
		q.Block = &block
	}

	p, err := common.NewPagination(r)
	if err != nil {
		c.logger.Info("pagination failed",
			"request_id", ctx.Value(storage.RequestIDContextKey),
			"err", err.Error(),
		)
		return nil, common.ErrBadRequest
	}

	storageTransactions, err := c.storage.RuntimeTransactions(ctx, &q, &p)
	if err != nil {
		return nil, err
	}

	var apiTransactions RuntimeTransactionList
	for _, storageTransaction := range storageTransactions.Transactions {
		var utx types.UnverifiedTransaction
		if err = cbor.Unmarshal(storageTransaction.Raw, &utx); err != nil {
			return nil, fmt.Errorf("round %d tx %d utx unmarshal: %w", storageTransaction.Round, storageTransaction.Index, err)
		}
		tx, err := uncategorized.OpenUtxNoVerify(&utx)
		if err != nil {
			return nil, fmt.Errorf("round %d tx %d utx open no verify: %w", storageTransaction.Round, storageTransaction.Index, err)
		}
		sender0, err := uncategorized.StringifyAddressSpec(&tx.AuthInfo.SignerInfo[0].AddressSpec)
		if err != nil {
			return nil, fmt.Errorf("round %d tx %d signer 0: %w", storageTransaction.Round, storageTransaction.Index, err)
		}
		var cr types.CallResult
		if err = cbor.Unmarshal(storageTransaction.ResultRaw, &cr); err != nil {
			return nil, fmt.Errorf("round %d tx %d result unmarshal: %w", storageTransaction.Round, storageTransaction.Index, err)
		}
		apiTransaction := RuntimeTransaction{
			Round:   storageTransaction.Round,
			Index:   storageTransaction.Index,
			Hash:    storageTransaction.Hash,
			EthHash: storageTransaction.EthHash,
			// TODO: Get timestamp from that round's block
			Sender0:   sender0,
			Nonce0:    tx.AuthInfo.SignerInfo[0].Nonce,
			FeeAmount: tx.AuthInfo.Fee.Amount.Amount.String(),
			FeeGas:    tx.AuthInfo.Fee.Gas,
			Method:    tx.Call.Method,
			Body:      tx.Call.Body,
			Success:   cr.IsSuccess(),
		}
		if err = uncategorized.VisitCall(&tx.Call, &cr, &uncategorized.CallHandler{
			AccountsTransfer: func(body *accounts.Transfer) error {
				to, err2 := uncategorized.StringifySdkAddress(&body.To)
				if err2 != nil {
					return fmt.Errorf("to: %w", err2)
				}
				apiTransaction.To = &to
				amount, err2 := uncategorized.StringifyNativeDenomination(&body.Amount)
				if err2 != nil {
					return fmt.Errorf("amount: %w", err2)
				}
				apiTransaction.Amount = &amount
				return nil
			},
			ConsensusAccountsDeposit: func(body *consensusaccounts.Deposit) error {
				to, err2 := uncategorized.StringifySdkAddress(body.To)
				if err2 != nil {
					return fmt.Errorf("to: %w", err2)
				}
				apiTransaction.To = &to
				amount, err2 := uncategorized.StringifyNativeDenomination(&body.Amount)
				if err2 != nil {
					return fmt.Errorf("amount: %w", err2)
				}
				apiTransaction.Amount = &amount
				return nil
			},
			ConsensusAccountsWithdraw: func(body *consensusaccounts.Withdraw) error {
				to, err2 := uncategorized.StringifySdkAddress(body.To)
				if err2 != nil {
					return fmt.Errorf("to: %w", err2)
				}
				// todo: is this right? we don't otherwise register this off-chain .To
				apiTransaction.To = &to
				// todo: ensure native denomination?
				amount := body.Amount.Amount.String()
				apiTransaction.Amount = &amount
				return nil
			},
			EvmCreate: func(body *evm.Create, ok *[]byte) error {
				if !cr.IsUnknown() && cr.IsSuccess() && len(*ok) == 32 {
					// todo: is this rigorous enough?
					to, err2 := uncategorized.StringifyEthAddress(uncategorized.SliceEthAddress(*ok))
					if err2 != nil {
						return fmt.Errorf("created contract: %w", err2)
					}
					apiTransaction.To = &to
				}
				amount := uncategorized.StringifyBytes(body.Value)
				apiTransaction.Amount = &amount
				return nil
			},
			EvmCall: func(body *evm.Call, ok *[]byte) error {
				to, err2 := uncategorized.StringifyEthAddress(body.Address)
				if err2 != nil {
					return fmt.Errorf("to: %w", err2)
				}
				apiTransaction.To = &to
				amount := uncategorized.StringifyBytes(body.Value)
				apiTransaction.Amount = &amount
				return nil
			},
		}); err != nil {
			return nil, fmt.Errorf("round %d tx %d: %w", storageTransaction.Round, storageTransaction.Index, err)
		}
		apiTransactions.Transactions = append(apiTransactions.Transactions, apiTransaction)
	}

	return &apiTransactions, err
}

// TransactionsPerSecond returns a list of tps checkpoint values.
func (c *storageClient) TransactionsPerSecond(ctx context.Context, r *http.Request) (*storage.TpsCheckpointList, error) {
	p, err := common.NewPagination(r)
	if err != nil {
		c.logger.Info("pagination failed",
			"request_id", ctx.Value(storage.RequestIDContextKey),
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
			"request_id", ctx.Value(storage.RequestIDContextKey),
			"err", err.Error(),
		)
		return nil, common.ErrBadRequest
	}

	return c.storage.DailyVolumes(ctx, &p)
}
