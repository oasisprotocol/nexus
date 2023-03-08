package client

import (
	"context"
	"fmt"
	"math/big"
	"time"

	"github.com/dgraph-io/ristretto"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	beacon "github.com/oasisprotocol/oasis-core/go/beacon/api"
	"github.com/oasisprotocol/oasis-core/go/common/crypto/signature"
	oasisErrors "github.com/oasisprotocol/oasis-core/go/common/errors"
	staking "github.com/oasisprotocol/oasis-core/go/staking/api"
	"github.com/oasisprotocol/oasis-indexer/analyzer/util"
	apiCommon "github.com/oasisprotocol/oasis-indexer/api"
	apiTypes "github.com/oasisprotocol/oasis-indexer/api/v1/types"
	common "github.com/oasisprotocol/oasis-indexer/common"
	"github.com/oasisprotocol/oasis-indexer/log"
	"github.com/oasisprotocol/oasis-indexer/storage"
	"github.com/oasisprotocol/oasis-indexer/storage/client/queries"
)

const (
	blockCost = 1
	txCost    = 1

	maxTotalCount = 1000
)

// StorageClient is a wrapper around a storage.TargetStorage
// with knowledge of network semantics.
type StorageClient struct {
	chainID string
	db      storage.TargetStorage

	blockCache *ristretto.Cache
	txCache    *ristretto.Cache

	logger *log.Logger
}

type rowsWithCount struct {
	rows                pgx.Rows
	totalCount          uint64
	isTotalCountClipped bool
}

func toString(b *BigInt) *string {
	if b == nil {
		return nil
	}
	s := b.String()
	return &s
}

func runtimeFromCtx(ctx context.Context) string {
	// Extract the runtime name. It's populated by a middleware based on the URL.
	runtime, ok := ctx.Value(common.RuntimeContextKey).(string)
	if !ok {
		// We're being called from a non-runtime-specific endpoint.
		// This shouldn't happen. Return a dummy value, let the caller deal with it.
		return "__NO_RUNTIME__"
	}
	return runtime
}

// NewStorageClient creates a new storage client.
func NewStorageClient(chainID string, db storage.TargetStorage, l *log.Logger) (*StorageClient, error) {
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
	return &StorageClient{chainID, db, blockCache, txCache, l}, nil
}

// Shutdown closes the backing TargetStorage.
func (c *StorageClient) Shutdown() {
	c.db.Shutdown()
}

// Wraps an error into one of the error types defined by the `common` package, if applicable.
func wrapError(err error) error {
	if err == pgx.ErrNoRows {
		return apiCommon.ErrNotFound
	}
	return apiCommon.ErrStorageError{Err: err}
}

// For queries that return multiple rows, returns the rows for a given query, as well as
// the total count of matching records, i.e. the number of rows the query would return
// with limit=infinity.
// Assumes that the last two query parameters are limit and offset.
// The total count is capped by an internal limit for performance reasons.
func (c *StorageClient) withTotalCount(ctx context.Context, sql string, args ...interface{}) (*rowsWithCount, error) {
	var totalCount uint64
	if len(args) < 2 {
		return nil, fmt.Errorf("list queries must have at least two params (limit and offset)")
	}

	// A note on ordering: We query the totalCount before querying for the rows in order to
	// avoid deadlocks. The row returned by the totalCount query is `Scan`-ed immediately
	// and thus the underlying db connection is also released. However, the `rows` are
	// `Scan`-ed in the calling function, which means that the underlying db connection is
	// held (and unavailable to other goroutines) in the meantime.
	origLimit := args[len(args)-2]
	// Temporarily set limit to just high enough to learn
	// if there are >maxTotalCount matching items in the DB.
	args[len(args)-2] = maxTotalCount + 1
	if err := c.db.QueryRow(
		ctx,
		queries.TotalCountQuery(sql),
		args...,
	).Scan(&totalCount); err != nil {
		return nil, wrapError(err)
	}
	clipped := totalCount == maxTotalCount+1
	if clipped {
		totalCount = maxTotalCount
	}

	args[len(args)-2] = origLimit
	rows, err := c.db.Query(
		ctx,
		sql,
		args...,
	)
	if err != nil {
		return nil, err
	}

	return &rowsWithCount{
		rows:                rows,
		totalCount:          totalCount,
		isTotalCountClipped: clipped,
	}, nil
}

// Status returns status information for the Oasis Indexer.
func (c *StorageClient) Status(ctx context.Context) (*Status, error) {
	s := Status{
		LatestChainID: c.chainID,
	}
	if err := c.db.QueryRow(
		ctx,
		queries.Status,
	).Scan(&s.LatestBlock, &s.LatestUpdate); err != nil {
		return nil, wrapError(err)
	}
	// oasis-node control status returns time truncated to the second
	// https://github.com/oasisprotocol/oasis-core/blob/5985dc5c2844de28241b7b16b19d91a86e5cbeda/docs/oasis-node/cli.md?plain=1#L41
	s.LatestUpdate = s.LatestUpdate.Truncate(time.Second)

	return &s, nil
}

