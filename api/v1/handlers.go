package v1

import (
	"context"
	"encoding/json"
	"net/http"

	apiCommon "github.com/oasisprotocol/oasis-indexer/api/common"
	common "github.com/oasisprotocol/oasis-indexer/common"
)

// GetStatus gets the indexer status.
func (h *Handler) GetStatus(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	status, err := h.client.Status(ctx)
	if err != nil {
		h.logAndReply(ctx, "failed to get status", w, err)
		h.metrics.RequestCounter(r.URL.Path, "failure", "database_error").Inc()
		return
	}

	h.replyJSON(ctx, w, r.URL.Path, status)
}

// ListBlocks gets a list of consensus blocks.
func (h *Handler) ListBlocks(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	blocks, err := h.client.Blocks(ctx, r)
	if err != nil {
		h.logAndReply(ctx, "failed to list blocks", w, err)
		h.metrics.RequestCounter(r.URL.Path, "failure", "database_error").Inc()
		return
	}

	h.replyJSON(ctx, w, r.URL.Path, blocks)
}

// GetBlock gets a consensus block.
func (h *Handler) GetBlock(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	block, err := h.client.Block(ctx, r)
	if err != nil {
		h.logAndReply(ctx, "failed to get block", w, err)
		h.metrics.RequestCounter(r.URL.Path, "failure", "database_error").Inc()
		return
	}

	h.replyJSON(ctx, w, r.URL.Path, block)
}

// ListTransactions gets a list of consensus transactions.
func (h *Handler) ListTransactions(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	transactions, err := h.client.Transactions(ctx, r)
	if err != nil {
		h.logAndReply(ctx, "failed to list transactions", w, err)
		h.metrics.RequestCounter(r.URL.Path, "failure", "database_error").Inc()
		return
	}

	h.replyJSON(ctx, w, r.URL.Path, transactions)
}

// GetTransaction gets a consensus transaction.
func (h *Handler) GetTransaction(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	transaction, err := h.client.Transaction(ctx, r)
	if err != nil {
		h.logAndReply(ctx, "failed to get transaction", w, err)
		h.metrics.RequestCounter(r.URL.Path, "failure", "database_error").Inc()
		return
	}

	h.replyJSON(ctx, w, r.URL.Path, transaction)
}

// ListEvents gets a list of events.
func (h *Handler) ListEvents(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	events, err := h.client.Events(ctx, r)
	if err != nil {
		h.logAndReply(ctx, "failed to list events", w, err)
		h.metrics.RequestCounter(r.URL.Path, "failure", "database_error").Inc()
		return
	}

	h.replyJSON(ctx, w, r.URL.Path, events)
}

// ListEntities gets a list of registered entities.
func (h *Handler) ListEntities(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	entities, err := h.client.Entities(ctx, r)
	if err != nil {
		h.logAndReply(ctx, "failed to list entities", w, err)
		h.metrics.RequestCounter(r.URL.Path, "failure", "database_error").Inc()
		return
	}

	h.replyJSON(ctx, w, r.URL.Path, entities)
}

// GetEntity gets a registered entity.
func (h *Handler) GetEntity(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	entity, err := h.client.Entity(ctx, r)
	if err != nil {
		h.logAndReply(ctx, "failed to get entity", w, err)
		h.metrics.RequestCounter(r.URL.Path, "failure", "database_error").Inc()
		return
	}

	h.replyJSON(ctx, w, r.URL.Path, entity)
}

// ListEntityNodes gets a list of nodes controlled by the provided entity.
func (h *Handler) ListEntityNodes(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	nodes, err := h.client.EntityNodes(ctx, r)
	if err != nil {
		h.logAndReply(ctx, "failed to list entity nodes", w, err)
		h.metrics.RequestCounter(r.URL.Path, "failure", "database_error").Inc()
		return
	}

	h.replyJSON(ctx, w, r.URL.Path, nodes)
}

