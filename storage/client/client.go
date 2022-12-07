package client

import (
	"context"
	"fmt"
	"math/big"
	"time"

	"github.com/dgraph-io/ristretto"
	"github.com/iancoleman/strcase"
	"github.com/jackc/pgtype"
	oasisErrors "github.com/oasisprotocol/oasis-core/go/common/errors"

	beacon "github.com/oasisprotocol/oasis-core/go/beacon/api"
	staking "github.com/oasisprotocol/oasis-core/go/staking/api"

	"github.com/oasisprotocol/oasis-indexer/analyzer/util"
	"github.com/oasisprotocol/oasis-indexer/api/common"
	"github.com/oasisprotocol/oasis-indexer/log"
	"github.com/oasisprotocol/oasis-indexer/storage"
)

const (
	blockCost = 1
	txCost    = 1
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

// type DbBigInt struct {
// 	inner *big.Int
// }

// func (dst *DbBigInt) DecodeBinary(_ci *pgtype.ConnInfo, src []byte) error {
// 	if src == nil {
// 		dst.inner = nil
// 		return nil
// 	}

// 	dst.inner = big.NewInt(0)
// 	dst.inner.SetBytes(src)
// 	return nil
// }

// NumericToBigInt converts a pgtype.Numeric to a big.int using the private method found at
// https://github.com/jackc/pgtype/blob/master/numeric.go#L398
func (c *StorageClient) NumericToBigInt(ctx context.Context, n *pgtype.Numeric) (*big.Int, error) {
	if n.Exp == 0 {
		return n.Int, nil
	}

	big0 := big.NewInt(0)
	big10 := big.NewInt(10)
	bi := &big.Int{}
	bi.Set(n.Int)
	if n.Exp > 0 {
		mul := &big.Int{}
		mul.Exp(big10, big.NewInt(int64(n.Exp)), nil)
		bi.Mul(bi, mul)
		return bi, nil
	}

	div := &big.Int{}
	div.Exp(big10, big.NewInt(int64(-n.Exp)), nil)
	remainder := &big.Int{}
	bi.DivMod(bi, div, remainder)
	if remainder.Cmp(big0) != 0 {
		err := fmt.Errorf("cannot convert %v to integer", n)
		c.logger.Info("failed to convert pgtype.Numeric to big.Int",
			"request_id", ctx.Value(RequestIDContextKey),
			"err", err,
		)
		return nil, err
	}
	return bi, nil
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

// Status returns status information for the Oasis Indexer.
func (c *StorageClient) Status(ctx context.Context) (*Status, error) {
	qf := NewQueryFactory(strcase.ToSnake(c.chainID), "" /* no runtime identifier for the consensus layer */)

	s := Status{
		LatestChainID: c.chainID,
	}
	if err := c.db.QueryRow(
		ctx,
		qf.StatusQuery(),
	).Scan(&s.LatestBlock, &s.LatestUpdate); err != nil {
		c.logger.Info("row scan failed",
			"request_id", ctx.Value(RequestIDContextKey),
			"err", err.Error(),
		)
		return nil, common.ErrStorageError
	}
	// oasis-node control status returns time truncated to the second
	// https://github.com/oasisprotocol/oasis-core/blob/5985dc5c2844de28241b7b16b19d91a86e5cbeda/docs/oasis-node/cli.md?plain=1#L41
	s.LatestUpdate = s.LatestUpdate.Truncate(time.Second)

	return &s, nil
}

// Blocks returns a list of consensus blocks.
func (c *StorageClient) Blocks(ctx context.Context, r *BlocksRequest, p *common.Pagination) (*BlockList, error) {
	cid, ok := ctx.Value(ChainIDContextKey).(string)
	if !ok {
		return nil, common.ErrBadChainID
	}
	qf := NewQueryFactory(cid, "" /* no runtime identifier for the consensus layer */)

	rows, err := c.db.Query(
		ctx,
		qf.BlocksQuery(),
		r.From,
		r.To,
		r.After,
		r.Before,
		p.Limit,
		p.Offset,
	)
	if err != nil {
		c.logger.Info("query failed",
			"request_id", ctx.Value(RequestIDContextKey),
			"err", err.Error(),
		)
		return nil, common.ErrStorageError
	}
	defer rows.Close()

	bs := BlockList{
		Blocks: []Block{},
	}
	for rows.Next() {
		var b Block
		if err := rows.Scan(&b.Height, &b.Hash, &b.Timestamp); err != nil {
			c.logger.Info("row scan failed",
				"request_id", ctx.Value(RequestIDContextKey),
				"err", err.Error(),
			)
			return nil, common.ErrStorageError
		}
		b.Timestamp = b.Timestamp.UTC()

		bs.Blocks = append(bs.Blocks, b)
	}

	return &bs, nil
}

// Block returns a consensus block. This endpoint is cached.
func (c *StorageClient) Block(ctx context.Context, r *BlockRequest) (*Block, error) {
	// Check cache
	untypedBlock, ok := c.blockCache.Get(*r.Height)
	if ok {
		return untypedBlock.(*Block), nil
	}

	cid, ok := ctx.Value(ChainIDContextKey).(string)
	if !ok {
		return nil, common.ErrBadChainID
	}
	qf := NewQueryFactory(cid, "" /* no runtime identifier for the consensus layer */)

	var b Block
	if err := c.db.QueryRow(
		ctx,
		qf.BlockQuery(),
		r.Height,
	).Scan(&b.Height, &b.Hash, &b.Timestamp); err != nil {
		c.logger.Info("row scan failed",
			"request_id", ctx.Value(RequestIDContextKey),
			"err", err.Error(),
		)
		return nil, common.ErrStorageError
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
func (c *StorageClient) Transactions(ctx context.Context, r *TransactionsRequest, p *common.Pagination) (*TransactionList, error) {
	cid, ok := ctx.Value(ChainIDContextKey).(string)
	if !ok {
		return nil, common.ErrBadChainID
	}
	qf := NewQueryFactory(cid, "" /* no runtime identifier for the consensus layer */)

	rows, err := c.db.Query(
		ctx,
		qf.TransactionsQuery(),
		r.Block,
		r.Method,
		r.Sender,
		r.MinFee,
		r.MaxFee,
		r.Code,
		p.Limit,
		p.Offset,
	)
	if err != nil {
		c.logger.Info("query failed",
			"request_id", ctx.Value(RequestIDContextKey),
			"err", err.Error(),
		)
		return nil, common.ErrStorageError
	}
	defer rows.Close()

	ts := TransactionList{
		Transactions: []Transaction{},
	}
	for rows.Next() {
		var t Transaction
		var code uint64
		var feeNum pgtype.Numeric
		if err := rows.Scan(
			&t.Height,
			&t.Hash,
			&t.Sender,
			&t.Nonce,
			&feeNum,
			&t.Method,
			&t.Body,
			&code,
		); err != nil {
			c.logger.Info("row scan failed",
				"request_id", ctx.Value(RequestIDContextKey),
				"err", err.Error(),
			)
			return nil, common.ErrStorageError
		}
		fee, err := c.NumericToBigInt(ctx, &feeNum)
		if err != nil {
			return nil, common.ErrStorageError
		}
		t.Fee = *fee
		if code == oasisErrors.CodeNoError {
			t.Success = true
		}

		ts.Transactions = append(ts.Transactions, t)
	}

	return &ts, nil
}

// Transaction returns a consensus transaction. This endpoint is cached.
func (c *StorageClient) Transaction(ctx context.Context, r *TransactionRequest) (*Transaction, error) {
	// Check cache
	untypedTx, ok := c.txCache.Get(*r.TxHash)
	if ok {
		return untypedTx.(*Transaction), nil
	}

	cid, ok := ctx.Value(ChainIDContextKey).(string)
	if !ok {
		return nil, common.ErrBadChainID
	}
	qf := NewQueryFactory(cid, "" /* no runtime identifier for the consensus layer */)

	var t Transaction
	var feeNum pgtype.Numeric
	var code uint64
	if err := c.db.QueryRow(
		ctx,
		qf.TransactionQuery(),
		r.TxHash,
	).Scan(
		&t.Height,
		&t.Hash,
		&t.Sender,
		&t.Nonce,
		&feeNum,
		&t.Method,
		&t.Body,
		&code,
	); err != nil {
		c.logger.Info("row scan failed",
			"request_id", ctx.Value(RequestIDContextKey),
			"err", err.Error(),
		)
		return nil, common.ErrStorageError
	}
	fee, err := c.NumericToBigInt(ctx, &feeNum)
	if err != nil {
		return nil, common.ErrStorageError
	}
	t.Fee = *fee
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

// Entities returns a list of registered entities.
func (c *StorageClient) Entities(ctx context.Context, p *common.Pagination) (*EntityList, error) {
	cid, ok := ctx.Value(ChainIDContextKey).(string)
	if !ok {
		return nil, common.ErrBadChainID
	}
	qf := NewQueryFactory(cid, "" /* no runtime identifier for the consensus layer */)

	rows, err := c.db.Query(
		ctx,
		qf.EntitiesQuery(),
		p.Limit,
		p.Offset,
	)
	if err != nil {
		c.logger.Info("query failed",
			"request_id", ctx.Value(RequestIDContextKey),
			"err", err.Error(),
		)
		return nil, common.ErrStorageError
	}
	defer rows.Close()

	es := EntityList{
		Entities: []Entity{},
	}
	for rows.Next() {
		var e Entity
		if err := rows.Scan(&e.ID, &e.Address); err != nil {
			c.logger.Info("query failed",
				"err", err.Error(),
			)
			return nil, common.ErrStorageError
		}

		es.Entities = append(es.Entities, e)
	}

	return &es, nil
}

// Entity returns a registered entity.
func (c *StorageClient) Entity(ctx context.Context, r *EntityRequest) (*Entity, error) {
	cid, ok := ctx.Value(ChainIDContextKey).(string)
	if !ok {
		return nil, common.ErrBadChainID
	}
	qf := NewQueryFactory(cid, "" /* no runtime identifier for the consensus layer */)

	var e Entity
	if err := c.db.QueryRow(
		ctx,
		qf.EntityQuery(),
		r.EntityID.String(),
	).Scan(&e.ID, &e.Address); err != nil {
		c.logger.Info("row scan failed",
			"request_id", ctx.Value(RequestIDContextKey),
			"err", err.Error(),
		)
		return nil, common.ErrStorageError
	}

	nodeRows, err := c.db.Query(
		ctx,
		qf.EntityNodeIdsQuery(),
		r.EntityID,
	)
	if err != nil {
		c.logger.Info("query failed",
			"request_id", ctx.Value(RequestIDContextKey),
			"err", err.Error(),
		)
		return nil, common.ErrStorageError
	}
	defer nodeRows.Close()

	for nodeRows.Next() {
		var nid string
		if err := nodeRows.Scan(&nid); err != nil {
			c.logger.Info("row scan failed",
				"request_id", ctx.Value(RequestIDContextKey),
				"err", err.Error(),
			)
			return nil, common.ErrStorageError
		}

		e.Nodes = append(e.Nodes, nid)
	}

	return &e, nil
}

// EntityNodes returns a list of nodes controlled by the provided entity.
func (c *StorageClient) EntityNodes(ctx context.Context, r *EntityNodesRequest, p *common.Pagination) (*NodeList, error) {
	cid, ok := ctx.Value(ChainIDContextKey).(string)
	if !ok {
		return nil, common.ErrBadChainID
	}
	qf := NewQueryFactory(cid, "" /* no runtime identifier for the consensus layer */)

	rows, err := c.db.Query(
		ctx,
		qf.EntityNodesQuery(),
		r.EntityID.String(),
		p.Limit,
		p.Offset,
	)
	if err != nil {
		c.logger.Info("query failed",
			"request_id", ctx.Value(RequestIDContextKey),
			"err", err.Error(),
		)
		return nil, common.ErrStorageError
	}
	defer rows.Close()

	ns := NodeList{
		Nodes: []Node{},
	}
	for rows.Next() {
		var n Node
		if err := rows.Scan(
			&n.ID,
			&n.EntityID,
			&n.Expiration,
			&n.TLSPubkey,
			&n.TLSNextPubkey,
			&n.P2PPubkey,
			&n.ConsensusPubkey,
			&n.Roles,
		); err != nil {
			c.logger.Info("row scan failed",
				"request_id", ctx.Value(RequestIDContextKey),
				"err", err.Error(),
			)
			return nil, common.ErrStorageError
		}

		ns.Nodes = append(ns.Nodes, n)
	}
	ns.EntityID = r.EntityID.String()

	return &ns, nil
}

// EntityNode returns a node controlled by the provided entity.
func (c *StorageClient) EntityNode(ctx context.Context, r *EntityNodeRequest) (*Node, error) {
	cid, ok := ctx.Value(ChainIDContextKey).(string)
	if !ok {
		return nil, common.ErrBadChainID
	}
	qf := NewQueryFactory(cid, "" /* no runtime identifier for the consensus layer */)

	var n Node
	if err := c.db.QueryRow(
		ctx,
		qf.EntityNodeQuery(),
		r.EntityID.String(),
		r.NodeID.String(),
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
		c.logger.Info("row scan failed",
			"request_id", ctx.Value(RequestIDContextKey),
			"err", err.Error(),
		)
		return nil, common.ErrStorageError
	}

	return &n, nil
}

// Accounts returns a list of consensus accounts.
func (c *StorageClient) Accounts(ctx context.Context, r *AccountsRequest, p *common.Pagination) (*AccountList, error) {
	cid, ok := ctx.Value(ChainIDContextKey).(string)
	if !ok {
		return nil, common.ErrBadChainID
	}
	qf := NewQueryFactory(cid, "" /* no runtime identifier for the consensus layer */)

	rows, err := c.db.Query(
		ctx,
		qf.AccountsQuery(),
		r.MinAvailable,
		r.MaxAvailable,
		r.MinEscrow,
		r.MaxEscrow,
		r.MinDebonding,
		r.MaxDebonding,
		r.MinTotalBalance,
		r.MaxTotalBalance,
		p.Limit,
		p.Offset,
	)
	if err != nil {
		c.logger.Info("query failed",
			"request_id", ctx.Value(RequestIDContextKey),
			"err", err.Error(),
		)
		return nil, common.ErrStorageError
	}
	defer rows.Close()

	as := AccountList{
		Accounts: []Account{},
	}
	for rows.Next() {
		var a Account
		var availableNum pgtype.Numeric
		var escrowNum pgtype.Numeric
		var debondingNum pgtype.Numeric
		if err := rows.Scan(
			&a.Address,
			&a.Nonce,
			&availableNum,
			&escrowNum,
			&debondingNum,
		); err != nil {
			c.logger.Info("row scan failed",
				"request_id", ctx.Value(RequestIDContextKey),
				"err", err.Error(),
			)
			return nil, common.ErrStorageError
		}
		available, err := c.NumericToBigInt(ctx, &availableNum)
		if err != nil {
			return nil, common.ErrStorageError
		}
		escrow, err := c.NumericToBigInt(ctx, &escrowNum)
		if err != nil {
			return nil, common.ErrStorageError
		}
		debonding, err := c.NumericToBigInt(ctx, &debondingNum)
		if err != nil {
			return nil, common.ErrStorageError
		}
		a.Available = *available
		a.Escrow = *escrow
		a.Debonding = *debonding

		as.Accounts = append(as.Accounts, a)
	}

	return &as, nil
}

// Account returns a consensus account.
func (c *StorageClient) Account(ctx context.Context, r *AccountRequest) (*Account, error) {
	cid, ok := ctx.Value(ChainIDContextKey).(string)
	if !ok {
		return nil, common.ErrBadChainID
	}
	qf := NewQueryFactory(cid, "" /* no runtime identifier for the consensus layer */)

	a := Account{
		Allowances: []Allowance{},
	}
	var availableNum pgtype.Numeric
	var escrowNum pgtype.Numeric
	var debondingNum pgtype.Numeric
	var delegationsBalanceNum pgtype.Numeric
	var debondingDelegationsBalanceNum pgtype.Numeric
	if err := c.db.QueryRow(
		ctx,
		qf.AccountQuery(),
		r.Address.String(),
	).Scan(
		&a.Address,
		&a.Nonce,
		&availableNum,
		&escrowNum,
		&debondingNum,
		&delegationsBalanceNum,
		&debondingDelegationsBalanceNum,
	); err != nil {
		c.logger.Info("row scan failed",
			"request_id", ctx.Value(RequestIDContextKey),
			"err", err.Error(),
		)
		return nil, common.ErrStorageError
	}
	available, err := c.NumericToBigInt(ctx, &availableNum)
	if err != nil {
		return nil, common.ErrStorageError
	}
	escrow, err := c.NumericToBigInt(ctx, &escrowNum)
	if err != nil {
		return nil, common.ErrStorageError
	}
	debonding, err := c.NumericToBigInt(ctx, &debondingNum)
	if err != nil {
		return nil, common.ErrStorageError
	}
	delegationsBalance, err := c.NumericToBigInt(ctx, &delegationsBalanceNum)
	if err != nil {
		return nil, common.ErrStorageError
	}
	debondingDelegationsBalance, err := c.NumericToBigInt(ctx, &debondingDelegationsBalanceNum)
	if err != nil {
		return nil, common.ErrStorageError
	}
	a.Available = *available
	a.Escrow = *escrow
	a.Debonding = *debonding
	a.DelegationsBalance = *delegationsBalance
	a.DebondingDelegationsBalance = *debondingDelegationsBalance

	allowanceRows, err := c.db.Query(
		ctx,
		qf.AccountAllowancesQuery(),
		r.Address,
	)
	if err != nil {
		c.logger.Info("query failed",
			"request_id", ctx.Value(RequestIDContextKey),
			"err", err.Error(),
		)
		return nil, common.ErrStorageError
	}
	defer allowanceRows.Close()

	for allowanceRows.Next() {
		var al Allowance
		var amountNum pgtype.Numeric
		if err := allowanceRows.Scan(
			&al.Address,
			&amountNum,
		); err != nil {
			c.logger.Info("row scan failed",
				"request_id", ctx.Value(RequestIDContextKey),
				"err", err.Error(),
			)
			return nil, common.ErrStorageError
		}
		amount, err := c.NumericToBigInt(ctx, &amountNum)
		if err != nil {
			return nil, common.ErrStorageError
		}
		al.Amount = *amount

		a.Allowances = append(a.Allowances, al)
	}

	return &a, nil
}

// Delegations returns a list of delegations.
func (c *StorageClient) Delegations(ctx context.Context, r *DelegationsRequest, p *common.Pagination) (*DelegationList, error) {
	cid, ok := ctx.Value(ChainIDContextKey).(string)
	if !ok {
		return nil, common.ErrBadChainID
	}
	qf := NewQueryFactory(cid, "" /* no runtime identifier for the consensus layer */)

	rows, err := c.db.Query(
		ctx,
		qf.DelegationsQuery(),
		r.Address.String(),
		p.Limit,
		p.Offset,
	)
	if err != nil {
		c.logger.Info("query failed",
			"request_id", ctx.Value(RequestIDContextKey),
			"err", err.Error(),
		)
		return nil, common.ErrStorageError
	}
	defer rows.Close()

	ds := DelegationList{
		Delegations: []Delegation{},
	}
	for rows.Next() {
		var d Delegation
		var sharesNum pgtype.Numeric
		var escrowBalanceActiveNum pgtype.Numeric
		var escrowTotalSharesActiveNum pgtype.Numeric
		if err := rows.Scan(
			&d.ValidatorAddress,
			&sharesNum,
			&escrowBalanceActiveNum,
			&escrowTotalSharesActiveNum,
		); err != nil {
			c.logger.Info("row scan failed",
				"request_id", ctx.Value(RequestIDContextKey),
				"err", err.Error(),
			)
			return nil, common.ErrStorageError
		}
		shares, err := c.NumericToBigInt(ctx, &sharesNum)
		if err != nil {
			return nil, common.ErrStorageError
		}
		escrowBalanceActive, err := c.NumericToBigInt(ctx, &escrowBalanceActiveNum)
		if err != nil {
			return nil, common.ErrStorageError
		}
		escrowTotalSharesActive, err := c.NumericToBigInt(ctx, &escrowBalanceActiveNum)
		if err != nil {
			return nil, common.ErrStorageError
		}
		amount := big.NewInt(0)
		amount.Mul(shares, escrowBalanceActive)
		amount.Quo(amount, escrowTotalSharesActive)
		d.Amount = *amount
		d.Shares = *shares

		ds.Delegations = append(ds.Delegations, d)
	}

	return &ds, nil
}

// DebondingDelegations returns a list of debonding delegations.
func (c *StorageClient) DebondingDelegations(ctx context.Context, r *DebondingDelegationsRequest, p *common.Pagination) (*DebondingDelegationList, error) {
	cid, ok := ctx.Value(ChainIDContextKey).(string)
	if !ok {
		return nil, common.ErrBadChainID
	}
	qf := NewQueryFactory(cid, "" /* no runtime identifier for the consensus layer */)

	rows, err := c.db.Query(
		ctx,
		qf.DebondingDelegationsQuery(),
		r.Address.String(),
		p.Limit,
		p.Offset,
	)
	if err != nil {
		c.logger.Info("query failed",
			"request_id", ctx.Value(RequestIDContextKey),
			"err", err.Error(),
		)
		return nil, common.ErrStorageError
	}
	defer rows.Close()

	ds := DebondingDelegationList{
		DebondingDelegations: []DebondingDelegation{},
	}
	for rows.Next() {
		var d DebondingDelegation
		var sharesNum pgtype.Numeric
		var escrowBalanceDebondingNum pgtype.Numeric
		var escrowTotalSharesDebondingNum pgtype.Numeric
		if err := rows.Scan(
			&d.ValidatorAddress,
			&sharesNum,
			&d.DebondEnd,
			&escrowBalanceDebondingNum,
			&escrowTotalSharesDebondingNum,
		); err != nil {
			c.logger.Info("row scan failed",
				"request_id", ctx.Value(RequestIDContextKey),
				"err", err.Error(),
			)
			return nil, common.ErrStorageError
		}

		shares, err := c.NumericToBigInt(ctx, &sharesNum)
		if err != nil {
			return nil, common.ErrStorageError
		}
		escrowBalanceDebonding, err := c.NumericToBigInt(ctx, &escrowBalanceDebondingNum)
		if err != nil {
			return nil, common.ErrStorageError
		}
		escrowTotalSharesDebonding, err := c.NumericToBigInt(ctx, &escrowBalanceDebondingNum)
		if err != nil {
			return nil, common.ErrStorageError
		}
		amount := big.NewInt(0)
		amount.Mul(shares, escrowBalanceDebonding)
		amount.Quo(amount, escrowTotalSharesDebonding)
		d.Amount = *amount
		d.Shares = *shares

		ds.DebondingDelegations = append(ds.DebondingDelegations, d)
	}

	return &ds, nil
}

// Epochs returns a list of consensus epochs.
func (c *StorageClient) Epochs(ctx context.Context, p *common.Pagination) (*EpochList, error) {
	cid, ok := ctx.Value(ChainIDContextKey).(string)
	if !ok {
		return nil, common.ErrBadChainID
	}
	qf := NewQueryFactory(cid, "" /* no runtime identifier for the consensus layer */)

	rows, err := c.db.Query(
		ctx,
		qf.EpochsQuery(),
		p.Limit,
		p.Offset,
	)
	if err != nil {
		c.logger.Info("query failed",
			"request_id", ctx.Value(RequestIDContextKey),
			"err", err.Error(),
		)
		return nil, common.ErrStorageError
	}

	es := EpochList{
		Epochs: []Epoch{},
	}
	for rows.Next() {
		var e Epoch
		var endHeight *uint64
		if err := rows.Scan(&e.ID, &e.StartHeight, &endHeight); err != nil {
			c.logger.Info("row scan failed",
				"request_id", ctx.Value(RequestIDContextKey),
				"err", err.Error(),
			)
			return nil, common.ErrStorageError
		}
		if endHeight != nil {
			e.EndHeight = *endHeight
		}

		es.Epochs = append(es.Epochs, e)
	}

	return &es, nil
}

// Epoch returns a consensus epoch.
func (c *StorageClient) Epoch(ctx context.Context, r *EpochRequest) (*Epoch, error) {
	cid, ok := ctx.Value(ChainIDContextKey).(string)
	if !ok {
		return nil, common.ErrBadChainID
	}
	qf := NewQueryFactory(cid, "" /* no runtime identifier for the consensus layer */)

	var e Epoch
	if err := c.db.QueryRow(
		ctx,
		qf.EpochQuery(),
		r.Epoch,
	).Scan(&e.ID, &e.StartHeight, &e.EndHeight); err != nil {
		c.logger.Info("row scan failed",
			"request_id", ctx.Value(RequestIDContextKey),
			"err", err.Error(),
		)
		return nil, common.ErrStorageError
	}

	return &e, nil
}

// Proposals returns a list of governance proposals.
func (c *StorageClient) Proposals(ctx context.Context, r *ProposalsRequest, p *common.Pagination) (*ProposalList, error) {
	cid, ok := ctx.Value(ChainIDContextKey).(string)
	if !ok {
		return nil, common.ErrBadChainID
	}
	qf := NewQueryFactory(cid, "" /* no runtime identifier for the consensus layer */)

	rows, err := c.db.Query(
		ctx,
		qf.ProposalsQuery(),
		r.Submitter,
		r.State,
		p.Limit,
		p.Offset,
	)
	if err != nil {
		c.logger.Info("query failed",
			"request_id", ctx.Value(RequestIDContextKey),
			"err", err.Error(),
		)
		return nil, common.ErrStorageError
	}
	defer rows.Close()

	ps := ProposalList{
		Proposals: []Proposal{},
	}
	for rows.Next() {
		var p Proposal
		var depositNum pgtype.Numeric
		var invalidVotesNum pgtype.Numeric
		if err := rows.Scan(
			&p.ID,
			&p.Submitter,
			&p.State,
			&depositNum,
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
			c.logger.Info("row scan failed",
				"request_id", ctx.Value(RequestIDContextKey),
				"err", err.Error(),
			)
			return nil, common.ErrStorageError
		}
		deposit, err := c.NumericToBigInt(ctx, &depositNum)
		if err != nil {
			return nil, common.ErrStorageError
		}
		invalidVotes, err := c.NumericToBigInt(ctx, &invalidVotesNum)
		if err != nil {
			return nil, common.ErrStorageError
		}
		p.Deposit = *deposit
		p.InvalidVotes = *invalidVotes

		ps.Proposals = append(ps.Proposals, p)
	}

	return &ps, nil
}

// Proposal returns a governance proposal.
func (c *StorageClient) Proposal(ctx context.Context, r *ProposalRequest) (*Proposal, error) {
	cid, ok := ctx.Value(ChainIDContextKey).(string)
	if !ok {
		return nil, common.ErrBadChainID
	}
	qf := NewQueryFactory(cid, "" /* no runtime identifier for the consensus layer */)

	var p Proposal
	var depositNum pgtype.Numeric
	var invalidVotesNum pgtype.Numeric
	if err := c.db.QueryRow(
		ctx,
		qf.ProposalQuery(),
		r.ProposalID,
	).Scan(
		&p.ID,
		&p.Submitter,
		&p.State,
		&depositNum,
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
		c.logger.Info("row scan failed",
			"request_id", ctx.Value(RequestIDContextKey),
			"err", err.Error(),
		)
		return nil, common.ErrStorageError
	}
	deposit, err := c.NumericToBigInt(ctx, &depositNum)
	if err != nil {
		return nil, common.ErrStorageError
	}
	invalidVotes, err := c.NumericToBigInt(ctx, &invalidVotesNum)
	if err != nil {
		return nil, common.ErrStorageError
	}
	p.Deposit = *deposit
	p.InvalidVotes = *invalidVotes

	return &p, nil
}

// ProposalVotes returns votes for a governance proposal.
func (c *StorageClient) ProposalVotes(ctx context.Context, r *ProposalVotesRequest, p *common.Pagination) (*ProposalVotes, error) {
	cid, ok := ctx.Value(ChainIDContextKey).(string)
	if !ok {
		return nil, common.ErrBadChainID
	}
	qf := NewQueryFactory(cid, "" /* no runtime identifier for the consensus layer */)

	rows, err := c.db.Query(
		ctx,
		qf.ProposalVotesQuery(),
		r.ProposalID,
		p.Limit,
		p.Offset,
	)
	if err != nil {
		c.logger.Info("query failed",
			"request_id", ctx.Value(RequestIDContextKey),
			"err", err.Error(),
		)
		return nil, common.ErrStorageError
	}
	defer rows.Close()

	vs := ProposalVotes{
		Votes: []ProposalVote{},
	}
	for rows.Next() {
		var v ProposalVote
		if err := rows.Scan(
			&v.Address,
			&v.Vote,
		); err != nil {
			c.logger.Info("row scan failed",
				"request_id", ctx.Value(RequestIDContextKey),
				"err", err.Error(),
			)
			return nil, common.ErrStorageError
		}

		vs.Votes = append(vs.Votes, v)
	}
	vs.ProposalID = *r.ProposalID

	return &vs, nil
}

// Validators returns a list of validators.
func (c *StorageClient) Validators(ctx context.Context, p *common.Pagination) (*ValidatorList, error) {
	cid, ok := ctx.Value(ChainIDContextKey).(string)
	if !ok {
		return nil, common.ErrBadChainID
	}
	qf := NewQueryFactory(cid, "" /* no runtime identifier for the consensus layer */)

	var epoch Epoch
	if err := c.db.QueryRow(
		ctx,
		qf.ValidatorsQuery(),
	).Scan(&epoch.ID, &epoch.StartHeight); err != nil {
		c.logger.Info("row scan failed",
			"request_id", ctx.Value(RequestIDContextKey),
			"err", err.Error(),
		)
		return nil, common.ErrStorageError
	}

	rows, err := c.db.Query(
		ctx,
		qf.ValidatorsDataQuery(),
		p.Limit,
		p.Offset,
	)
	if err != nil {
		c.logger.Info("query failed",
			"request_id", ctx.Value(RequestIDContextKey),
			"err", err.Error(),
		)
		return nil, common.ErrStorageError
	}
	defer rows.Close()

	vs := ValidatorList{
		Validators: []Validator{},
	}
	for rows.Next() {
		var v Validator
		var escrowNum pgtype.Numeric
		var schedule staking.CommissionSchedule
		if err := rows.Scan(
			&v.EntityID,
			&v.EntityAddress,
			&v.NodeID,
			&escrowNum,
			&schedule,
			&v.Active,
			&v.Status,
			&v.Media,
		); err != nil {
			c.logger.Info("query failed",
				"err", err.Error(),
			)
			return nil, common.ErrStorageError
		}
		escrow, err := c.NumericToBigInt(ctx, &escrowNum)
		if err != nil {
			return nil, common.ErrStorageError
		}
		v.Escrow = *escrow
		// Match API for now
		v.Name = v.Media.Name

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
func (c *StorageClient) Validator(ctx context.Context, r *ValidatorRequest) (*Validator, error) {
	cid, ok := ctx.Value(ChainIDContextKey).(string)
	if !ok {
		return nil, common.ErrBadChainID
	}
	qf := NewQueryFactory(cid, "" /* no runtime identifier for the consensus layer */)

	var epoch Epoch
	if err := c.db.QueryRow(
		ctx,
		qf.ValidatorQuery(),
	).Scan(&epoch.ID, &epoch.StartHeight); err != nil {
		c.logger.Info("row scan failed",
			"request_id", ctx.Value(RequestIDContextKey),
			"err", err.Error(),
		)
		return nil, common.ErrStorageError
	}

	row := c.db.QueryRow(
		ctx,
		qf.ValidatorDataQuery(),
		r.EntityID.String(),
	)

	var v Validator
	var escrowNum pgtype.Numeric
	var schedule staking.CommissionSchedule
	if err := row.Scan(
		&v.EntityID,
		&v.EntityAddress,
		&v.NodeID,
		&escrowNum,
		&schedule,
		&v.Active,
		&v.Status,
		&v.Media,
	); err != nil {
		c.logger.Info("query failed",
			"err", err.Error(),
		)
		return nil, common.ErrStorageError
	}
	escrow, err := c.NumericToBigInt(ctx, &escrowNum)
	if err != nil {
		return nil, common.ErrStorageError
	}
	v.Escrow = *escrow
	// Match API for now
	v.Name = v.Media.Name

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
func (c *StorageClient) RuntimeBlocks(ctx context.Context, r *RuntimeBlocksRequest, p *common.Pagination) (*RuntimeBlockList, error) {
	cid, ok := ctx.Value(ChainIDContextKey).(string)
	if !ok {
		return nil, common.ErrBadChainID
	}
	runtime, ok := ctx.Value(RuntimeContextKey).(string)
	if !ok {
		return nil, common.ErrBadRuntime
	}
	qf := NewQueryFactory(cid, runtime)

	rows, err := c.db.Query(
		ctx,
		qf.RuntimeBlocksQuery(),
		r.From,
		r.To,
		r.After,
		r.Before,
		p.Limit,
		p.Offset,
	)
	if err != nil {
		c.logger.Info("query failed",
			"request_id", ctx.Value(RequestIDContextKey),
			"err", err.Error(),
		)
		return nil, common.ErrStorageError
	}
	defer rows.Close()

	bs := RuntimeBlockList{
		Blocks: []RuntimeBlock{},
	}
	for rows.Next() {
		var b RuntimeBlock
		if err := rows.Scan(&b.Round, &b.Hash, &b.Timestamp, &b.NumTransactions, &b.Size, &b.GasUsed); err != nil {
			c.logger.Info("row scan failed",
				"request_id", ctx.Value(RequestIDContextKey),
				"err", err.Error(),
			)
			return nil, common.ErrStorageError
		}
		b.Timestamp = b.Timestamp.UTC()

		bs.Blocks = append(bs.Blocks, b)
	}

	return &bs, nil
}

// RuntimeTransactions returns a list of runtime transactions.
func (c *StorageClient) RuntimeTransactions(ctx context.Context, r *RuntimeTransactionsRequest, p *common.Pagination) (*RuntimeTransactionList, error) {
	cid, ok := ctx.Value(ChainIDContextKey).(string)
	if !ok {
		return nil, common.ErrBadChainID
	}
	runtime, ok := ctx.Value(RuntimeContextKey).(string)
	if !ok {
		return nil, common.ErrBadRuntime
	}
	qf := NewQueryFactory(cid, runtime)

	rows, err := c.db.Query(
		ctx,
		qf.RuntimeTransactionsQuery(),
		r.Block,
		p.Limit,
		p.Offset,
	)
	if err != nil {
		c.logger.Info("query failed",
			"request_id", ctx.Value(RequestIDContextKey),
			"err", err.Error(),
		)
		return nil, common.ErrStorageError
	}
	defer rows.Close()

	ts := RuntimeTransactionList{
		Transactions: []RuntimeTransaction{},
	}
	for rows.Next() {
		var t RuntimeTransaction
		if err := rows.Scan(
			&t.Round,
			&t.Index,
			&t.Hash,
			&t.EthHash,
			&t.Raw,
			&t.ResultRaw,
		); err != nil {
			c.logger.Info("row scan failed",
				"request_id", ctx.Value(RequestIDContextKey),
				"err", err.Error(),
			)
			return nil, common.ErrStorageError
		}

		ts.Transactions = append(ts.Transactions, t)
	}

	return &ts, nil
}

func (c *StorageClient) RuntimeTokens(ctx context.Context, r *RuntimeTokensRequest, p *common.Pagination) (*RuntimeTokenList, error) {
	cid, ok := ctx.Value(ChainIDContextKey).(string)
	if !ok {
		return nil, common.ErrBadChainID
	}
	runtime, ok := ctx.Value(RuntimeContextKey).(string)
	if !ok {
		return nil, common.ErrBadRuntime
	}
	qf := NewQueryFactory(cid, runtime)

	rows, err := c.db.Query(
		ctx,
		qf.RuntimeTokensQuery(),
		p.Limit,
		p.Offset,
	)
	if err != nil {
		c.logger.Info("query failed",
			"request_id", ctx.Value(RequestIDContextKey),
			"err", err.Error(),
		)
		return nil, common.ErrStorageError
	}
	defer rows.Close()

	ts := RuntimeTokenList{
		Tokens: []RuntimeToken{},
	}
	for rows.Next() {
		var t RuntimeToken
		if err := rows.Scan(
			&t.TokenAddr,
			&t.NumHolders,
		); err != nil {
			c.logger.Info("row scan failed",
				"request_id", ctx.Value(RequestIDContextKey),
				"err", err.Error(),
			)
			return nil, common.ErrStorageError
		}

		ts.Tokens = append(ts.Tokens, t)
	}

	return &ts, nil
}

// TxVolumes returns a list of transaction volumes per time bucket.
func (c *StorageClient) TxVolumes(ctx context.Context, p *common.Pagination, q *common.BucketedStatsParams) (*TxVolumeList, error) {
	qf := NewQueryFactory(strcase.ToSnake(c.chainID), "")
	var query string
	if q.BucketSizeSeconds == 300 {
		query = qf.FineTxVolumesQuery()
	} else {
		query = qf.TxVolumesQuery()
	}

	rows, err := c.db.Query(
		ctx,
		query,
		p.Limit,
		p.Offset,
	)
	if err != nil {
		c.logger.Info("query failed",
			"request_id", ctx.Value(RequestIDContextKey),
			"err", err.Error(),
		)
		return nil, common.ErrStorageError
	}
	defer rows.Close()

	ts := TxVolumeList{
		BucketSizeSeconds: q.BucketSizeSeconds,
		Buckets:           []TxVolume{},
	}
	for rows.Next() {
		var d struct {
			BucketStart time.Time
			TxVolume    uint64
		}
		if err := rows.Scan(
			&d.BucketStart,
			&d.TxVolume,
		); err != nil {
			c.logger.Info("query failed",
				"err", err.Error(),
			)
			return nil, common.ErrStorageError
		}

		t := TxVolume{
			BucketStart: d.BucketStart.UTC(),
			Volume:      d.TxVolume,
		}
		ts.Buckets = append(ts.Buckets, t)
	}

	return &ts, nil
}