// Blocks returns a list of consensus blocks.
func (c *StorageClient) Blocks(ctx context.Context, r apiTypes.GetConsensusBlocksParams) (*BlockList, error) {
	res, err := c.withTotalCount(
		ctx,
		queries.Blocks,
		r.From,
		r.To,
		r.After,
		r.Before,
		r.Limit,
		r.Offset,
	)
	if err != nil {
		return nil, wrapError(err)
	}
	defer res.rows.Close()

	bs := BlockList{
		Blocks:              []Block{},
		TotalCount:          res.totalCount,
		IsTotalCountClipped: res.isTotalCountClipped,
	}
	for res.rows.Next() {
		var b Block
		if err := res.rows.Scan(&b.Height, &b.Hash, &b.Timestamp, &b.NumTransactions); err != nil {
			return nil, wrapError(err)
		}
		b.Timestamp = b.Timestamp.UTC()

		bs.Blocks = append(bs.Blocks, b)
	}

	return &bs, nil
}

// Block returns a consensus block. This endpoint is cached.
func (c *StorageClient) Block(ctx context.Context, height int64) (*Block, error) {
	// Check cache
	untypedBlock, ok := c.blockCache.Get(height)
	if ok {
		return untypedBlock.(*Block), nil
	}

	var b Block
	if err := c.db.QueryRow(
		ctx,
		queries.Block,
		height,
	).Scan(&b.Height, &b.Hash, &b.Timestamp, &b.NumTransactions); err != nil {
		return nil, wrapError(err)
	}
	b.Timestamp = b.Timestamp.UTC()

	c.cacheBlock(&b)
	return &b, nil
}

// cacheBlock adds a block to the client's block cache.
func (c *StorageClient) cacheBlock(blk *Block) {
	c.blockCache.Set(blk.Height, blk, blockCost)
}

// Transactions returns a list of consensus transactions.
func (c *StorageClient) Transactions(ctx context.Context, p apiTypes.GetConsensusTransactionsParams) (*TransactionList, error) {
	res, err := c.withTotalCount(
		ctx,
		queries.Transactions,
		p.Block,
		p.Method,
		p.Sender,
		p.Rel,
		toString(p.MinFee),
		toString(p.MaxFee),
		p.Code,
		p.Limit,
		p.Offset,
	)
	if err != nil {
		return nil, wrapError(err)
	}
	defer res.rows.Close()

	ts := TransactionList{
		Transactions:        []Transaction{},
		TotalCount:          res.totalCount,
		IsTotalCountClipped: res.isTotalCountClipped,
	}
	for res.rows.Next() {
		var t Transaction
		var code uint64
		if err := res.rows.Scan(
			&t.Block,
			&t.Index,
			&t.Hash,
			&t.Sender,
			&t.Nonce,
			&t.Fee,
			&t.Method,
			&t.Body,
			&code,
			&t.Timestamp,
		); err != nil {
			return nil, wrapError(err)
		}
		if code == oasisErrors.CodeNoError {
			t.Success = true
		}

		ts.Transactions = append(ts.Transactions, t)
	}

	return &ts, nil
}

// Transaction returns a consensus transaction. This endpoint is cached.
func (c *StorageClient) Transaction(ctx context.Context, txHash string) (*Transaction, error) {
	// Check cache
	untypedTx, ok := c.txCache.Get(txHash)
	if ok {
		return untypedTx.(*Transaction), nil
	}

	var t Transaction
	var code uint64
	if err := c.db.QueryRow(
		ctx,
		queries.Transaction,
		txHash,
	).Scan(
		&t.Block,
		&t.Index,
		&t.Hash,
		&t.Sender,
		&t.Nonce,
		&t.Fee,
		&t.Method,
		&t.Body,
		&code,
		&t.Timestamp,
	); err != nil {
		return nil, wrapError(err)
	}
	if code == oasisErrors.CodeNoError {
		t.Success = true
	}

	c.cacheTx(&t)
	return &t, nil
}

// cacheTx adds a transaction to the client's transaction cache.
func (c *StorageClient) cacheTx(tx *Transaction) {
	c.txCache.Set(tx.Hash, tx, txCost)
}