// GetEntityNode gets a node controlled by the provided entity.
func (h *Handler) GetEntityNode(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	node, err := h.client.EntityNode(ctx, r)
	if err != nil {
		h.logAndReply(ctx, "failed to get entity node", w, err)
		h.metrics.RequestCounter(r.URL.Path, "failure", "database_error").Inc()
		return
	}

	h.replyJSON(ctx, w, r.URL.Path, node)
}

// ListAccounts gets a list of consensus accounts.
func (h *Handler) ListAccounts(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	accounts, err := h.client.Accounts(ctx, r)
	if err != nil {
		h.logAndReply(ctx, "failed to list accounts", w, err)
		h.metrics.RequestCounter(r.URL.Path, "failure", "database_error").Inc()
		return
	}

	h.replyJSON(ctx, w, r.URL.Path, accounts)
}

// GetAccount gets a consensus account.
func (h *Handler) GetAccount(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	account, err := h.client.Account(ctx, r)
	if err != nil {
		h.logAndReply(ctx, "failed to get account", w, err)
		h.metrics.RequestCounter(r.URL.Path, "failure", "database_error").Inc()
		return
	}

	h.replyJSON(ctx, w, r.URL.Path, account)
}

// GetDelegations gets an account's delegations.
func (h *Handler) GetDelegations(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	delegations, err := h.client.Delegations(ctx, r)
	if err != nil {
		h.logAndReply(ctx, "failed to get delegations", w, err)
		h.metrics.RequestCounter(r.URL.Path, "failure", "database_error").Inc()
		return
	}

	h.replyJSON(ctx, w, r.URL.Path, delegations)
}

// GetDebondingDelegations gets an account's debonding delegations.
func (h *Handler) GetDebondingDelegations(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	debondingDelegations, err := h.client.DebondingDelegations(ctx, r)
	if err != nil {
		h.logAndReply(ctx, "failed to get debonding delegations", w, err)
		h.metrics.RequestCounter(r.URL.Path, "failure", "database_error").Inc()
		return
	}

	h.replyJSON(ctx, w, r.URL.Path, debondingDelegations)
}

// ListEpochs gets a list of epochs.
func (h *Handler) ListEpochs(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	epochs, err := h.client.Epochs(ctx, r)
	if err != nil {
		h.logAndReply(ctx, "failed to list epochs", w, err)
		h.metrics.RequestCounter(r.URL.Path, "failure", "database_error").Inc()
		return
	}

	h.replyJSON(ctx, w, r.URL.Path, epochs)
}

// GetEpoch gets an epoch.
func (h *Handler) GetEpoch(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	epoch, err := h.client.Epoch(ctx, r)
	if err != nil {
		h.logAndReply(ctx, "failed to get epoch", w, err)
		h.metrics.RequestCounter(r.URL.Path, "failure", "database_error").Inc()
		return
	}

	h.replyJSON(ctx, w, r.URL.Path, epoch)
}

// ListProposals gets a list of governance proposals.
func (h *Handler) ListProposals(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	proposals, err := h.client.Proposals(ctx, r)
	if err != nil {
		h.logAndReply(ctx, "failed to list proposals", w, err)
		h.metrics.RequestCounter(r.URL.Path, "failure", "database_error").Inc()
		return
	}

	h.replyJSON(ctx, w, r.URL.Path, proposals)
}

// GetProposal gets a governance proposal.
func (h *Handler) GetProposal(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	proposal, err := h.client.Proposal(ctx, r)
	if err != nil {
		h.logAndReply(ctx, "failed to get proposal", w, err)
		h.metrics.RequestCounter(r.URL.Path, "failure", "database_error").Inc()
		return
	}

	h.replyJSON(ctx, w, r.URL.Path, proposal)
}

// GetProposalVotes gets votes for a governance proposal.
func (h *Handler) GetProposalVotes(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	votes, err := h.client.ProposalVotes(ctx, r)
	if err != nil {
		h.logAndReply(ctx, "failed to get proposal votes", w, err)
		h.metrics.RequestCounter(r.URL.Path, "failure", "database_error").Inc()
		return
	}

	h.replyJSON(ctx, w, r.URL.Path, votes)
}

