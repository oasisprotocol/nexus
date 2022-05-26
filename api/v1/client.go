package v1

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi"
	"github.com/iancoleman/strcase"
	"github.com/oasislabs/oasis-block-indexer/go/log"
	"github.com/oasislabs/oasis-block-indexer/go/storage"
	oasisErrors "github.com/oasisprotocol/oasis-core/go/common/errors"

	"github.com/oasislabs/oasis-block-indexer/go/api/common"
)

// QueryBuilder is used for building queries to submit to storage.
type QueryBuilder struct {
	inner *strings.Builder
	db    storage.TargetStorage
}

// NewQueryBuilder creates a new query builder, with the provided SQL query
// as the base query.
func NewQueryBuilder(sql string, db storage.TargetStorage) *QueryBuilder {
	inner := &strings.Builder{}
	inner.WriteString(sql)
	return &QueryBuilder{inner, db}
}

// AddPagination adds pagination to the query builder.
func (q *QueryBuilder) AddPagination(_ctx context.Context, p common.Pagination) error {
	_, err := q.inner.WriteString(
		fmt.Sprintf("\n\tORDER BY %s\n\tLIMIT %d\n\tOFFSET %d", p.Order, p.Limit, p.Offset),
	)
	return err
}

// AddTimestamp adds time travel to the query builder, at the time of the provided height.
func (q *QueryBuilder) AddTimestamp(ctx context.Context, height int64) error {
	row, err := q.db.QueryRow(
		ctx,
		fmt.Sprintf(`
			SELECT processed_time
				FROM %s.processed_blocks
				WHERE height = $1
				ORDER BY processed_time DESC
				LIMIT 1`,
			strcase.ToSnake(LatestChainID)),
		height,
	)
	if err != nil {
		return err
	}

	var processedTime time.Time
	if err := row.Scan(&processedTime); err != nil {
		return err
	}

	_, err = q.inner.WriteString(fmt.Sprintf("\n\tAS OF SYSTEM TIME %s", processedTime.String()))
	return err
}

// AddFilters adds the provided filters to the query builder.
func (q *QueryBuilder) AddFilters(_ctx context.Context, filters []string) error {
	if len(filters) > 0 {
		_, err := q.inner.WriteString(fmt.Sprintf("\n\tWHERE %s", strings.Join(filters, " AND ")))
		return err
	}
	return nil
}

// String returns the string representation of the query.
func (q *QueryBuilder) String() string {
	return q.inner.String()
}

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
	row, err := c.db.QueryRow(
		ctx,
		fmt.Sprintf(`
			SELECT height, processed_time
				FROM %s.processed_blocks
				ORDER BY processed_time DESC
				LIMIT 1`,
			strcase.ToSnake(LatestChainID)),
	)
	if err != nil {
		c.logger.Info("query failed",
			"request_id", ctx.Value(RequestIDContextKey),
			"err", err.Error(),
		)
		return nil, common.ErrStorageError
	}

	s := Status{
		LatestChainID: LatestChainID,
	}
	if err := row.Scan(&s.LatestBlock, &s.LatestUpdate); err != nil {
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
	chainID, ok := ctx.Value(ChainIDContextKey).(string)
	if !ok {
		return nil, common.ErrBadChainID
	}

	qb := NewQueryBuilder(fmt.Sprintf(`
			SELECT height, block_hash, time
				FROM %s.blocks`,
		chainID), c.db)

	params := r.URL.Query()

	var filters []string
	for param, condition := range map[string]string{
		"from":   "height >= %s",
		"to":     "height <= %s",
		"after":  "time >= TIMESTAMP %s",
		"before": "time <= TIMESTAMP %s",
	} {
		if v := params.Get(param); v != "" {
			filters = append(filters, fmt.Sprintf(condition, v))
		}
	}
	qb.AddFilters(ctx, filters)

	pagination, err := common.NewPagination(r)
	if err != nil {
		c.logger.Info("pagination failed",
			"request_id", ctx.Value(RequestIDContextKey),
			"err", err.Error(),
		)
		return nil, common.ErrBadRequest
	}
	qb.AddPagination(ctx, pagination)

	rows, err := c.db.Query(ctx, qb.String())
	if err != nil {
		c.logger.Info("query failed",
			"request_id", ctx.Value(RequestIDContextKey),
			"err", err.Error(),
		)
		return nil, common.ErrStorageError
	}
	defer rows.Close()

	var bs BlockList
	for rows.Next() {
		var b Block
		if err := rows.Scan(&b.Height, &b.Hash, &b.Timestamp); err != nil {
			c.logger.Info("row scan failed",
				"request_id", ctx.Value(RequestIDContextKey),
				"err", err.Error(),
			)
			return nil, common.ErrStorageError
		}

		bs.Blocks = append(bs.Blocks, b)
	}

	return &bs, nil
}