// Events returns a list of events.
func (c *StorageClient) Events(ctx context.Context, p apiTypes.GetConsensusEventsParams) (*EventList, error) {
	res, err := c.withTotalCount(
		ctx,
		queries.Events,
		p.Block,
		p.TxIndex,
		p.TxHash,
		p.Type,
		p.Rel,
		p.Limit,
		p.Offset,
	)
	if err != nil {
		return nil, wrapError(err)
	}
	defer res.rows.Close()

	es := EventList{
		Events:              []Event{},
		TotalCount:          res.totalCount,
		IsTotalCountClipped: res.isTotalCountClipped,
	}

	for res.rows.Next() {
		var e Event
		if err := res.rows.Scan(&e.Block, &e.TxIndex, &e.TxHash, &e.Type, &e.Body); err != nil {
			return nil, wrapError(err)
		}
		es.Events = append(es.Events, e)
	}

	return &es, nil
}

// Entities returns a list of registered entities.
func (c *StorageClient) Entities(ctx context.Context, p apiTypes.GetConsensusEntitiesParams) (*EntityList, error) {
	res, err := c.withTotalCount(
		ctx,
		queries.Entities,
		p.Limit,
		p.Offset,
	)
	if err != nil {
		return nil, wrapError(err)
	}
	defer res.rows.Close()

	es := EntityList{
		Entities:            []Entity{},
		TotalCount:          res.totalCount,
		IsTotalCountClipped: res.isTotalCountClipped,
	}
	for res.rows.Next() {
		var e Entity
		if err := res.rows.Scan(&e.ID, &e.Address); err != nil {
			return nil, wrapError(err)
		}

		es.Entities = append(es.Entities, e)
	}

	return &es, nil
}

// Entity returns a registered entity.
func (c *StorageClient) Entity(ctx context.Context, entityID signature.PublicKey) (*Entity, error) {
	var e Entity
	if err := c.db.QueryRow(
		ctx,
		queries.Entity,
		entityID.String(),
	).Scan(&e.ID, &e.Address); err != nil {
		return nil, wrapError(err)
	}

	nodeRows, err := c.db.Query(
		ctx,
		queries.EntityNodeIds,
		entityID.String(),
	)
	if err != nil {
		return nil, wrapError(err)
	}
	defer nodeRows.Close()

	for nodeRows.Next() {
		var nid string
		if err := nodeRows.Scan(&nid); err != nil {
			return nil, wrapError(err)
		}

		e.Nodes = append(e.Nodes, nid)
	}

	return &e, nil
}

// EntityNodes returns a list of nodes controlled by the provided entity.
func (c *StorageClient) EntityNodes(ctx context.Context, entityID signature.PublicKey, r apiTypes.GetConsensusEntitiesEntityIdNodesParams) (*NodeList, error) {
	res, err := c.withTotalCount(
		ctx,
		queries.EntityNodes,
		entityID.String(),
		r.Limit,
		r.Offset,
	)
	if err != nil {
		return nil, wrapError(err)
	}
	defer res.rows.Close()

	ns := NodeList{
		Nodes:               []Node{},
		TotalCount:          res.totalCount,
		IsTotalCountClipped: res.isTotalCountClipped,
	}
	for res.rows.Next() {
		var n Node
		if err := res.rows.Scan(
			&n.ID,
			&n.EntityID,
			&n.Expiration,
			&n.TLSPubkey,
			&n.TLSNextPubkey,
			&n.P2PPubkey,
			&n.ConsensusPubkey,
			&n.Roles,
		); err != nil {
			return nil, wrapError(err)
		}

		ns.Nodes = append(ns.Nodes, n)
	}
	ns.EntityID = entityID.String()

	return &ns, nil
}

// EntityNode returns a node controlled by the provided entity.
func (c *StorageClient) EntityNode(ctx context.Context, entityID signature.PublicKey, nodeID signature.PublicKey) (*Node, error) {
	var n Node
	if err := c.db.QueryRow(
		ctx,
		queries.EntityNode,
		entityID.String(),
		nodeID.String(),
	).Scan(
		&n.ID,
		&n.EntityID,
		&n.Expiration,
		&n.TLSPubkey,
		&n.TLSNextPubkey,
		&n.P2PPubkey,
		&n.ConsensusPubkey,
		&n.Roles,
	); err != nil {
		return nil, wrapError(err)
	}

	return &n, nil
}

// Accounts returns a list of consensus accounts.
func (c *StorageClient) Accounts(ctx context.Context, r apiTypes.GetConsensusAccountsParams) (*AccountList, error) {
	res, err := c.withTotalCount(
		ctx,
		queries.Accounts,
		toString(r.MinAvailable),
		toString(r.MaxAvailable),
		toString(r.MinEscrow),
		toString(r.MaxEscrow),
		toString(r.MinDebonding),
		toString(r.MaxDebonding),
		toString(r.MinTotalBalance),
		toString(r.MaxTotalBalance),
		r.Limit,
		r.Offset,
	)
	if err != nil {
		return nil, wrapError(err)
	}
	defer res.rows.Close()

	as := AccountList{
		Accounts:            []Account{},
		TotalCount:          res.totalCount,
		IsTotalCountClipped: res.isTotalCountClipped,
	}
	for res.rows.Next() {
		var a Account
		if err := res.rows.Scan(
			&a.Address,
			&a.Nonce,
			&a.Available,
			&a.Escrow,
			&a.Debonding,
		); err != nil {
			return nil, wrapError(err)
		}

		as.Accounts = append(as.Accounts, a)
	}

	return &as, nil
}

