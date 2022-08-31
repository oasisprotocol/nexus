package v1

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/iancoleman/strcase"
	oasisErrors "github.com/oasisprotocol/oasis-core/go/common/errors"

	beacon "github.com/oasisprotocol/oasis-core/go/beacon/api"
	staking "github.com/oasisprotocol/oasis-core/go/staking/api"

	"github.com/oasisprotocol/oasis-indexer/analyzer/util"
	"github.com/oasisprotocol/oasis-indexer/api/common"
	"github.com/oasisprotocol/oasis-indexer/log"
	"github.com/oasisprotocol/oasis-indexer/storage"
)

const (
	tpsWindowSizeMinutes = 5
)

// storageClient is a wrapper around a storage.TargetStorage
// with knowledge of network semantics.
type storageClient struct {
	db     storage.TargetStorage
	logger *log.Logger
}

// newStorageClient creates a new storage client.
func newStorageClient(db storage.TargetStorage, l *log.Logger) *storageClient {
	return &storageClient{db, l}
}

// Status returns status information for the Oasis Indexer.
func (c *storageClient) Status(ctx context.Context) (*Status, error) {
	qf := NewQueryFactory(strcase.ToSnake(LatestChainID))

	s := Status{
		LatestChainID: LatestChainID,
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
	return &s, nil
}

// Blocks returns a list of consensus blocks.
func (c *storageClient) Blocks(ctx context.Context, r *http.Request) (*BlockList, error) {
	cid, ok := ctx.Value(ChainIDContextKey).(string)
	if !ok {
		return nil, common.ErrBadChainID
	}
	qf := NewQueryFactory(cid)

	params := r.URL.Query()

	var from *string
	if v := params.Get("from"); v != "" {
		from = &v
	}
	var to *string
	if v := params.Get("to"); v != "" {
		to = &v
	}
	var after *string
	if v := params.Get("after"); v != "" {
		after = &v
	}
	var before *string
	if v := params.Get("before"); v != "" {
		before = &v
	}

	pagination, err := common.NewPagination(r)
	if err != nil {
		c.logger.Info("pagination failed",
			"request_id", ctx.Value(RequestIDContextKey),
			"err", err.Error(),
		)
		return nil, common.ErrBadRequest
	}

	rows, err := c.db.Query(
		ctx,
		qf.BlocksQuery(),
		from,
		to,
		after,
		before,
		pagination.Order,
		pagination.Limit,
		pagination.Offset,
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

// Block returns a consensus block.
func (c *storageClient) Block(ctx context.Context, r *http.Request) (*Block, error) {
	cid, ok := ctx.Value(ChainIDContextKey).(string)
	if !ok {
		return nil, common.ErrBadChainID
	}
	qf := NewQueryFactory(cid)

	var b Block
	if err := c.db.QueryRow(
		ctx,
		qf.BlockQuery(),
		chi.URLParam(r, "height"),
	).Scan(&b.Height, &b.Hash, &b.Timestamp); err != nil {
		c.logger.Info("row scan failed",
			"request_id", ctx.Value(RequestIDContextKey),
			"err", err.Error(),
		)
		return nil, common.ErrStorageError
	}
	b.Timestamp = b.Timestamp.UTC()

	return &b, nil
}

// Transactions returns a list of consensus transactions.
func (c *storageClient) Transactions(ctx context.Context, r *http.Request) (*TransactionList, error) {
	cid, ok := ctx.Value(ChainIDContextKey).(string)
	if !ok {
		return nil, common.ErrBadChainID
	}
	qf := NewQueryFactory(cid)

	params := r.URL.Query()

	var block *string
	if v := params.Get("block"); v != "" {
		block = &v
	}
	var method *string
	if v := params.Get("method"); v != "" {
		method = &v
	}
	var sender *string
	if v := params.Get("sender"); v != "" {
		sender = &v
	}
	var minFee *string
	if v := params.Get("minFee"); v != "" {
		minFee = &v
	}
	var maxFee *string
	if v := params.Get("maxFee"); v != "" {
		maxFee = &v
	}
	var code *string
	if v := params.Get("code"); v != "" {
		code = &v
	}

	pagination, err := common.NewPagination(r)
	if err != nil {
		c.logger.Info("pagination failed",
			"request_id", ctx.Value(RequestIDContextKey),
			"err", err.Error(),
		)
		return nil, common.ErrBadRequest
	}

	rows, err := c.db.Query(
		ctx,
		qf.TransactionsQuery(),
		block,
		method,
		sender,
		minFee,
		maxFee,
		code,
		pagination.Order,
		pagination.Limit,
		pagination.Offset,
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
		if err := rows.Scan(
			&t.Height,
			&t.Hash,
			&t.Sender,
			&t.Nonce,
			&t.Fee,
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
		if code == oasisErrors.CodeNoError {
			t.Success = true
		}

		ts.Transactions = append(ts.Transactions, t)
	}

	return &ts, nil
}

// Transaction returns a consensus transaction.
func (c *storageClient) Transaction(ctx context.Context, r *http.Request) (*Transaction, error) {
	cid, ok := ctx.Value(ChainIDContextKey).(string)
	if !ok {
		return nil, common.ErrBadChainID
	}
	qf := NewQueryFactory(cid)

	var t Transaction
	var code uint64
	if err := c.db.QueryRow(
		ctx,
		qf.TransactionQuery(),
		chi.URLParam(r, "txn_hash"),
	).Scan(
		&t.Height,
		&t.Hash,
		&t.Sender,
		&t.Nonce,
		&t.Fee,
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
	if code == oasisErrors.CodeNoError {
		t.Success = true
	}

	return &t, nil
}

// Entities returns a list of registered entities.
func (c *storageClient) Entities(ctx context.Context, r *http.Request) (*EntityList, error) {
	cid, ok := ctx.Value(ChainIDContextKey).(string)
	if !ok {
		return nil, common.ErrBadChainID
	}
	qf := NewQueryFactory(cid)

	pagination, err := common.NewPagination(r)
	if err != nil {
		c.logger.Info("pagination failed",
			"request_id", ctx.Value(RequestIDContextKey),
			"err", err.Error(),
		)
		return nil, common.ErrBadRequest
	}

	rows, err := c.db.Query(
		ctx,
		qf.EntitiesQuery(),
		pagination.Order,
		pagination.Limit,
		pagination.Offset,
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
func (c *storageClient) Entity(ctx context.Context, r *http.Request) (*Entity, error) {
	cid, ok := ctx.Value(ChainIDContextKey).(string)
	if !ok {
		return nil, common.ErrBadChainID
	}
	qf := NewQueryFactory(cid)

	entityID, err := url.PathUnescape(chi.URLParam(r, "entity_id"))
	if err != nil {
		return nil, common.ErrBadRequest
	}

	var e Entity
	if err = c.db.QueryRow(
		ctx,
		qf.EntityQuery(),
		entityID,
	).Scan(&e.ID, &e.Address); err != nil {
		c.logger.Info("row scan failed",
			"request_id", ctx.Value(RequestIDContextKey),
			"err", err.Error(),
		)
		return nil, common.ErrStorageError
	}

	entityID, err = url.PathUnescape(chi.URLParam(r, "entity_id"))
	if err != nil {
		return nil, common.ErrBadRequest
	}

	nodeRows, err := c.db.Query(
		ctx,
		qf.EntityNodeIdsQuery(),
		entityID,
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
func (c *storageClient) EntityNodes(ctx context.Context, r *http.Request) (*NodeList, error) {
	cid, ok := ctx.Value(ChainIDContextKey).(string)
	if !ok {
		return nil, common.ErrBadChainID
	}
	qf := NewQueryFactory(cid)

	pagination, err := common.NewPagination(r)
	if err != nil {
		c.logger.Info("pagination failed",
			"request_id", ctx.Value(RequestIDContextKey),
			"err", err.Error(),
		)
		return nil, common.ErrBadRequest
	}

	id, err := url.PathUnescape(chi.URLParam(r, "entity_id"))
	if err != nil {
		return nil, common.ErrBadRequest
	}
	rows, err := c.db.Query(
		ctx,
		qf.EntityNodesQuery(),
		id,
		pagination.Order,
		pagination.Limit,
		pagination.Offset,
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
	ns.EntityID = id

	return &ns, nil
}

// EntityNode returns a node controlled by the provided entity.
func (c *storageClient) EntityNode(ctx context.Context, r *http.Request) (*Node, error) {
	cid, ok := ctx.Value(ChainIDContextKey).(string)
	if !ok {
		return nil, common.ErrBadChainID
	}
	qf := NewQueryFactory(cid)

	entityID, err := url.PathUnescape(chi.URLParam(r, "entity_id"))
	if err != nil {
		return nil, common.ErrBadRequest
	}
	nodeID, err := url.PathUnescape(chi.URLParam(r, "node_id"))
	if err != nil {
		return nil, common.ErrBadRequest
	}
	var n Node
	if err := c.db.QueryRow(
		ctx,
		qf.EntityNodeQuery(),
		entityID,
		nodeID,
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
func (c *storageClient) Accounts(ctx context.Context, r *http.Request) (*AccountList, error) {
	cid, ok := ctx.Value(ChainIDContextKey).(string)
	if !ok {
		return nil, common.ErrBadChainID
	}
	qf := NewQueryFactory(cid)

	params := r.URL.Query()

	var minAvailable *string
	if v := params.Get("minAvailable"); v != "" {
		minAvailable = &v
	}
	var maxAvailable *string
	if v := params.Get("maxAvailable"); v != "" {
		maxAvailable = &v
	}
	var minEscrow *string
	if v := params.Get("minEscrow"); v != "" {
		minEscrow = &v
	}
	var maxEscrow *string
	if v := params.Get("maxEscrow"); v != "" {
		maxEscrow = &v
	}
	var minDebonding *string
	if v := params.Get("minDebonding"); v != "" {
		minDebonding = &v
	}
	var maxDebonding *string
	if v := params.Get("maxDebonding"); v != "" {
		maxDebonding = &v
	}
	var minTotalBalance *string
	if v := params.Get("minTotalBalance"); v != "" {
		minTotalBalance = &v
	}
	var maxTotalBalance *string
	if v := params.Get("maxTotalBalance"); v != "" {
		maxTotalBalance = &v
	}

	pagination, err := common.NewPagination(r)
	if err != nil {
		c.logger.Info("pagination failed",
			"request_id", ctx.Value(RequestIDContextKey),
			"err", err.Error(),
		)
		return nil, common.ErrBadRequest
	}

	rows, err := c.db.Query(
		ctx,
		qf.AccountsQuery(),
		minAvailable,
		maxAvailable,
		minEscrow,
		maxEscrow,
		minDebonding,
		maxDebonding,
		minTotalBalance,
		maxTotalBalance,
		pagination.Order,
		pagination.Limit,
		pagination.Offset,
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
		if err := rows.Scan(
			&a.Address,
			&a.Nonce,
			&a.Available,
			&a.Escrow,
			&a.Debonding,
		); err != nil {
			c.logger.Info("row scan failed",
				"request_id", ctx.Value(RequestIDContextKey),
				"err", err.Error(),
			)
			return nil, common.ErrStorageError
		}

		as.Accounts = append(as.Accounts, a)
	}

	return &as, nil
}

// Account returns a consensus account.
func (c *storageClient) Account(ctx context.Context, r *http.Request) (*Account, error) {
	cid, ok := ctx.Value(ChainIDContextKey).(string)
	if !ok {
		return nil, common.ErrBadChainID
	}
	qf := NewQueryFactory(cid)

	a := Account{
		Allowances: []Allowance{},
	}
	if err := c.db.QueryRow(
		ctx,
		qf.AccountQuery(),
		chi.URLParam(r, "address"),
	).Scan(
		&a.Address,
		&a.Nonce,
		&a.Available,
		&a.Escrow,
		&a.Debonding,
		&a.DelegationsBalance,
		&a.DebondingDelegationsBalance,
	); err != nil {
		c.logger.Info("row scan failed",
			"request_id", ctx.Value(RequestIDContextKey),
			"err", err.Error(),
		)
		return nil, common.ErrStorageError
	}

	allowanceRows, err := c.db.Query(
		ctx,
		qf.AccountAllowancesQuery(),
		chi.URLParam(r, "address"),
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
		if err := allowanceRows.Scan(
			&al.Address,
			&al.Amount,
		); err != nil {
			c.logger.Info("row scan failed",
				"request_id", ctx.Value(RequestIDContextKey),
				"err", err.Error(),
			)
			return nil, common.ErrStorageError
		}

		a.Allowances = append(a.Allowances, al)
	}

	return &a, nil
}

// Delegations returns a list of delegations.
func (c *storageClient) Delegations(ctx context.Context, r *http.Request) (*DelegationList, error) {
	cid, ok := ctx.Value(ChainIDContextKey).(string)
	if !ok {
		return nil, common.ErrBadChainID
	}
	qf := NewQueryFactory(cid)

	pagination, err := common.NewPagination(r)
	if err != nil {
		c.logger.Info("pagination failed",
			"request_id", ctx.Value(RequestIDContextKey),
			"err", err.Error(),
		)
		return nil, common.ErrBadRequest
	}

	rows, err := c.db.Query(
		ctx,
		qf.DelegationsQuery(),
		chi.URLParam(r, "address"),
		pagination.Order,
		pagination.Limit,
		pagination.Offset,
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
		var escrowBalanceActive uint64
		var escrowTotalSharesActive uint64
		if err := rows.Scan(
			&d.ValidatorAddress,
			&d.Shares,
			&escrowBalanceActive,
			&escrowTotalSharesActive,
		); err != nil {
			c.logger.Info("row scan failed",
				"request_id", ctx.Value(RequestIDContextKey),
				"err", err.Error(),
			)
			return nil, common.ErrStorageError
		}

		d.Amount = uint64(float64(d.Shares) * float64(escrowBalanceActive) / float64(escrowTotalSharesActive))

		ds.Delegations = append(ds.Delegations, d)
	}

	return &ds, nil
}

// DebondingDelegations returns a list of debonding delegations.
func (c *storageClient) DebondingDelegations(ctx context.Context, r *http.Request) (*DebondingDelegationList, error) {
	cid, ok := ctx.Value(ChainIDContextKey).(string)
	if !ok {
		return nil, common.ErrBadChainID
	}
	qf := NewQueryFactory(cid)

	pagination, err := common.NewPagination(r)
	if err != nil {
		c.logger.Info("pagination failed",
			"request_id", ctx.Value(RequestIDContextKey),
			"err", err.Error(),
		)
		return nil, common.ErrBadRequest
	}

	rows, err := c.db.Query(
		ctx,
		qf.DebondingDelegationsQuery(),
		chi.URLParam(r, "address"),
		pagination.Order,
		pagination.Limit,
		pagination.Offset,
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
		var escrowBalanceDebonding uint64
		var escrowTotalSharesDebonding uint64
		if err := rows.Scan(
			&d.ValidatorAddress,
			&d.Shares,
			&d.DebondEnd,
			&escrowBalanceDebonding,
			&escrowTotalSharesDebonding,
		); err != nil {
			c.logger.Info("row scan failed",
				"request_id", ctx.Value(RequestIDContextKey),
				"err", err.Error(),
			)
			return nil, common.ErrStorageError
		}
		d.Amount = uint64(float64(d.Shares) * float64(escrowBalanceDebonding) / float64(escrowTotalSharesDebonding))
		ds.DebondingDelegations = append(ds.DebondingDelegations, d)
	}

	return &ds, nil
}

// Epochs returns a list of consensus epochs.
func (c *storageClient) Epochs(ctx context.Context, r *http.Request) (*EpochList, error) {
	cid, ok := ctx.Value(ChainIDContextKey).(string)
	if !ok {
		return nil, common.ErrBadChainID
	}
	qf := NewQueryFactory(cid)

	pagination, err := common.NewPagination(r)
	if err != nil {
		c.logger.Info("pagination failed",
			"request_id", ctx.Value(RequestIDContextKey),
			"err", err.Error(),
		)
		return nil, common.ErrBadRequest
	}

	rows, err := c.db.Query(
		ctx,
		qf.EpochsQuery(),
		pagination.Order,
		pagination.Limit,
		pagination.Offset,
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
func (c *storageClient) Epoch(ctx context.Context, r *http.Request) (*Epoch, error) {
	cid, ok := ctx.Value(ChainIDContextKey).(string)
	if !ok {
		return nil, common.ErrBadChainID
	}
	qf := NewQueryFactory(cid)

	var e Epoch
	if err := c.db.QueryRow(
		ctx,
		qf.EpochQuery(),
		chi.URLParam(r, "epoch"),
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
func (c *storageClient) Proposals(ctx context.Context, r *http.Request) (*ProposalList, error) {
	cid, ok := ctx.Value(ChainIDContextKey).(string)
	if !ok {
		return nil, common.ErrBadChainID
	}
	qf := NewQueryFactory(cid)

	params := r.URL.Query()

	var submitter *string
	if v := params.Get("submitter"); v != "" {
		submitter = &v
	}
	var state *string
	if v := params.Get("state"); v != "" {
		state = &v
	}

	pagination, err := common.NewPagination(r)
	if err != nil {
		c.logger.Info("pagination failed",
			"request_id", ctx.Value(RequestIDContextKey),
			"err", err.Error(),
		)
		return nil, common.ErrBadRequest
	}

	rows, err := c.db.Query(
		ctx,
		qf.ProposalsQuery(),
		submitter,
		state,
		pagination.Order,
		pagination.Limit,
		pagination.Offset,
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
		if err := rows.Scan(
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
			c.logger.Info("row scan failed",
				"request_id", ctx.Value(RequestIDContextKey),
				"err", err.Error(),
			)
			return nil, common.ErrStorageError
		}

		ps.Proposals = append(ps.Proposals, p)
	}

	return &ps, nil
}

// Proposal returns a governance proposal.
func (c *storageClient) Proposal(ctx context.Context, r *http.Request) (*Proposal, error) {
	cid, ok := ctx.Value(ChainIDContextKey).(string)
	if !ok {
		return nil, common.ErrBadChainID
	}
	qf := NewQueryFactory(cid)

	var p Proposal
	if err := c.db.QueryRow(
		ctx,
		qf.ProposalQuery(),
		chi.URLParam(r, "proposal_id"),
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
		c.logger.Info("row scan failed",
			"request_id", ctx.Value(RequestIDContextKey),
			"err", err.Error(),
		)
		return nil, common.ErrStorageError
	}

	return &p, nil
}

// ProposalVotes returns votes for a governance proposal.
func (c *storageClient) ProposalVotes(ctx context.Context, r *http.Request) (*ProposalVotes, error) {
	cid, ok := ctx.Value(ChainIDContextKey).(string)
	if !ok {
		return nil, common.ErrBadChainID
	}
	qf := NewQueryFactory(cid)

	pagination, err := common.NewPagination(r)
	if err != nil {
		c.logger.Info("pagination failed",
			"request_id", ctx.Value(RequestIDContextKey),
			"err", err.Error(),
		)
		return nil, common.ErrBadRequest
	}

	id, err := strconv.ParseUint(chi.URLParam(r, "proposal_id"), 10, 64)
	if err != nil {
		return nil, common.ErrBadRequest
	}

	rows, err := c.db.Query(
		ctx,
		qf.ProposalVotesQuery(),
		id,
		pagination.Order,
		pagination.Limit,
		pagination.Offset,
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
	vs.ProposalID = id

	return &vs, nil
}

// Validators returns a list of validators.
func (c *storageClient) Validators(ctx context.Context, r *http.Request) (*ValidatorList, error) {
	cid, ok := ctx.Value(ChainIDContextKey).(string)
	if !ok {
		return nil, common.ErrBadChainID
	}
	qf := NewQueryFactory(cid)

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

	order := "voting_power"
	pagination := common.Pagination{
		Order:  &order,
		Limit:  1000,
		Offset: 0,
	}

	rows, err := c.db.Query(
		ctx,
		qf.ValidatorsDataQuery(),
		pagination.Order,
		pagination.Limit,
		pagination.Offset,
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
		var schedule staking.CommissionSchedule
		if err := rows.Scan(
			&v.EntityID,
			&v.EntityAddress,
			&v.NodeID,
			&v.Escrow,
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
func (c *storageClient) Validator(ctx context.Context, r *http.Request) (*Validator, error) {
	cid, ok := ctx.Value(ChainIDContextKey).(string)
	if !ok {
		return nil, common.ErrBadChainID
	}
	qf := NewQueryFactory(cid)

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
		chi.URLParam(r, "entity_id"),
	)

	var v Validator
	var schedule staking.CommissionSchedule
	if err := row.Scan(
		&v.EntityID,
		&v.EntityAddress,
		&v.NodeID,
		&v.Escrow,
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

// TransactionsPerSecond returns a list of tps checkpoint values.
func (c *storageClient) TransactionsPerSecond(ctx context.Context, r *http.Request) (*TpsCheckpointList, error) {
	qf := NewQueryFactory(strcase.ToSnake(LatestChainID))

	pagination := common.Pagination{
		Limit:  100,
		Offset: 0,
	}

	rows, err := c.db.Query(
		ctx,
		qf.TpsCheckpointQuery(),
		pagination.Limit,
		pagination.Offset,
	)
	if err != nil {
		c.logger.Info("query failed",
			"request_id", ctx.Value(RequestIDContextKey),
			"err", err.Error(),
		)
		return nil, common.ErrStorageError
	}
	defer rows.Close()

	ts := TpsCheckpointList{
		IntervalMinutes: tpsWindowSizeMinutes,
		TpsCheckpoints:  []TpsCheckpoint{},
	}
	for rows.Next() {
		var d struct {
			Hour     time.Time
			MinSlot  int
			TxVolume uint64
		}
		if err := rows.Scan(
			&d.Hour,
			&d.MinSlot,
			&d.TxVolume,
		); err != nil {
			c.logger.Info("query failed",
				"err", err.Error(),
			)
			return nil, common.ErrStorageError
		}

		t := TpsCheckpoint{
			Timestamp: d.Hour.Add(time.Duration(tpsWindowSizeMinutes*d.MinSlot) * time.Minute),
			TxVolume:  d.TxVolume,
		}
		ts.TpsCheckpoints = append(ts.TpsCheckpoints, t)
	}

	return &ts, nil
}

// DailyVolumes returns a list of daily transaction volumes.
func (c *storageClient) DailyVolumes(ctx context.Context, r *http.Request) (*VolumeList, error) {
	qf := NewQueryFactory(strcase.ToSnake(LatestChainID))

	pagination := common.Pagination{
		Limit:  100,
		Offset: 0,
	}

	rows, err := c.db.Query(
		ctx,
		qf.TxVolumesQuery(),
		pagination.Order,
		pagination.Limit,
		pagination.Offset,
	)
	if err != nil {
		c.logger.Info("query failed",
			"request_id", ctx.Value(RequestIDContextKey),
			"err", err.Error(),
		)
		return nil, common.ErrStorageError
	}
	defer rows.Close()

	vs := VolumeList{
		Volumes: []Volume{},
	}
	for rows.Next() {
		var v Volume
		if err := rows.Scan(
			&v.Date,
			&v.TxVolume,
		); err != nil {
			c.logger.Info("query failed",
				"err", err.Error(),
			)
			return nil, common.ErrStorageError
		}

		vs.Volumes = append(vs.Volumes, v)
	}

	return &vs, nil
}
