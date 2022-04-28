package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi"
	"github.com/iancoleman/strcase"
	oasisErrors "github.com/oasisprotocol/oasis-core/go/common/errors"
)

// Status is the API response for GetStatus.
type Status struct {
	LatestChainID string    `json:"latest_chain_id"`
	LatestBlock   int64     `json:"latest_block"`
	LatestUpdate  time.Time `json:"latest_update"`
}

// GetStatus gets the indexer status.
func (h *Handler) GetStatus(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	row, err := h.db.QueryRow(
		ctx,
		fmt.Sprintf(`SELECT height, processed_time
			FROM %s.processed_blocks
			ORDER BY processed_time DESC
			LIMIT 1`, strcase.ToSnake(LatestChainID)),
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s := Status{
		LatestChainID: LatestChainID,
	}
	if err := row.Scan(&s.LatestBlock, &s.LatestUpdate); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	resp, err := json.Marshal(s)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("content-type", "application/json")
	w.Write(resp)
}

// BlockList is the API response for ListBlocks.
type BlockList struct {
	Blocks []Block `json:"blocks"`
}

// ListBlocks gets a list of consensus blocks.
func (h *Handler) ListBlocks(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	chainID, ok := ctx.Value(ChainIDContextKey).(string)
	if !ok {
		http.Error(w, "unable to resolve chain ID", http.StatusBadRequest)
	}

	query :=
		fmt.Sprintf(`SELECT height, block_hash, time
			FROM %s.blocks`, chainID)

	var filters []string
	params := r.URL.Query()
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
	if len(filters) > 0 {
		query = fmt.Sprintf("%s WHERE %s", query, strings.Join(filters, " AND "))
	}
	pagination, err := unpackPagination(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	query = withPagination(query, pagination)

	rows, err := h.db.Query(ctx, query)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var bs BlockList
	for rows.Next() {
		var b Block
		if err := rows.Scan(&b.Height, &b.Hash, &b.Timestamp); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		bs.Blocks = append(bs.Blocks, b)
	}

	resp, err := json.Marshal(bs)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("content-type", "application/json")
	w.Write(resp)
}

// Block is the API response for GetBlock.
type Block struct {
	Height    int64     `json:"height"`
	Hash      string    `json:"hash"`
	Timestamp time.Time `json:"timestamp"`
}

// GetBlock gets a consensus block.
func (h *Handler) GetBlock(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	chainID, ok := ctx.Value(ChainIDContextKey).(string)
	if !ok {
		http.Error(w, "unable to resolve chain ID", http.StatusBadRequest)
	}

	row, err := h.db.QueryRow(
		ctx,
		fmt.Sprintf(`SELECT height, block_hash, time
			FROM %s.blocks
			WHERE height = $1::bigint`, chainID),
		chi.URLParam(r, "height"),
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var b Block
	if err := row.Scan(&b.Height, &b.Hash, &b.Timestamp); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	resp, err := json.Marshal(b)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("content-type", "application/json")
	w.Write(resp)
}

// TransactionList is the API response for ListTransactions.
type TransactionList struct {
	Transactions []Transaction `json:"transactions"`
}

// ListTransactions gets a list of consensus transactions.
func (h *Handler) ListTransactions(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	chainID, ok := ctx.Value(ChainIDContextKey).(string)
	if !ok {
		http.Error(w, "unable to resolve chain ID", http.StatusBadRequest)
	}

	query :=
		fmt.Sprintf(`SELECT block, txn_hash, nonce, fee_amount, method, body, code
			FROM %s.transactions`, chainID)

	var filters []string
	params := r.URL.Query()
	for param, condition := range map[string]string{
		"block":  "block = %s",
		"method": "method = %s",
		"minFee": "fee_amount >= %s",
		"maxFee": "fee_amount <= %s",
		"code":   "code = %s",
	} {
		if v := params.Get(param); v != "" {
			filters = append(filters, fmt.Sprintf(condition, v))
		}
	}
	if len(filters) > 0 {
		query = fmt.Sprintf("%s WHERE %s", query, strings.Join(filters, " AND "))
	}
	pagination, err := unpackPagination(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	query = withPagination(query, pagination)

	rows, err := h.db.Query(ctx, query)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
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
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if code == oasisErrors.CodeNoError {
			t.Success = true
		}

		ts.Transactions = append(ts.Transactions, t)
	}

	resp, err := json.Marshal(ts)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("content-type", "application/json")
	w.Write(resp)
}

// Transaction is the API response for GetTransaction.
type Transaction struct {
	Height  uint64 `json:"height"`
	Hash    string `json:"hash"`
	Nonce   uint64 `json:"nonce"`
	Fee     uint64 `json:"fee"`
	Method  string `json:"method"`
	Body    []byte `json:"body"`
	Success bool   `json:"success"`
}

// GetTransaction gets a consensus transaction.
func (h *Handler) GetTransaction(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	chainID, ok := ctx.Value(ChainIDContextKey).(string)
	if !ok {
		http.Error(w, "unable to resolve chain ID", http.StatusBadRequest)
	}

	query :=
		fmt.Sprintf(`SELECT block, txn_hash, nonce, fee_amount, method, body, code
			FROM %s.transactions
			WHERE txn_hash = $1::hash`, chainID)

	row, err := h.db.QueryRow(
		ctx,
		query,
		chi.URLParam(r, "txn_hash"),
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
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
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if code == oasisErrors.CodeNoError {
		t.Success = true
	}

	resp, err := json.Marshal(t)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("content-type", "application/json")
	w.Write(resp)
}

// EntityList is the API response for ListEntitys.
type EntityList struct {
	Entities []Entity `json:"entities"`
}

// ListEntities gets a list of registered entities.
func (h *Handler) ListEntities(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	chainID, ok := ctx.Value(ChainIDContextKey).(string)
	if !ok {
		http.Error(w, "unable to resolve chain ID", http.StatusBadRequest)
	}

	query := fmt.Sprintf(`SELECT id, address FROM %s.entities`, chainID)
	pagination, err := unpackPagination(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	query = withPagination(query, pagination)

	rows, err := h.db.Query(ctx, query)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var es EntityList
	for rows.Next() {
		var e Entity
		if err := rows.Scan(&e.ID, &e.Address); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		es.Entities = append(es.Entities, e)
	}

	resp, err := json.Marshal(es)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("content-type", "application/json")
	w.Write(resp)
}

// Entity is the API response for GetEntity.
type Entity struct {
	ID      string `json:"id"`
	Address string `json:"address,omitempty"`

	Nodes []string `json:"nodes,omitempty"`
}

// GetEntity gets a registered entity.
func (h *Handler) GetEntity(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	chainID, ok := ctx.Value(ChainIDContextKey).(string)
	if !ok {
		http.Error(w, "unable to resolve chain ID", http.StatusBadRequest)
	}

	entityRow, err := h.db.QueryRow(
		ctx,
		fmt.Sprintf(`SELECT id, address
			FROM %s.entities
			WHERE id = $1::text`, chainID),
		chi.URLParam(r, "entity_id"),
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var e Entity
	if err := entityRow.Scan(&e.ID, &e.Address); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	nodeRows, err := h.db.Query(
		ctx,
		fmt.Sprintf(`SELECT id
			FROM %s.nodes
			WHERE entity_id = $1::text`, chainID),
		chi.URLParam(r, "entity_id"),
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer nodeRows.Close()

	for nodeRows.Next() {
		var nid string
		if err := nodeRows.Scan(&nid); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		e.Nodes = append(e.Nodes, nid)
	}

	resp, err := json.Marshal(e)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("content-type", "application/json")
	w.Write(resp)
}

// NodeList is the API response for ListEntityNodes.
type NodeList struct {
	EntityID string `json:"entity_id"`
	Nodes    []Node `json:"nodes"`
}

// ListEntityNodes gets a list of nodes controlled by the provided entity.
func (h *Handler) ListEntityNodes(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	chainID, ok := ctx.Value(ChainIDContextKey).(string)
	if !ok {
		http.Error(w, "unable to resolve chain ID", http.StatusBadRequest)
	}

	query :=
		fmt.Sprintf(`SELECT id, entity_id, expiration, tls_pubkey, tls_next_pubkey, p2p_pubkey, consensus_pubkey, roles
			FROM %s.nodes
			WHERE entity_id = $1::text`, chainID)

	pagination, err := unpackPagination(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	query = withPagination(query, pagination)

	id := chi.URLParam(r, "entity_id")
	rows, err := h.db.Query(ctx, query, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
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
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		ns.Nodes = append(ns.Nodes, n)
	}
	ns.EntityID = id

	resp, err := json.Marshal(ns)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("content-type", "application/json")
	w.Write(resp)
}

// Node is the API response for GetEntityNode.
type Node struct {
	ID              string `json:"id"`
	EntityID        string `json:"entity_id"`
	Expiration      uint64 `json:"expiration"`
	TLSPubkey       string `json:"tls_pubkey"`
	TLSNextPubkey   string `json:"tls_next_pubkey,omitempty"`
	P2PPubkey       string `json:"p2p_pubkey"`
	ConsensusPubkey string `json:"consensus_pubkey"`
	Roles           string `json:"roles"`
}

// GetEntityNode gets a node controlled by the provided entity.
func (h *Handler) GetEntityNode(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	chainID, ok := ctx.Value(ChainIDContextKey).(string)
	if !ok {
		http.Error(w, "unable to resolve chain ID", http.StatusBadRequest)
	}

	row, err := h.db.QueryRow(
		ctx,
		fmt.Sprintf(`SELECT id, entity_id, expiration, tls_pubkey, tls_next_pubkey, p2p_pubkey, consensus_pubkey, roles
			FROM %s.nodes
			WHERE entity_id = $1::text AND id = $2::text`, chainID),
		chi.URLParam(r, "entity_id"),
		chi.URLParam(r, "node_id"),
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
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
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	resp, err := json.Marshal(n)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("content-type", "application/json")
	w.Write(resp)
}

// AccountList is the API response for ListAccounts.
type AccountList struct {
	Accounts []Account `json:"accounts"`
}

// ListAccounts gets a list of consensus accounts.
func (h *Handler) ListAccounts(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	chainID, ok := ctx.Value(ChainIDContextKey).(string)
	if !ok {
		http.Error(w, "unable to resolve chain ID", http.StatusBadRequest)
	}

	query :=
		fmt.Sprintf(`SELECT address, nonce, general_balance, escrow_balance_active, escrow_balance_debonding
				FROM %s.accounts`, chainID)

	var filters []string
	params := r.URL.Query()
	for param, condition := range map[string]string{
		"minBalance":   "general_balance >= %s",
		"maxBalance":   "general_balance <= %s",
		"minEscrow":    "escrow_balance_active >= %s",
		"maxEscrow":    "escrow_balance_active <= %s",
		"minDebonding": "escrow_balance_debonding >= %s",
		"maxDebonding": "escrow_balance_debonding <= %s",
	} {
		if v := params.Get(param); v != "" {
			filters = append(filters, fmt.Sprintf(condition, v))
		}
	}
	if len(filters) > 0 {
		query = fmt.Sprintf("%s WHERE %s", query, strings.Join(filters, " AND "))
	}

	pagination, err := unpackPagination(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	query = withPagination(query, pagination)

	rows, err := h.db.Query(ctx, query)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
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
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		a.Total = a.Available + a.Escrow + a.Debonding

		as.Accounts = append(as.Accounts, a)
	}

	resp, err := json.Marshal(as)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("content-type", "application/json")
	w.Write(resp)
}

// Account is the API response for GetAccount.
type Account struct {
	Address   string `json:"address"`
	Nonce     uint64 `json:"nonce"`
	Available uint64 `json:"available"`
	Escrow    uint64 `json:"escrow"`
	Debonding uint64 `json:"debonding"`
	Total     uint64 `json:"total"`

	Allowances []Allowance `json:"allowances,omitempty"`
}

type Allowance struct {
	Address string `json:"address"`
	Amount  uint64 `json:"amount"`
}

// GetAccount gets a consensus account.
func (h *Handler) GetAccount(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	chainID, ok := ctx.Value(ChainIDContextKey).(string)
	if !ok {
		http.Error(w, "unable to resolve chain ID", http.StatusBadRequest)
	}

	accountRow, err := h.db.QueryRow(
		ctx,
		fmt.Sprintf(`SELECT address, nonce, general_balance, escrow_balance_active, escrow_balance_debonding
			FROM %s.accounts
			WHERE address = $1::text`, chainID),
		chi.URLParam(r, "address"),
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var a Account
	if err := accountRow.Scan(
		&a.Address,
		&a.Nonce,
		&a.Available,
		&a.Escrow,
		&a.Debonding,
	); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	a.Total = a.Available + a.Escrow + a.Debonding

	allowanceRows, err := h.db.Query(
		ctx,
		fmt.Sprintf(`SELECT beneficiary, allowance
			FROM %s.allowances
			WHERE owner = $1::text`, chainID),
		chi.URLParam(r, "address"),
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer allowanceRows.Close()

	for allowanceRows.Next() {
		var al Allowance
		if err := allowanceRows.Scan(
			&al.Address,
			&al.Amount,
		); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		a.Allowances = append(a.Allowances, al)
	}

	resp, err := json.Marshal(a)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("content-type", "application/json")
	w.Write(resp)
}

// ListEpochs gets a list of epochs.
func (h *Handler) ListEpochs(w http.ResponseWriter, r *http.Request) {
	var resp []byte
	resp, err := json.Marshal(Block{})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("content-type", "application/json")
	w.Write(resp)
}

// GetEpoch gets an epoch.
func (h *Handler) GetEpoch(w http.ResponseWriter, r *http.Request) {
	var resp []byte
	resp, err := json.Marshal(Block{})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("content-type", "application/json")
	w.Write(resp)
}

// ProposalList is the API response for ListProposals.
type ProposalList struct {
	Proposals []Proposal `json:"proposals"`
}

// ListProposals gets a list of governance proposals.
func (h *Handler) ListProposals(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	chainID, ok := ctx.Value(ChainIDContextKey).(string)
	if !ok {
		http.Error(w, "unable to resolve chain ID", http.StatusBadRequest)
	}

	query :=
		fmt.Sprintf(`SELECT id, submitter, state, deposit, handler, cp_target_version, rhp_target_version, rcp_target_version,
				upgrade_epoch, cancels, created_at, closes_at, invalid_votes
			FROM %s.proposals`, chainID)

	var filters []string
	params := r.URL.Query()
	for param, condition := range map[string]string{
		"submitter": "submitter = %s",
		"state":     "state = %s",
	} {
		if v := params.Get(param); v != "" {
			filters = append(filters, fmt.Sprintf(condition, v))
		}
	}
	if len(filters) > 0 {
		query = fmt.Sprintf("%s WHERE %s", query, strings.Join(filters, " AND "))
	}

	pagination, err := unpackPagination(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	query = withPagination(query, pagination)

	rows, err := h.db.Query(ctx, query)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
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
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		ps.Proposals = append(ps.Proposals, p)
	}

	resp, err := json.Marshal(ps)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("content-type", "application/json")
	w.Write(resp)
}

// Proposal is the API response for GetProposal.
type Proposal struct {
	ID           uint64  `json:"id"`
	Submitter    string  `json:"submitter"`
	State        string  `json:"state"`
	Deposit      uint64  `json:"deposit"`
	Handler      *string `json:"handler,omitempty"`
	Target       Target  `json:"target,omitempty"`
	Epoch        *uint64 `json:"epoch,omitempty"`
	Cancels      *int64  `json:"cancels,omitempty"`
	CreatedAt    uint64  `json:"created_at"`
	ClosesAt     uint64  `json:"closes_at"`
	InvalidVotes uint64  `json:"invalid_votes"`
}

type Target struct {
	ConsensusProtocol        *string `json:"consensus_protocol"`
	RuntimeHostProtocol      *string `json:"runtime_host_protocol"`
	RuntimeCommitteeProtocol *string `json:"runtime_committee_protocol"`
}

// GetProposal gets a governance proposal.
func (h *Handler) GetProposal(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	chainID, ok := ctx.Value(ChainIDContextKey).(string)
	if !ok {
		http.Error(w, "unable to resolve chain ID", http.StatusBadRequest)
	}

	row, err := h.db.QueryRow(
		ctx,
		fmt.Sprintf(`SELECT id, submitter, state, deposit, handler, cp_target_version, rhp_target_version, rcp_target_version,
						upgrade_epoch, cancels, created_at, closes_at, invalid_votes
			FROM %s.proposals
			WHERE id = $1::bigint`, chainID),
		chi.URLParam(r, "proposal_id"),
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
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
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	resp, err := json.Marshal(p)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("content-type", "application/json")
	w.Write(resp)
}

// ProposalVotes is the API response for GetProposalVotes.
type ProposalVotes struct {
	ProposalID uint64         `json:"proposal_id"`
	Votes      []ProposalVote `json:"votes"`
}

type ProposalVote struct {
	Address string `json:"address"`
	Vote    string `json:"vote"`
}

// GetProposalVotes gets votes for a governance proposal.
func (h *Handler) GetProposalVotes(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	chainID, ok := ctx.Value(ChainIDContextKey).(string)
	if !ok {
		http.Error(w, "unable to resolve chain ID", http.StatusBadRequest)
	}

	query :=
		fmt.Sprintf(`SELECT voter, vote
			FROM %s.votes
			WHERE proposal = $1::bigint`, chainID)

	pagination, err := unpackPagination(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	query = withPagination(query, pagination)

	id, err := strconv.ParseUint(chi.URLParam(r, "proposal_id"), 10, 64)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	rows, err := h.db.Query(ctx, query, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var p ProposalVotes
	for rows.Next() {
		var v ProposalVote
		if err := rows.Scan(
			&v.Address,
			&v.Vote,
		); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		p.Votes = append(p.Votes, v)
	}
	p.ProposalID = id

	resp, err := json.Marshal(p)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("content-type", "application/json")
	w.Write(resp)
}