// Account returns a consensus account.
func (c *StorageClient) Account(ctx context.Context, address staking.Address) (*Account, error) {
	// Get basic account info.
	a := Account{
		// Initialize optional fields to empty values to avoid null pointer dereferences
		// when filling them from the database.
		Allowances:                  []Allowance{},
		DelegationsBalance:          &common.BigInt{},
		DebondingDelegationsBalance: &common.BigInt{},
	}
	var delegationsBalanceNum pgtype.Numeric
	var debondingDelegationsBalanceNum pgtype.Numeric
	err := c.db.QueryRow(
		ctx,
		queries.Account,
		address.String(),
	).Scan(
		&a.Address,
		&a.Nonce,
		&a.Available,
		&a.Escrow,
		&a.Debonding,
		&delegationsBalanceNum,
		&debondingDelegationsBalanceNum,
	)
	if err == nil { //nolint:gocritic
		var err2 error
		// Convert numeric values to big.Int. pgx has a bug where it doesn't support reading into *big.Int.
		*a.DelegationsBalance, err2 = common.NumericToBigInt(delegationsBalanceNum)
		if err2 != nil {
			return nil, wrapError(err2)
		}
		*a.DebondingDelegationsBalance, err2 = common.NumericToBigInt(debondingDelegationsBalanceNum)
		if err2 != nil {
			return nil, wrapError(err2)
		}
	} else if err == pgx.ErrNoRows {
		// An address can have no entry in the `accounts` table, which means the indexer
		// hasn't seen any activity for this address before. However, the address itself is
		// still valid, with 0 balance. We rely on type-checking of the input `address` to
		// ensure that we do not return these responses for malformed oasis addresses.
		a.Address = address.String()
	} else {
		return nil, wrapError(err)
	}

	// Get allowances.
	allowanceRows, queryErr := c.db.Query(
		ctx,
		queries.AccountAllowances,
		address.String(),
	)
	if queryErr != nil {
		return nil, wrapError(err)
	}
	defer allowanceRows.Close()

	for allowanceRows.Next() {
		var al Allowance
		if err2 := allowanceRows.Scan(
			&al.Address,
			&al.Amount,
		); err2 != nil {
			return nil, wrapError(err2)
		}

		a.Allowances = append(a.Allowances, al)
	}

	return &a, nil
}

// Delegations returns a list of delegations.
func (c *StorageClient) Delegations(ctx context.Context, address staking.Address, p apiTypes.GetConsensusAccountsAddressDelegationsParams) (*DelegationList, error) {
	res, err := c.withTotalCount(
		ctx,
		queries.Delegations,
		address.String(),
		p.Limit,
		p.Offset,
	)
	if err != nil {
		return nil, wrapError(err)
	}
	defer res.rows.Close()

	ds := DelegationList{
		Delegations:         []Delegation{},
		TotalCount:          res.totalCount,
		IsTotalCountClipped: res.isTotalCountClipped,
	}
	for res.rows.Next() {
		var d Delegation
		var shares, escrowBalanceActive, escrowTotalSharesActive common.BigInt
		if err := res.rows.Scan(
			&d.ValidatorAddress,
			&shares,
			&escrowBalanceActive,
			&escrowTotalSharesActive,
		); err != nil {
			return nil, wrapError(err)
		}
		amount := new(big.Int).Mul(&shares.Int, &escrowBalanceActive.Int)
		amount.Quo(amount, &escrowTotalSharesActive.Int)
		d.Amount = BigInt{Int: *amount}
		d.Shares = shares

		ds.Delegations = append(ds.Delegations, d)
	}

	return &ds, nil
}

// DebondingDelegations returns a list of debonding delegations.
func (c *StorageClient) DebondingDelegations(ctx context.Context, address staking.Address, p apiTypes.GetConsensusAccountsAddressDebondingDelegationsParams) (*DebondingDelegationList, error) {
	res, err := c.withTotalCount(
		ctx,
		queries.DebondingDelegations,
		address.String(),
		p.Limit,
		p.Offset,
	)
	if err != nil {
		return nil, wrapError(err)
	}
	defer res.rows.Close()

	ds := DebondingDelegationList{
		DebondingDelegations: []DebondingDelegation{},
		TotalCount:           res.totalCount,
		IsTotalCountClipped:  res.isTotalCountClipped,
	}
	for res.rows.Next() {
		var d DebondingDelegation
		var shares, escrowBalanceDebonding, escrowTotalSharesDebonding common.BigInt
		if err := res.rows.Scan(
			&d.ValidatorAddress,
			&shares,
			&d.DebondEnd,
			&escrowBalanceDebonding,
			&escrowTotalSharesDebonding,
		); err != nil {
			return nil, wrapError(err)
		}

		amount := new(big.Int).Mul(&shares.Int, &escrowBalanceDebonding.Int)
		amount.Quo(amount, &escrowTotalSharesDebonding.Int)
		d.Amount = BigInt{Int: *amount}
		d.Shares = shares

		ds.DebondingDelegations = append(ds.DebondingDelegations, d)
	}

	return &ds, nil
}