// GetValidator gets a validator.
func (h *Handler) GetValidator(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	validator, err := h.client.Validator(ctx, r)
	if err != nil {
		h.logAndReply(ctx, "failed to get validator", w, err)
		h.metrics.RequestCounter(r.URL.Path, "failure", "database_error").Inc()
		return
	}

	h.replyJSON(ctx, w, r.URL.Path, validator)
}

// ListValidators gets a list of validators.
func (h *Handler) ListValidators(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	validators, err := h.client.Validators(ctx, r)
	if err != nil {
		h.logAndReply(ctx, "failed to list validators", w, err)
		h.metrics.RequestCounter(r.URL.Path, "failure", "database_error").Inc()
		return
	}

	h.replyJSON(ctx, w, r.URL.Path, validators)
}

func (h *Handler) RuntimeListBlocks(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	blocks, err := h.client.RuntimeBlocks(ctx, r)
	if err != nil {
		h.logAndReply(ctx, "failed to list blocks", w, err)
		h.metrics.RequestCounter(r.URL.Path, "failure", "database_error").Inc()
		return
	}

	h.replyJSON(ctx, w, r.URL.Path, blocks)
}

// RuntimeListTransactions gets a list of runtime transactions.
func (h *Handler) RuntimeListTransactions(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	transactions, err := h.client.RuntimeTransactions(ctx, r)
	if err != nil {
		h.logAndReply(ctx, "failed to list runtime transactions", w, err)
		h.metrics.RequestCounter(r.URL.Path, "failure", "database_error").Inc()
		return
	}

	h.replyJSON(ctx, w, r.URL.Path, transactions)
}

func (h *Handler) RuntimeListTokens(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	tokens, err := h.client.RuntimeTokens(ctx, r)
	if err != nil {
		h.logAndReply(ctx, "failed to list runtime tokens", w, err)
		h.metrics.RequestCounter(r.URL.Path, "failure", "database_error").Inc()
		return
	}

	h.replyJSON(ctx, w, r.URL.Path, tokens)
}

// ListTxVolumes gets a list of transaction volume per time bucket.
func (h *Handler) ListTxVolumes(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	volumes, err := h.client.TxVolumes(ctx, r)
	if err != nil {
		h.logAndReply(ctx, "failed to list tx volume", w, err)
		h.metrics.RequestCounter(r.URL.Path, "failure", "database_error").Inc()
		return
	}

	h.replyJSON(ctx, w, r.URL.Path, volumes)
}

func (h *Handler) logAndReply(ctx context.Context, msg string, w http.ResponseWriter, err error) {
	h.logger.Error(msg,
		"request_id", ctx.Value(common.RequestIDContextKey),
		"error", err,
	)
	if err = apiCommon.ReplyWithError(w, err); err != nil {
		h.logger.Error("failed to reply with error",
			"request_id", ctx.Value(common.RequestIDContextKey),
			"error", err,
		)
	}
}

// replyJSON serializes a response `res` and writes it to an
// http.ResponseWriter `w`. Any errors go to logger and/or the response. Pass
// the URL path in `endpoint` for labeling in the metrics counters.
func (h *Handler) replyJSON(ctx context.Context, w http.ResponseWriter, endpoint string, res interface{}) {
	resp, err := json.Marshal(res)
	if err != nil {
		h.logAndReply(ctx, "failed to marshal response", w, err)
		h.metrics.RequestCounter(endpoint, "failure", "serde_error").Inc()
		return
	}

	w.Header().Set("content-type", "application/json")
	if _, err = w.Write(resp); err != nil {
		h.logger.Error("failed to write response",
			"request_id", ctx.Value(common.RequestIDContextKey),
			"error", err,
		)
		h.metrics.RequestCounter(endpoint, "failure", "http_error").Inc()
	} else {
		h.metrics.RequestCounter(endpoint, "success").Inc()
	}
}
