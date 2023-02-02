package client

import (
	"context"
	"math/big"
	"time"

	"github.com/dgraph-io/ristretto"
	"github.com/jackc/pgtype"
	"github.com/jackc/pgx/v4"
	oasisErrors "github.com/oasisprotocol/oasis-core/go/common/errors"

	beacon "github.com/oasisprotocol/oasis-core/go/beacon/api"
	staking "github.com/oasisprotocol/oasis-core/go/staking/api"

	"github.com/oasisprotocol/oasis-core/go/common/crypto/signature"
	"github.com/oasisprotocol/oasis-indexer/analyzer/util"
	apiCommon "github.com/oasisprotocol/oasis-indexer/api"
	apiTypes "github.com/oasisprotocol/oasis-indexer/api/v1/types"
	common "github.com/oasisprotocol/oasis-indexer/common"
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

func toString(b *BigInt) *string {
	if b == nil {
		return nil
	}
	s := b.String()
	return &s
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

// Status returns status information for the Oasis Indexer.
func (c *StorageClient) Status(ctx context.Context) (*Status, error) {
	s := Status{
		LatestChainID: c.chainID,
	}
	if err := c.db.QueryRow(
		ctx,
		QueryFactoryFromCtx(ctx).StatusQuery(),
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
	rows, err := c.db.Query(
		ctx,
		QueryFactoryFromCtx(ctx).BlocksQuery(),
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
	defer rows.Close()

	bs := BlockList{
		Blocks: []Block{},
	}
	for rows.Next() {
		var b Block
		if err := rows.Scan(&b.Height, &b.Hash, &b.Timestamp, &b.NumTransactions); err != nil {
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
		QueryFactoryFromCtx(ctx).BlockQuery(),
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
	rows, err := c.db.Query(
		ctx,
		QueryFactoryFromCtx(ctx).TransactionsQuery(),
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
	defer rows.Close()

	ts := TransactionList{
		Transactions: []Transaction{},
	}
	for rows.Next() {
		var t Transaction
		var code uint64
		if err := rows.Scan(
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
		QueryFactoryFromCtx(ctx).TransactionQuery(),
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
	var rows pgx.Rows
	var err error
	rows, err = c.db.Query(
		ctx,
		QueryFactoryFromCtx(ctx).EventsQuery(),
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
	defer rows.Close()

	es := EventList{
		Events: []Event{},
	}

	for rows.Next() {
		var e Event
		if err := rows.Scan(&e.Block, &e.TxIndex, &e.TxHash, &e.Type, &e.Body); err != nil {
			return nil, wrapError(err)
		}
		es.Events = append(es.Events, e)
	}

	return &es, nil
}

// Entities returns a list of registered entities.
func (c *StorageClient) Entities(ctx context.Context, p apiTypes.GetConsensusEntitiesParams) (*EntityList, error) {
	rows, err := c.db.Query(
		ctx,
		QueryFactoryFromCtx(ctx).EntitiesQuery(),
		p.Limit,
		p.Offset,
	)
	if err != nil {
		return nil, wrapError(err)
	}
	defer rows.Close()

	es := EntityList{
		Entities: []Entity{},
	}
	for rows.Next() {
		var e Entity
		if err := rows.Scan(&e.ID, &e.Address); err != nil {
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
		QueryFactoryFromCtx(ctx).EntityQuery(),
		entityID.String(),
	).Scan(&e.ID, &e.Address); err != nil {
		return nil, wrapError(err)
	}

	nodeRows, err := c.db.Query(
		ctx,
		QueryFactoryFromCtx(ctx).EntityNodeIdsQuery(),
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
	rows, err := c.db.Query(
		ctx,
		QueryFactoryFromCtx(ctx).EntityNodesQuery(),
		entityID.String(),
		r.Limit,
		r.Offset,
	)
	if err != nil {
		return nil, wrapError(err)
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
		QueryFactoryFromCtx(ctx).EntityNodeQuery(),
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
	rows, err := c.db.Query(
		ctx,
		QueryFactoryFromCtx(ctx).AccountsQuery(),
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
	defer rows.Close()

	as := AccountList{
		Accounts: []Account{},
	}
	for rows.Next() {
		a := Account{AddressPreimage: &AddressPreimage{}}
		var preimageContext *string
		if err := rows.Scan(
			&a.Address,
			&a.Nonce,
			&a.Available,
			&a.Escrow,
			&a.Debonding,
			&preimageContext,
			&a.AddressPreimage.ContextVersion,
			&a.AddressPreimage.AddressData,
		); err != nil {
			return nil, wrapError(err)
		}
		if preimageContext != nil {
			a.AddressPreimage.Context = AddressDerivationContext(*preimageContext)
		} else {
			a.AddressPreimage = nil
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
		AddressPreimage:             &AddressPreimage{},
		DelegationsBalance:          &common.BigInt{},
		DebondingDelegationsBalance: &common.BigInt{},
		RuntimeSdkBalances:          &[]RuntimeSdkBalance{},
		RuntimeEvmBalances:          &[]RuntimeEvmBalance{},
	}
	var preimageContext *string
	var delegationsBalanceNum pgtype.Numeric
	var debondingDelegationsBalanceNum pgtype.Numeric
	err := c.db.QueryRow(
		ctx,
		QueryFactoryFromCtx(ctx).AccountQuery(),
		address.String(),
	).Scan(
		&a.Address,
		&a.Nonce,
		&a.Available,
		&a.Escrow,
		&a.Debonding,
		&preimageContext,
		&a.AddressPreimage.ContextVersion,
		&a.AddressPreimage.AddressData,
		&delegationsBalanceNum,
		&debondingDelegationsBalanceNum,
	)
	if err == nil { //nolint:gocritic,nestif
		// Convert numeric values to big.Int. pgx has a bug where it doesn't support reading into *big.Int.
		var err2 error
		*a.DelegationsBalance, err2 = common.NumericToBigInt(delegationsBalanceNum)
		if err2 != nil {
			return nil, wrapError(err)
		}
		*a.DebondingDelegationsBalance, err2 = common.NumericToBigInt(debondingDelegationsBalanceNum)
		if err2 != nil {
			return nil, wrapError(err)
		}
		if preimageContext != nil {
			a.AddressPreimage.Context = AddressDerivationContext(*preimageContext)
		} else {
			a.AddressPreimage = nil
		}
	} else if err == pgx.ErrNoRows {
		// An address can have no entries in the consensus `accounts` table (= no balance, nonce, etc)
		// but it's still valid, and it might have balances in the runtimes.
		// Leave the consensus-specific info initialized to defaults.
		a.Address = address.String()
		a.AddressPreimage = nil
	} else {
		return nil, wrapError(err)
	}

	// Get allowances.
	allowanceRows, queryErr := c.db.Query(
		ctx,
		QueryFactoryFromCtx(ctx).AccountAllowancesQuery(),
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

	// Get paratime balances.
	runtimeSdkRows, queryErr := c.db.Query(
		ctx,
		QueryFactoryFromCtx(ctx).AccountRuntimeSdkBalancesQuery(),
		address.String(),
	)
	if queryErr != nil {
		return nil, wrapError(err)
	}
	defer runtimeSdkRows.Close()

	for runtimeSdkRows.Next() {
		b := RuntimeSdkBalance{
			// HACK: 18 is accurate for Emerald and Sapphire, but Cipher has 9.
			// Once we add a non-18-decimals runtime, we'll need to query the runtime for this
			// at analysis time and store it in a table, similar to how we store the EVM token metadata.
			TokenDecimals: 18,
		}
		if err2 := runtimeSdkRows.Scan(
			&b.Runtime,
			&b.Balance,
			&b.TokenSymbol,
		); err2 != nil {
			return nil, wrapError(err2)
		}
		*a.RuntimeSdkBalances = append(*a.RuntimeSdkBalances, b)
	}

	runtimeEvmRows, queryErr := c.db.Query(
		ctx,
		QueryFactoryFromCtx(ctx).AccountRuntimeEvmBalancesQuery(),
		address.String(),
	)
	if queryErr != nil {
		return nil, wrapError(err)
	}
	defer runtimeEvmRows.Close()

	for runtimeEvmRows.Next() {
		b := RuntimeEvmBalance{}
		if err := runtimeEvmRows.Scan(
			&b.Runtime,
			&b.Balance,
			&b.TokenContractAddr,
			&b.TokenSymbol,
			&b.TokenName,
			&b.TokenType,
			&b.TokenDecimals,
		); err != nil {
			return nil, wrapError(err)
		}
		*a.RuntimeEvmBalances = append(*a.RuntimeEvmBalances, b)
	}

	return &a, nil
}

// Delegations returns a list of delegations.
func (c *StorageClient) Delegations(ctx context.Context, address staking.Address, p apiTypes.GetConsensusAccountsAddressDelegationsParams) (*DelegationList, error) {
	rows, err := c.db.Query(
		ctx,
		QueryFactoryFromCtx(ctx).DelegationsQuery(),
		address.String(),
		p.Limit,
		p.Offset,
	)
	if err != nil {
		return nil, wrapError(err)
	}
	defer rows.Close()

	ds := DelegationList{
		Delegations: []Delegation{},
	}
	for rows.Next() {
		var d Delegation
		var shares, escrowBalanceActive, escrowTotalSharesActive common.BigInt
		if err := rows.Scan(
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
	rows, err := c.db.Query(
		ctx,
		QueryFactoryFromCtx(ctx).DebondingDelegationsQuery(),
		address.String(),
		p.Limit,
		p.Offset,
	)
	if err != nil {
		return nil, wrapError(err)
	}
	defer rows.Close()

	ds := DebondingDelegationList{
		DebondingDelegations: []DebondingDelegation{},
	}
	for rows.Next() {
		var d DebondingDelegation
		var shares, escrowBalanceDebonding, escrowTotalSharesDebonding common.BigInt
		if err := rows.Scan(
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
	rows, err := c.db.Query(
		ctx,
		QueryFactoryFromCtx(ctx).EpochsQuery(),
		p.Limit,
		p.Offset,
	)
	if err != nil {
		return nil, wrapError(err)
	}

	es := EpochList{
		Epochs: []Epoch{},
	}
	for rows.Next() {
		var e Epoch
		if err := rows.Scan(&e.ID, &e.StartHeight, &e.EndHeight); err != nil {
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
		QueryFactoryFromCtx(ctx).EpochQuery(),
		epoch,
	).Scan(&e.ID, &e.StartHeight, &e.EndHeight); err != nil {
		return nil, wrapError(err)
	}

	return &e, nil
}

// Proposals returns a list of governance proposals.
func (c *StorageClient) Proposals(ctx context.Context, p apiTypes.GetConsensusProposalsParams) (*ProposalList, error) {
	rows, err := c.db.Query(
		ctx,
		QueryFactoryFromCtx(ctx).ProposalsQuery(),
		p.Submitter,
		p.State,
		p.Limit,
		p.Offset,
	)
	if err != nil {
		return nil, wrapError(err)
	}
	defer rows.Close()

	ps := ProposalList{
		Proposals: []Proposal{},
	}
	for rows.Next() {
		p := Proposal{Target: &ProposalTarget{}}
		var invalidVotesNum pgtype.Numeric
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
		QueryFactoryFromCtx(ctx).ProposalQuery(),
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
	rows, err := c.db.Query(
		ctx,
		QueryFactoryFromCtx(ctx).ProposalVotesQuery(),
		proposalID,
		p.Limit,
		p.Offset,
	)
	if err != nil {
		return nil, wrapError(err)
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
		QueryFactoryFromCtx(ctx).ValidatorsQuery(),
	).Scan(&epoch.ID, &epoch.StartHeight); err != nil {
		return nil, wrapError(err)
	}

	rows, err := c.db.Query(
		ctx,
		QueryFactoryFromCtx(ctx).ValidatorsDataQuery(),
		p.Limit,
		p.Offset,
	)
	if err != nil {
		return nil, wrapError(err)
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
		QueryFactoryFromCtx(ctx).ValidatorQuery(),
	).Scan(&epoch.ID, &epoch.StartHeight); err != nil {
		return nil, wrapError(err)
	}

	row := c.db.QueryRow(
		ctx,
		QueryFactoryFromCtx(ctx).ValidatorDataQuery(),
		entityID.String(),
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
	rows, err := c.db.Query(
		ctx,
		QueryFactoryFromCtx(ctx).RuntimeBlocksQuery(),
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
	defer rows.Close()

	bs := RuntimeBlockList{
		Blocks: []RuntimeBlock{},
	}
	for rows.Next() {
		var b RuntimeBlock
		if err := rows.Scan(&b.Round, &b.Hash, &b.Timestamp, &b.NumTransactions, &b.Size, &b.GasUsed); err != nil {
			return nil, wrapError(err)
		}
		b.Timestamp = b.Timestamp.UTC()

		bs.Blocks = append(bs.Blocks, b)
	}

	return &bs, nil
}

// RuntimeTransactions returns a list of runtime transactions.
func (c *StorageClient) RuntimeTransactions(ctx context.Context, p apiTypes.GetRuntimeTransactionsParams) (*RuntimeTransactionList, error) {
	rows, err := c.db.Query(
		ctx,
		QueryFactoryFromCtx(ctx).RuntimeTransactionsQuery(),
		p.Block,
		nil, // tx_hash; filter not supported by this endpoint
		p.Limit,
		p.Offset,
	)
	if err != nil {
		return nil, wrapError(err)
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
			&t.Timestamp,
			&t.Raw,
			&t.ResultRaw,
		); err != nil {
			return nil, wrapError(err)
		}

		ts.Transactions = append(ts.Transactions, t)
	}

	return &ts, nil
}

// RuntimeTransaction returns a single runtime transaction.
func (c *StorageClient) RuntimeTransaction(ctx context.Context, txHash string) (*RuntimeTransaction, error) {
	t := RuntimeTransaction{}
	err := c.db.QueryRow(
		ctx,
		QueryFactoryFromCtx(ctx).RuntimeTransactionsQuery(),
		nil, // block; filter not supported by this endpoint
		txHash,
		1, // limit
		0, // offset
	).Scan(
		&t.Round,
		&t.Index,
		&t.Hash,
		&t.EthHash,
		&t.Timestamp,
		&t.Raw,
		&t.ResultRaw,
	)
	if err != nil {
		return nil, wrapError(err)
	}

	return &t, nil
}

// RuntimeEvents returns a list of runtime events.
func (c *StorageClient) RuntimeEvents(ctx context.Context, p apiTypes.GetRuntimeEventsParams) (*RuntimeEventList, error) {
	rows, err := c.db.Query(
		ctx,
		QueryFactoryFromCtx(ctx).RuntimeEventsQuery(),
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
	defer rows.Close()

	es := RuntimeEventList{
		Events: []RuntimeEvent{},
	}
	for rows.Next() {
		var e RuntimeEvent
		if err := rows.Scan(
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

func (c *StorageClient) RuntimeTokens(ctx context.Context, p apiTypes.GetRuntimeEvmTokensParams) (*EvmTokenList, error) {
	rows, err := c.db.Query(
		ctx,
		QueryFactoryFromCtx(ctx).EvmTokensQuery(),
		p.Limit,
		p.Offset,
	)
	if err != nil {
		return nil, wrapError(err)
	}
	defer rows.Close()

	ts := EvmTokenList{
		EvmTokens: []EvmToken{},
	}
	for rows.Next() {
		var t EvmToken
		var totalSupplyNum pgtype.Numeric
		if err2 := rows.Scan(
			&t.ContractAddr,
			&t.EvmContractAddr,
			&t.Name,
			&t.Symbol,
			&t.Decimals,
			&totalSupplyNum,
			&t.Type,
			&t.NumHolders,
		); err2 != nil {
			return nil, wrapError(err)
		}
		if totalSupplyNum.Status == pgtype.Present {
			t.TotalSupply = &common.BigInt{}
			*t.TotalSupply, err = common.NumericToBigInt(totalSupplyNum)
			if err != nil {
				return nil, wrapError(err)
			}
		}

		ts.EvmTokens = append(ts.EvmTokens, t)
	}

	return &ts, nil
}

// TxVolumes returns a list of transaction volumes per time bucket.
func (c *StorageClient) TxVolumes(ctx context.Context, layer apiTypes.Layer, p apiTypes.GetLayerStatsTxVolumeParams) (*TxVolumeList, error) {
	var query string
	if *p.BucketSizeSeconds == 300 {
		query = QueryFactoryFromCtx(ctx).FineTxVolumesQuery()
	} else {
		var day uint32 = 86400
		p.BucketSizeSeconds = &day
		query = QueryFactoryFromCtx(ctx).TxVolumesQuery()
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
		var d struct {
			BucketStart time.Time
			TxVolume    uint64
		}
		if err := rows.Scan(
			&d.BucketStart,
			&d.TxVolume,
		); err != nil {
			return nil, wrapError(err)
		}

		t := TxVolume{
			BucketStart: d.BucketStart.UTC(),
			TxVolume:    d.TxVolume,
		}
		ts.Buckets = append(ts.Buckets, t)
	}

	return &ts, nil
}