// Epochs returns a list of consensus epochs.
func (c *StorageClient) Epochs(ctx context.Context, p apiTypes.GetConsensusEpochsParams) (*EpochList, error) {
	res, err := c.withTotalCount(
		ctx,
		queries.Epochs,
		p.Limit,
		p.Offset,
	)
	if err != nil {
		return nil, wrapError(err)
	}

	es := EpochList{
		Epochs:              []Epoch{},
		TotalCount:          res.totalCount,
		IsTotalCountClipped: res.isTotalCountClipped,
	}
	for res.rows.Next() {
		var e Epoch
		if err := res.rows.Scan(&e.ID, &e.StartHeight, &e.EndHeight); err != nil {
			return nil, wrapError(err)
		}

		es.Epochs = append(es.Epochs, e)
	}

	return &es, nil
}

// Epoch returns a consensus epoch.
func (c *StorageClient) Epoch(ctx context.Context, epoch int64) (*Epoch, error) {
	var e Epoch
	if err := c.db.QueryRow(
		ctx,
		queries.Epoch,
		epoch,
	).Scan(&e.ID, &e.StartHeight, &e.EndHeight); err != nil {
		return nil, wrapError(err)
	}

	return &e, nil
}

// Proposals returns a list of governance proposals.
func (c *StorageClient) Proposals(ctx context.Context, p apiTypes.GetConsensusProposalsParams) (*ProposalList, error) {
	res, err := c.withTotalCount(
		ctx,
		queries.Proposals,
		p.Submitter,
		p.State,
		p.Limit,
		p.Offset,
	)
	if err != nil {
		return nil, wrapError(err)
	}
	defer res.rows.Close()

	ps := ProposalList{
		Proposals:           []Proposal{},
		TotalCount:          res.totalCount,
		IsTotalCountClipped: res.isTotalCountClipped,
	}
	for res.rows.Next() {
		p := Proposal{Target: &ProposalTarget{}}
		var invalidVotesNum pgtype.Numeric
		if err := res.rows.Scan(
			&p.ID,
			&p.Submitter,
			&p.State,
			&p.Deposit,
			&p.Handler,
			&p.Target.ConsensusProtocol,
			&p.Target.RuntimeHostProtocol,
			&p.Target.RuntimeCommitteeProtocol,
			&p.Epoch,
			&p.Cancels,
			&p.CreatedAt,
			&p.ClosesAt,
			&invalidVotesNum,
		); err != nil {
			return nil, wrapError(err)
		}

		ps.Proposals = append(ps.Proposals, p)
	}

	return &ps, nil
}

// Proposal returns a governance proposal.
func (c *StorageClient) Proposal(ctx context.Context, proposalID uint64) (*Proposal, error) {
	p := Proposal{Target: &ProposalTarget{}}
	if err := c.db.QueryRow(
		ctx,
		queries.Proposal,
		proposalID,
	).Scan(
		&p.ID,
		&p.Submitter,
		&p.State,
		&p.Deposit,
		&p.Handler,
		&p.Target.ConsensusProtocol,
		&p.Target.RuntimeHostProtocol,
		&p.Target.RuntimeCommitteeProtocol,
		&p.Epoch,
		&p.Cancels,
		&p.CreatedAt,
		&p.ClosesAt,
		&p.InvalidVotes,
	); err != nil {
		return nil, wrapError(err)
	}

	return &p, nil
}

// ProposalVotes returns votes for a governance proposal.
func (c *StorageClient) ProposalVotes(ctx context.Context, proposalID uint64, p apiTypes.GetConsensusProposalsProposalIdVotesParams) (*ProposalVotes, error) {
	res, err := c.withTotalCount(
		ctx,
		queries.ProposalVotes,
		proposalID,
		p.Limit,
		p.Offset,
	)
	if err != nil {
		return nil, wrapError(err)
	}
	defer res.rows.Close()

	vs := ProposalVotes{
		Votes:               []ProposalVote{},
		TotalCount:          res.totalCount,
		IsTotalCountClipped: res.isTotalCountClipped,
	}
	for res.rows.Next() {
		var v ProposalVote
		if err := res.rows.Scan(
			&v.Address,
			&v.Vote,
		); err != nil {
			return nil, wrapError(err)
		}

		vs.Votes = append(vs.Votes, v)
	}
	vs.ProposalID = proposalID

	return &vs, nil
}