// Block returns a consensus block.
func (c *storageClient) Block(ctx context.Context, r *http.Request) (*Block, error) {
	chainID, ok := ctx.Value(ChainIDContextKey).(string)
	if !ok {
		return nil, common.ErrBadChainID
	}

	row, err := c.db.QueryRow(
		ctx,
		fmt.Sprintf(`
			SELECT height, block_hash, time
				FROM %s.blocks
				WHERE height = $1::bigint`,
			chainID),
		chi.URLParam(r, "height"),
	)
	if err != nil {
		c.logger.Info("query failed",
			"request_id", ctx.Value(RequestIDContextKey),
			"err", err.Error(),
		)
		return nil, common.ErrStorageError
	}

	var b Block
	if err := row.Scan(&b.Height, &b.Hash, &b.Timestamp); err != nil {
		c.logger.Info("row scan failed",
			"request_id", ctx.Value(RequestIDContextKey),
			"err", err.Error(),
		)
		return nil, common.ErrStorageError
	}

	return &b, nil
}

// Transactions returns a list of consensus transactions.
func (c *storageClient) Transactions(ctx context.Context, r *http.Request) (*TransactionList, error) {
	chainID, ok := ctx.Value(ChainIDContextKey).(string)
	if !ok {
		return nil, common.ErrBadChainID
	}

	qb := NewQueryBuilder(fmt.Sprintf(`
			SELECT block, txn_hash, nonce, fee_amount, method, body, code
				FROM %s.transactions`,
		chainID), c.db)

	params := r.URL.Query()

	var filters []string
	for param, condition := range map[string]string{
		"block":  "block = %s",
		"method": "method = %s",
		"sender": "sender = %s",
		"minFee": "fee_amount >= %s",
		"maxFee": "fee_amount <= %s",
		"code":   "code = %s",
	} {
		if v := params.Get(param); v != "" {
			filters = append(filters, fmt.Sprintf(condition, v))
		}
	}
	qb.AddFilters(ctx, filters)
	pagination, err := common.NewPagination(r)
	if err != nil {
		c.logger.Info("pagination failed",
			"request_id", ctx.Value(RequestIDContextKey),
			"err", err.Error(),
		)
		return nil, common.ErrBadRequest
	}
	qb.AddPagination(ctx, pagination)

	rows, err := c.db.Query(ctx, qb.String())
	if err != nil {
		c.logger.Info("query failed",
			"request_id", ctx.Value(RequestIDContextKey),
			"err", err.Error(),
		)
		return nil, common.ErrStorageError
	}
	defer rows.Close()

	var ts TransactionList
	for rows.Next() {
		var t Transaction
		var code uint64
		if err := rows.Scan(
			&t.Height,
			&t.Hash,
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
	chainID, ok := ctx.Value(ChainIDContextKey).(string)
	if !ok {
		return nil, common.ErrBadChainID
	}

	row, err := c.db.QueryRow(
		ctx,
		fmt.Sprintf(`
			SELECT block, txn_hash, nonce, fee_amount, method, body, code
				FROM %s.transactions
				WHERE txn_hash = $1::hash`, chainID),
		chi.URLParam(r, "txn_hash"),
	)
	if err != nil {
		c.logger.Info("query failed",
			"request_id", ctx.Value(RequestIDContextKey),
			"err", err.Error(),
		)
		return nil, common.ErrStorageError
	}

	var t Transaction
	var code uint64
	if err := row.Scan(
		&t.Height,
		&t.Hash,
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
	chainID, ok := ctx.Value(ChainIDContextKey).(string)
	if !ok {
		return nil, common.ErrBadChainID
	}

	qb := NewQueryBuilder(fmt.Sprintf("SELECT id, address FROM %s.entities", chainID), c.db)

	params := r.URL.Query()
	if v := params.Get("height"); v != "" {
		if h, err := strconv.ParseInt(v, 10, 64); err != nil {
			qb.AddTimestamp(ctx, h)
		}
	}

	pagination, err := common.NewPagination(r)
	if err != nil {
		c.logger.Info("pagination failed",
			"request_id", ctx.Value(RequestIDContextKey),
			"err", err.Error(),
		)
		return nil, common.ErrBadRequest
	}
	qb.AddPagination(ctx, pagination)

	rows, err := c.db.Query(ctx, qb.String())
	if err != nil {
		c.logger.Info("query failed",
			"request_id", ctx.Value(RequestIDContextKey),
			"err", err.Error(),
		)
		return nil, common.ErrStorageError
	}
	defer rows.Close()

	var es EntityList
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
	chainID, ok := ctx.Value(ChainIDContextKey).(string)
	if !ok {
		return nil, common.ErrBadChainID
	}

	qb := NewQueryBuilder(fmt.Sprintf("SELECT id, address FROM %s.entities", chainID), c.db)

	params := r.URL.Query()
	if v := params.Get("height"); v != "" {
		if h, err := strconv.ParseInt(v, 10, 64); err != nil {
			qb.AddTimestamp(ctx, h)
		}
	}
	qb.AddFilters(ctx, []string{"id = $1::text"})

	entityRow, err := c.db.QueryRow(
		ctx,
		qb.String(),
		chi.URLParam(r, "entity_id"),
	)
	if err != nil {
		c.logger.Info("query failed",
			"request_id", ctx.Value(RequestIDContextKey),
			"err", err.Error(),
		)
		return nil, common.ErrStorageError
	}

	var e Entity
	if err := entityRow.Scan(&e.ID, &e.Address); err != nil {
		c.logger.Info("row scan failed",
			"request_id", ctx.Value(RequestIDContextKey),
			"err", err.Error(),
		)
		return nil, common.ErrStorageError
	}

	qb = NewQueryBuilder(fmt.Sprintf("SELECT id FROM %s.nodes", chainID), c.db)
	if v := params.Get("height"); v != "" {
		if h, err := strconv.ParseInt(v, 10, 64); err != nil {
			qb.AddTimestamp(ctx, h)
		}
	}
	qb.AddFilters(ctx, []string{"entity_id = $1::text"})

	nodeRows, err := c.db.Query(
		ctx,
		qb.String(),
		chi.URLParam(r, "entity_id"),
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
	chainID, ok := ctx.Value(ChainIDContextKey).(string)
	if !ok {
		return nil, common.ErrBadChainID
	}

	qb := NewQueryBuilder(fmt.Sprintf(`
			SELECT id, entity_id, expiration, tls_pubkey, tls_next_pubkey, p2p_pubkey, consensus_pubkey, roles
				FROM %s.nodes`,
		chainID), c.db)

	params := r.URL.Query()
	if v := params.Get("height"); v != "" {
		if h, err := strconv.ParseInt(v, 10, 64); err != nil {
			qb.AddTimestamp(ctx, h)
		}
	}

	qb.AddFilters(ctx, []string{"entity_id = $1::text"})

	pagination, err := common.NewPagination(r)
	if err != nil {
		c.logger.Info("pagination failed",
			"request_id", ctx.Value(RequestIDContextKey),
			"err", err.Error(),
		)
		return nil, common.ErrBadRequest
	}
	qb.AddPagination(ctx, pagination)

	id := chi.URLParam(r, "entity_id")
	rows, err := c.db.Query(ctx, qb.String(), id)
	if err != nil {
		c.logger.Info("query failed",
			"request_id", ctx.Value(RequestIDContextKey),
			"err", err.Error(),
		)
		return nil, common.ErrStorageError
	}
	defer rows.Close()

	var ns NodeList
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
	chainID, ok := ctx.Value(ChainIDContextKey).(string)
	if !ok {
		return nil, common.ErrBadChainID
	}

	qb := NewQueryBuilder(fmt.Sprintf(`
			SELECT id, entity_id, expiration, tls_pubkey, tls_next_pubkey, p2p_pubkey, consensus_pubkey, roles
				FROM %s.nodes`,
		chainID), c.db)

	params := r.URL.Query()
	if v := params.Get("height"); v != "" {
		if h, err := strconv.ParseInt(v, 10, 64); err != nil {
			qb.AddTimestamp(ctx, h)
		}
	}
	qb.AddFilters(ctx, []string{"entity_id = $1::text", "id = $2::text"})

	row, err := c.db.QueryRow(
		ctx,
		qb.String(),
		chi.URLParam(r, "entity_id"),
		chi.URLParam(r, "node_id"),
	)
	if err != nil {
		c.logger.Info("query failed",
			"request_id", ctx.Value(RequestIDContextKey),
			"err", err.Error(),
		)
		return nil, common.ErrStorageError
	}

	var n Node
	if err := row.Scan(
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
	chainID, ok := ctx.Value(ChainIDContextKey).(string)
	if !ok {
		return nil, common.ErrBadChainID
	}

	qb := NewQueryBuilder(fmt.Sprintf(`
			SELECT address, nonce, general_balance, escrow_balance_active, escrow_balance_debonding
				FROM %s.accounts`,
		chainID), c.db)

	params := r.URL.Query()
	if v := params.Get("height"); v != "" {
		if h, err := strconv.ParseInt(v, 10, 64); err != nil {
			qb.AddTimestamp(ctx, h)
		}
	}

	var filters []string
	for param, condition := range map[string]string{
		"minAvailable":    "general_balance >= %s",
		"maxAvailable":    "general_balance <= %s",
		"minEscrow":       "escrow_balance_active >= %s",
		"maxEscrow":       "escrow_balance_active <= %s",
		"minDebonding":    "escrow_balance_debonding >= %s",
		"maxDebonding":    "escrow_balance_debonding <= %s",
		"minTotalBalance": "general_balance + escrow_balance_active + escrow_balance_debonding <= %s",
		"maxTotalBalance": "general_balance + escrow_balance_active + escrow_balance_debonding <= %s",
	} {
		if v := params.Get(param); v != "" {
			filters = append(filters, fmt.Sprintf(condition, v))
		}
	}
	qb.AddFilters(ctx, filters)

	pagination, err := common.NewPagination(r)
	if err != nil {
		c.logger.Info("pagination failed",
			"request_id", ctx.Value(RequestIDContextKey),
			"err", err.Error(),
		)
		return nil, common.ErrBadRequest
	}
	qb.AddPagination(ctx, pagination)

	rows, err := c.db.Query(ctx, qb.String())
	if err != nil {
		c.logger.Info("query failed",
			"request_id", ctx.Value(RequestIDContextKey),
			"err", err.Error(),
		)
		return nil, common.ErrStorageError
	}
	defer rows.Close()

	var as AccountList
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
		a.Total = a.Available + a.Escrow + a.Debonding

		as.Accounts = append(as.Accounts, a)
	}

	return &as, nil
}

// Account returns a consensus account.
func (c *storageClient) Account(ctx context.Context, r *http.Request) (*Account, error) {
	chainID, ok := ctx.Value(ChainIDContextKey).(string)
	if !ok {
		return nil, common.ErrBadChainID
	}

	qb := NewQueryBuilder(fmt.Sprintf(`
			SELECT address, nonce, general_balance, escrow_balance_active, escrow_balance_debonding
				FROM %s.accounts`,
		chainID), c.db)

	params := r.URL.Query()
	if v := params.Get("height"); v != "" {
		if h, err := strconv.ParseInt(v, 10, 64); err != nil {
			qb.AddTimestamp(ctx, h)
		}
	}
	qb.AddFilters(ctx, []string{"address = $1::text"})

	accountRow, err := c.db.QueryRow(
		ctx,
		qb.String(),
		chi.URLParam(r, "address"),
	)
	if err != nil {
		c.logger.Info("query failed",
			"request_id", ctx.Value(RequestIDContextKey),
			"err", err.Error(),
		)
		return nil, common.ErrStorageError
	}

	var a Account
	if err := accountRow.Scan(
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
	a.Total = a.Available + a.Escrow + a.Debonding

	qb = NewQueryBuilder(fmt.Sprintf(`
			SELECT beneficiary, allowance
				FROM %s.allowances`,
		chainID), c.db)
	if v := params.Get("height"); v != "" {
		if h, err := strconv.ParseInt(v, 10, 64); err != nil {
			qb.AddTimestamp(ctx, h)
		}
	}
	qb.AddFilters(ctx, []string{"owner = $1::text"})

	allowanceRows, err := c.db.Query(
		ctx,
		qb.String(),
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

// Epochs returns a list of consensus epochs.
func (c *storageClient) Epochs(ctx context.Context, r *http.Request) (*EpochList, error) {
	chainID, ok := ctx.Value(ChainIDContextKey).(string)
	if !ok {
		return nil, common.ErrBadChainID
	}

	qb := NewQueryBuilder(fmt.Sprintf(`
			SELECT id, start_height, end_height
				FROM %s.epochs`,
		chainID), c.db)

	// TODO: Add filters.

	pagination, err := common.NewPagination(r)
	if err != nil {
		c.logger.Info("pagination failed",
			"request_id", ctx.Value(RequestIDContextKey),
			"err", err.Error(),
		)
		return nil, common.ErrBadRequest
	}
	qb.AddPagination(ctx, pagination)

	rows, err := c.db.Query(ctx, qb.String())
	if err != nil {
		c.logger.Info("query failed",
			"request_id", ctx.Value(RequestIDContextKey),
			"err", err.Error(),
		)
		return nil, common.ErrStorageError
	}

	var es EpochList
	for rows.Next() {
		var e Epoch
		if err := rows.Scan(&e.ID, &e.StartHeight, &e.EndHeight); err != nil {
			c.logger.Info("row scan failed",
				"request_id", ctx.Value(RequestIDContextKey),
				"err", err.Error(),
			)
			return nil, common.ErrStorageError
		}

		es.Epochs = append(es.Epochs, e)
	}

	return &es, nil
}

// Epoch returns a consensus epoch.
func (c *storageClient) Epoch(ctx context.Context, r *http.Request) (*Epoch, error) {
	chainID, ok := ctx.Value(ChainIDContextKey).(string)
	if !ok {
		return nil, common.ErrBadChainID
	}

	row, err := c.db.QueryRow(
		ctx,
		fmt.Sprintf(`
			SELECT id, start_height, end_height
				FROM %s.epochs
				WHERE id = $1::bigint`,
			chainID),
		chi.URLParam(r, "epoch"),
	)
	if err != nil {
		c.logger.Info("query failed",
			"request_id", ctx.Value(RequestIDContextKey),
			"err", err.Error(),
		)
		return nil, common.ErrStorageError
	}

	var e Epoch
	if err := row.Scan(&e.ID, &e.StartHeight, &e.EndHeight); err != nil {
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
	chainID, ok := ctx.Value(ChainIDContextKey).(string)
	if !ok {
		return nil, common.ErrBadChainID
	}

	qb := NewQueryBuilder(fmt.Sprintf(`
			SELECT id, submitter, state, deposit, handler, cp_target_version, rhp_target_version, rcp_target_version,
					upgrade_epoch, cancels, created_at, closes_at, invalid_votes
				FROM %s.proposals`,
		chainID), c.db)

	params := r.URL.Query()

	var filters []string
	for param, condition := range map[string]string{
		"submitter": "submitter = %s",
		"state":     "state = %s",
	} {
		if v := params.Get(param); v != "" {
			filters = append(filters, fmt.Sprintf(condition, v))
		}
	}
	qb.AddFilters(ctx, filters)

	pagination, err := common.NewPagination(r)
	if err != nil {
		c.logger.Info("pagination failed",
			"request_id", ctx.Value(RequestIDContextKey),
			"err", err.Error(),
		)
		return nil, common.ErrBadRequest
	}
	qb.AddPagination(ctx, pagination)

	rows, err := c.db.Query(ctx, qb.String())
	if err != nil {
		c.logger.Info("query failed",
			"request_id", ctx.Value(RequestIDContextKey),
			"err", err.Error(),
		)
		return nil, common.ErrStorageError
	}
	defer rows.Close()

	var ps ProposalList
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
	chainID, ok := ctx.Value(ChainIDContextKey).(string)
	if !ok {
		return nil, common.ErrBadChainID
	}

	row, err := c.db.QueryRow(
		ctx,
		fmt.Sprintf(`
			SELECT id, submitter, state, deposit, handler, cp_target_version, rhp_target_version, rcp_target_version,
						upgrade_epoch, cancels, created_at, closes_at, invalid_votes
				FROM %s.proposals
				WHERE id = $1::bigint`,
			chainID),
		chi.URLParam(r, "proposal_id"),
	)
	if err != nil {
		c.logger.Info("query failed",
			"request_id", ctx.Value(RequestIDContextKey),
			"err", err.Error(),
		)
		return nil, common.ErrStorageError
	}

	var p Proposal
	if err := row.Scan(
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
	chainID, ok := ctx.Value(ChainIDContextKey).(string)
	if !ok {
		return nil, common.ErrBadChainID
	}

	qb := NewQueryBuilder(fmt.Sprintf(`
			SELECT voter, vote
				FROM %s.votes
				WHERE proposal = $1::bigint`,
		chainID), c.db)

	pagination, err := common.NewPagination(r)
	if err != nil {
		c.logger.Info("pagination failed",
			"request_id", ctx.Value(RequestIDContextKey),
			"err", err.Error(),
		)
		return nil, common.ErrBadRequest
	}
	qb.AddPagination(ctx, pagination)

	id, err := strconv.ParseUint(chi.URLParam(r, "proposal_id"), 10, 64)
	if err != nil {
		return nil, common.ErrBadRequest
	}

	rows, err := c.db.Query(ctx, qb.String(), id)
	if err != nil {
		c.logger.Info("query failed",
			"request_id", ctx.Value(RequestIDContextKey),
			"err", err.Error(),
		)
		return nil, common.ErrStorageError
	}
	defer rows.Close()

	var vs ProposalVotes
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
