package client

import (
	"context"
	"time"

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

// StorageClient is a wrapper around a storage.TargetStorage
// with knowledge of network semantics.
type StorageClient struct {
	db     storage.TargetStorage
	logger *log.Logger
}

// NewStorageClient creates a new storage client.
func NewStorageClient(db storage.TargetStorage, l *log.Logger) *StorageClient {
	return &StorageClient{db, l}
}

// Shutdown closes the backing TargetStorage.
func (c *StorageClient) Shutdown() {
	c.db.Shutdown()
}

// Status returns status information for the Oasis Indexer.
func (c *StorageClient) Status(ctx context.Context) (*Status, error) {
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
func (c *StorageClient) Blocks(ctx context.Context, r *BlocksRequest, p *common.Pagination) (*BlockList, error) {
	cid, ok := ctx.Value(ChainIDContextKey).(string)
	if !ok {
		return nil, common.ErrBadChainID
	}
	qf := NewQueryFactory(cid)

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

// Block returns a consensus block.
func (c *StorageClient) Block(ctx context.Context, r *BlockRequest) (*Block, error) {
	cid, ok := ctx.Value(ChainIDContextKey).(string)
	if !ok {
		return nil, common.ErrBadChainID
	}
	qf := NewQueryFactory(cid)

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

	return &b, nil
}

// Transactions returns a list of consensus transactions.
func (c *StorageClient) Transactions(ctx context.Context, r *TransactionsRequest, p *common.Pagination) (*TransactionList, error) {
	cid, ok := ctx.Value(ChainIDContextKey).(string)
	if !ok {
		return nil, common.ErrBadChainID
	}
	qf := NewQueryFactory(cid)

	rows, err := c.db.Query(
		ctx,
		qf.TransactionsQuery(),
		r.Block,
		r.Method,
		r.Sender.String(),
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
func (c *StorageClient) Transaction(ctx context.Context, r *TransactionRequest) (*Transaction, error) {
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
		r.TxHash,
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
func (c *StorageClient) Entities(ctx context.Context, p *common.Pagination) (*EntityList, error) {
	cid, ok := ctx.Value(ChainIDContextKey).(string)
	if !ok {
		return nil, common.ErrBadChainID
	}
	qf := NewQueryFactory(cid)

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
	qf := NewQueryFactory(cid)

	var e Entity
	if err := c.db.QueryRow(
		ctx,
		qf.EntityQuery(),
		r.EntityId,
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
		r.EntityId,
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
	qf := NewQueryFactory(cid)

	rows, err := c.db.Query(
		ctx,
		qf.EntityNodesQuery(),
		r.EntityId,
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
	ns.EntityID = *r.EntityId

	return &ns, nil
}

// EntityNode returns a node controlled by the provided entity.
func (c *StorageClient) EntityNode(ctx context.Context, r *EntityNodeRequest) (*Node, error) {
	cid, ok := ctx.Value(ChainIDContextKey).(string)
	if !ok {
		return nil, common.ErrBadChainID
	}
	qf := NewQueryFactory(cid)

	var n Node
	if err := c.db.QueryRow(
		ctx,
		qf.EntityNodeQuery(),
		r.EntityId,
		r.NodeId,
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
	qf := NewQueryFactory(cid)

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
func (c *StorageClient) Account(ctx context.Context, r *AccountRequest) (*Account, error) {
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
		r.Address.String(),
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
func (c *StorageClient) Delegations(ctx context.Context, r *DelegationsRequest, p *common.Pagination) (*DelegationList, error) {
	cid, ok := ctx.Value(ChainIDContextKey).(string)
	if !ok {
		return nil, common.ErrBadChainID
	}
	qf := NewQueryFactory(cid)

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
func (c *StorageClient) DebondingDelegations(ctx context.Context, r *DebondingDelegationsRequest, p *common.Pagination) (*DebondingDelegationList, error) {
	cid, ok := ctx.Value(ChainIDContextKey).(string)
	if !ok {
		return nil, common.ErrBadChainID
	}
	qf := NewQueryFactory(cid)

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
func (c *StorageClient) Epochs(ctx context.Context, p *common.Pagination) (*EpochList, error) {
	cid, ok := ctx.Value(ChainIDContextKey).(string)
	if !ok {
		return nil, common.ErrBadChainID
	}
	qf := NewQueryFactory(cid)

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
	qf := NewQueryFactory(cid)

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
	qf := NewQueryFactory(cid)

	rows, err := c.db.Query(
		ctx,
		qf.ProposalsQuery(),
		r.Submitter.String(),
		r.State.String(),
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
func (c *StorageClient) Proposal(ctx context.Context, r *ProposalRequest) (*Proposal, error) {
	cid, ok := ctx.Value(ChainIDContextKey).(string)
	if !ok {
		return nil, common.ErrBadChainID
	}
	qf := NewQueryFactory(cid)

	var p Proposal
	if err := c.db.QueryRow(
		ctx,
		qf.ProposalQuery(),
		r.ProposalId,
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
func (c *StorageClient) ProposalVotes(ctx context.Context, r *ProposalVotesRequest, p *common.Pagination) (*ProposalVotes, error) {
	cid, ok := ctx.Value(ChainIDContextKey).(string)
	if !ok {
		return nil, common.ErrBadChainID
	}
	qf := NewQueryFactory(cid)

	rows, err := c.db.Query(
		ctx,
		qf.ProposalVotesQuery(),
		r.ProposalId,
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
	vs.ProposalID = *r.ProposalId

	return &vs, nil
}

// Validators returns a list of validators.
func (c *StorageClient) Validators(ctx context.Context, p *common.Pagination) (*ValidatorList, error) {
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
func (c *StorageClient) Validator(ctx context.Context, r *ValidatorRequest) (*Validator, error) {
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
		r.EntityId,
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
func (c *StorageClient) TransactionsPerSecond(ctx context.Context, p *common.Pagination) (*TpsCheckpointList, error) {
	qf := NewQueryFactory(strcase.ToSnake(LatestChainID))

	rows, err := c.db.Query(
		ctx,
		qf.TpsCheckpointQuery(),
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
			Timestamp: d.Hour.Add(time.Duration(tpsWindowSizeMinutes*d.MinSlot) * time.Minute).UTC(),
			TxVolume:  d.TxVolume,
		}
		ts.TpsCheckpoints = append(ts.TpsCheckpoints, t)
	}

	return &ts, nil
}

// DailyVolumes returns a list of daily transaction volumes.
func (c *StorageClient) DailyVolumes(ctx context.Context, p *common.Pagination) (*VolumeList, error) {
	qf := NewQueryFactory(strcase.ToSnake(LatestChainID))

	rows, err := c.db.Query(
		ctx,
		qf.TxVolumesQuery(),
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
		v.Date = v.Date.UTC()

		vs.Volumes = append(vs.Volumes, v)
	}

	return &vs, nil
}