// Validators returns a list of validators.
func (c *StorageClient) Validators(ctx context.Context, p apiTypes.GetConsensusValidatorsParams) (*ValidatorList, error) {
	var epoch Epoch
	if err := c.db.QueryRow(
		ctx,
		queries.Validators,
	).Scan(&epoch.ID, &epoch.StartHeight); err != nil {
		return nil, wrapError(err)
	}

	res, err := c.withTotalCount(
		ctx,
		queries.ValidatorsData,
		p.Limit,
		p.Offset,
	)
	if err != nil {
		return nil, wrapError(err)
	}
	defer res.rows.Close()

	vs := ValidatorList{
		Validators:          []Validator{},
		TotalCount:          res.totalCount,
		IsTotalCountClipped: res.isTotalCountClipped,
	}
	for res.rows.Next() {
		var v Validator
		var schedule staking.CommissionSchedule
		if err := res.rows.Scan(
			&v.EntityID,
			&v.EntityAddress,
			&v.NodeID,
			&v.Escrow,
			&schedule,
			&v.Active,
			&v.Status,
			&v.Media,
		); err != nil {
			return nil, wrapError(err)
		}

		currentRate := schedule.CurrentRate(beacon.EpochTime(epoch.ID))
		if currentRate != nil {
			v.CurrentRate = currentRate.ToBigInt().Uint64()
		}
		bound, next := util.CurrentBound(schedule, beacon.EpochTime(epoch.ID))
		if bound != nil {
			v.CurrentCommissionBound = ValidatorCommissionBound{
				Lower:      bound.RateMin.ToBigInt().Uint64(),
				Upper:      bound.RateMax.ToBigInt().Uint64(),
				EpochStart: uint64(bound.Start),
			}
		}

		if next > 0 {
			v.CurrentCommissionBound.EpochEnd = next
		}

		vs.Validators = append(vs.Validators, v)
	}

	return &vs, nil
}

// Validator returns a single validator.
func (c *StorageClient) Validator(ctx context.Context, entityID signature.PublicKey) (*Validator, error) {
	var epoch Epoch
	if err := c.db.QueryRow(
		ctx,
		queries.Validator,
	).Scan(&epoch.ID, &epoch.StartHeight); err != nil {
		return nil, wrapError(err)
	}

	var v Validator
	var schedule staking.CommissionSchedule
	if err := c.db.QueryRow(
		ctx,
		queries.ValidatorData,
		entityID.String(),
	).Scan(
		&v.EntityID,
		&v.EntityAddress,
		&v.NodeID,
		&v.Escrow,
		&schedule,
		&v.Active,
		&v.Status,
		&v.Media,
	); err != nil {
		return nil, wrapError(err)
	}

	currentRate := schedule.CurrentRate(beacon.EpochTime(epoch.ID))
	if currentRate != nil {
		v.CurrentRate = currentRate.ToBigInt().Uint64()
	}
	bound, next := util.CurrentBound(schedule, beacon.EpochTime(epoch.ID))
	if bound != nil {
		v.CurrentCommissionBound = ValidatorCommissionBound{
			Lower:      bound.RateMin.ToBigInt().Uint64(),
			Upper:      bound.RateMax.ToBigInt().Uint64(),
			EpochStart: uint64(bound.Start),
		}
	}

	if next > 0 {
		v.CurrentCommissionBound.EpochEnd = next
	}

	return &v, nil
}

// RuntimeBlocks returns a list of runtime blocks.
func (c *StorageClient) RuntimeBlocks(ctx context.Context, p apiTypes.GetRuntimeBlocksParams) (*RuntimeBlockList, error) {
	res, err := c.withTotalCount(
		ctx,
		queries.RuntimeBlocks,
		runtimeFromCtx(ctx),
		p.From,
		p.To,
		p.After,
		p.Before,
		p.Limit,
		p.Offset,
	)
	if err != nil {
		return nil, wrapError(err)
	}
	defer res.rows.Close()

	bs := RuntimeBlockList{
		Blocks:              []RuntimeBlock{},
		TotalCount:          res.totalCount,
		IsTotalCountClipped: res.isTotalCountClipped,
	}
	for res.rows.Next() {
		var b RuntimeBlock
		if err := res.rows.Scan(&b.Round, &b.Hash, &b.Timestamp, &b.NumTransactions, &b.Size, &b.GasUsed); err != nil {
			return nil, wrapError(err)
		}
		b.Timestamp = b.Timestamp.UTC()

		bs.Blocks = append(bs.Blocks, b)
	}

	return &bs, nil
}

// RuntimeTransactions returns a list of runtime transactions.
func (c *StorageClient) RuntimeTransactions(ctx context.Context, p apiTypes.GetRuntimeTransactionsParams, txHash *string) (*RuntimeTransactionList, error) {
	res, err := c.withTotalCount(
		ctx,
		queries.RuntimeTransactions,
		runtimeFromCtx(ctx),
		p.Block,
		txHash, // tx_hash; used only by GetRuntimeTransactionsTxHash
		p.Rel,
		p.Limit,
		p.Offset,
	)
	if err != nil {
		return nil, wrapError(err)
	}
	defer res.rows.Close()

	ts := RuntimeTransactionList{
		Transactions:        []RuntimeTransaction{},
		TotalCount:          res.totalCount,
		IsTotalCountClipped: res.isTotalCountClipped,
	}
	for res.rows.Next() {
		var t RuntimeTransaction
		if err := res.rows.Scan(
			&t.Round,
			&t.Index,
			&t.Hash,
			&t.EthHash,
			&t.GasUsed,
			&t.Size,
			&t.Timestamp,
			&t.Raw,
			&t.ResultRaw,
			&t.AddressPreimage,
		); err != nil {
			return nil, wrapError(err)
		}

		ts.Transactions = append(ts.Transactions, t)
	}

	return &ts, nil
}

// RuntimeEvents returns a list of runtime events.
func (c *StorageClient) RuntimeEvents(ctx context.Context, p apiTypes.GetRuntimeEventsParams) (*RuntimeEventList, error) {
	res, err := c.withTotalCount(
		ctx,
		queries.RuntimeEvents,
		runtimeFromCtx(ctx),
		p.Block,
		p.TxIndex,
		p.TxHash,
		p.Type,
		p.EvmLogSignature,
		p.Rel,
		p.Limit,
		p.Offset,
	)
	if err != nil {
		return nil, wrapError(err)
	}
	defer res.rows.Close()

	es := RuntimeEventList{
		Events:              []RuntimeEvent{},
		TotalCount:          res.totalCount,
		IsTotalCountClipped: res.isTotalCountClipped,
	}
	for res.rows.Next() {
		var e RuntimeEvent
		if err := res.rows.Scan(
			&e.Round,
			&e.TxIndex,
			&e.TxHash,
			&e.Type,
			&e.Body,
			&e.EvmLogName,
			&e.EvmLogParams,
		); err != nil {
			return nil, wrapError(err)
		}
		es.Events = append(es.Events, e)
	}

	return &es, nil
}

func (c *StorageClient) RuntimeAccount(ctx context.Context, address staking.Address) (*RuntimeAccount, error) {
	a := RuntimeAccount{
		Address:         address.String(),
		AddressPreimage: &AddressPreimage{},
		Balances:        []RuntimeSdkBalance{},
		EvmBalances:     []RuntimeEvmBalance{},
	}
	var preimageContext string
	err := c.db.QueryRow(
		ctx,
		queries.AddressPreimage,
		address,
	).Scan(
		&preimageContext,
		&a.AddressPreimage.ContextVersion,
		&a.AddressPreimage.AddressData,
	)
	if err == nil { //nolint:gocritic
		a.AddressPreimage.Context = AddressDerivationContext(preimageContext)
	} else if err == pgx.ErrNoRows {
		// An address can have no entry in the address preimage table, which means the indexer
		// hasn't seen any activity for this address before. However, the address itself is
		// still valid, with 0 balance. We rely on type-checking of the input `address` to
		// ensure that we do not return these responses for malformed oasis addresses.
		a.Address = address.String()
		a.AddressPreimage = nil
	} else {
		return nil, wrapError(err)
	}

	// Get paratime balances.
	runtimeSdkRows, queryErr := c.db.Query(
		ctx,
		queries.AccountRuntimeSdkBalances,
		runtimeFromCtx(ctx),
		address.String(),
	)
	if queryErr != nil {
		return nil, wrapError(queryErr)
	}
	defer runtimeSdkRows.Close()

	for runtimeSdkRows.Next() {
		b := RuntimeSdkBalance{
			// HACK: 18 is accurate for Emerald and Sapphire, but Cipher has 9.
			// Once we add a non-18-decimals runtime, we'll need to query the runtime for this
			// at analysis time and store it in a table, similar to how we store the EVM token metadata.
			TokenDecimals: 18,
		}
		if err = runtimeSdkRows.Scan(
			&b.Balance,
			&b.TokenSymbol,
		); err != nil {
			return nil, wrapError(err)
		}
		a.Balances = append(a.Balances, b)
	}

	runtimeEvmRows, queryErr := c.db.Query(
		ctx,
		queries.AccountRuntimeEvmBalances,
		runtimeFromCtx(ctx),
		address.String(),
	)
	if queryErr != nil {
		return nil, wrapError(queryErr)
	}
	defer runtimeEvmRows.Close()

	for runtimeEvmRows.Next() {
		b := RuntimeEvmBalance{}
		if err = runtimeEvmRows.Scan(
			&b.Balance,
			&b.TokenContractAddr,
			&b.TokenSymbol,
			&b.TokenName,
			&b.TokenType,
			&b.TokenDecimals,
		); err != nil {
			return nil, wrapError(err)
		}
		a.EvmBalances = append(a.EvmBalances, b)
	}

	var totalSent pgtype.Numeric
	var totalReceived pgtype.Numeric
	if err = c.db.QueryRow(
		ctx,
		queries.RuntimeAccountStats,
		address.String(),
	).Scan(
		&totalSent,
		&totalReceived,
		&a.Stats.NumTxns,
	); err != nil {
		return nil, wrapError(err)
	}
	a.Stats.TotalSent, err = common.NumericToBigInt(totalSent)
	if err != nil {
		return nil, wrapError(err)
	}
	a.Stats.TotalReceived, err = common.NumericToBigInt(totalReceived)
	if err != nil {
		return nil, wrapError(err)
	}

	return &a, nil
}

func (c *StorageClient) RuntimeTokens(ctx context.Context, p apiTypes.GetRuntimeEvmTokensParams) (*EvmTokenList, error) {
	res, err := c.withTotalCount(
		ctx,
		queries.EvmTokens,
		runtimeFromCtx(ctx),
		p.Limit,
		p.Offset,
	)
	if err != nil {
		return nil, wrapError(err)
	}
	defer res.rows.Close()

	ts := EvmTokenList{
		EvmTokens:           []EvmToken{},
		TotalCount:          res.totalCount,
		IsTotalCountClipped: res.isTotalCountClipped,
	}
	for res.rows.Next() {
		var t EvmToken
		if err2 := res.rows.Scan(
			&t.ContractAddr,
			&t.EvmContractAddr,
			&t.Name,
			&t.Symbol,
			&t.Decimals,
			&t.TotalSupply,
			&t.Type,
			&t.NumHolders,
		); err2 != nil {
			return nil, wrapError(err)
		}

		ts.EvmTokens = append(ts.EvmTokens, t)
	}

	return &ts, nil
}

// TxVolumes returns a list of transaction volumes per time bucket.
func (c *StorageClient) TxVolumes(ctx context.Context, layer apiTypes.Layer, p apiTypes.GetLayerStatsTxVolumeParams) (*TxVolumeList, error) {
	var query string
	if *p.BucketSizeSeconds == 300 {
		query = queries.FineTxVolumes
	} else {
		var day uint32 = 86400
		p.BucketSizeSeconds = &day
		query = queries.TxVolumes
	}

	rows, err := c.db.Query(
		ctx,
		query,
		layer,
		p.Limit,
		p.Offset,
	)
	if err != nil {
		return nil, wrapError(err)
	}
	defer rows.Close()

	ts := TxVolumeList{
		BucketSizeSeconds: *p.BucketSizeSeconds,
		Buckets:           []apiTypes.TxVolume{},
	}
	for rows.Next() {
		var t TxVolume
		if err := rows.Scan(
			&t.BucketStart,
			&t.TxVolume,
		); err != nil {
			return nil, wrapError(err)
		}
		t.BucketStart = t.BucketStart.UTC() // Ensure UTC timestamp in response.
		ts.Buckets = append(ts.Buckets, t)
	}

	return &ts, nil
}

// DailyActiveAccounts returns a list of daily active accounts.
func (c *StorageClient) DailyActiveAccounts(ctx context.Context, layer apiTypes.Layer, p apiTypes.GetLayerStatsActiveAccountsParams) (*DailyActiveAccountsList, error) {
	var query string
	switch {
	case p.WindowStepSeconds != nil && *p.WindowStepSeconds == 300:
		query = queries.FineDailyActiveAccounts
	default:
		query = queries.DailyActiveAccounts
	}

	rows, err := c.db.Query(
		ctx,
		query,
		layer,
		p.Limit,
		p.Offset,
	)
	if err != nil {
		return nil, wrapError(err)
	}
	defer rows.Close()

	ts := DailyActiveAccountsList{
		WindowSizeSeconds: 86400, // Day.
		Windows:           []apiTypes.ActiveAccounts{},
	}
	for rows.Next() {
		var t apiTypes.ActiveAccounts
		if err := rows.Scan(
			&t.WindowEnd,
			&t.ActiveAccounts,
		); err != nil {
			return nil, wrapError(err)
		}
		t.WindowEnd = t.WindowEnd.UTC() // Ensure UTC timestamp in response.
		ts.Windows = append(ts.Windows, t)
	}

	return &ts, nil
}
