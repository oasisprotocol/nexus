package v1

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/oasislabs/oasis-indexer/api/common"
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

	resp, err := json.Marshal(status)
	if err != nil {
		h.logAndReply(ctx, "failed to marshal status", w, err)
		h.metrics.RequestCounter(r.URL.Path, "failure", "serde_error").Inc()
		return
	}

	w.Header().Set("content-type", "application/json")
	if _, err := w.Write(resp); err != nil {
		h.logger.Error("failed to write response",
			"request_id", ctx.Value(RequestIDContextKey),
			"error", err,
		)
		h.metrics.RequestCounter(r.URL.Path, "failure", "http_error").Inc()
	} else {
		h.metrics.RequestCounter(r.URL.Path, "success").Inc()
	}
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

	resp, err := json.Marshal(blocks)
	if err != nil {
		h.logAndReply(ctx, "failed to marshal blocks", w, err)
		h.metrics.RequestCounter(r.URL.Path, "failure", "serde_error").Inc()
		return
	}

	w.Header().Set("content-type", "application/json")
	if _, err := w.Write(resp); err != nil {
		h.logger.Error("failed to write response",
			"request_id", ctx.Value(RequestIDContextKey),
			"error", err,
		)
		h.metrics.RequestCounter(r.URL.Path, "failure", "http_error").Inc()
	} else {
		h.metrics.RequestCounter(r.URL.Path, "success").Inc()
	}
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

	resp, err := json.Marshal(block)
	if err != nil {
		h.logAndReply(ctx, "failed to marshal block", w, err)
		h.metrics.RequestCounter(r.URL.Path, "failure", "serde_error").Inc()
		return
	}

	w.Header().Set("content-type", "application/json")
	if _, err := w.Write(resp); err != nil {
		h.logger.Error("failed to write response",
			"request_id", ctx.Value(RequestIDContextKey),
			"error", err,
		)
		h.metrics.RequestCounter(r.URL.Path, "failure", "http_error").Inc()
	} else {
		h.metrics.RequestCounter(r.URL.Path, "success").Inc()
	}
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

	resp, err := json.Marshal(transactions)
	if err != nil {
		h.logAndReply(ctx, "failed to marshal transactions", w, err)
		h.metrics.RequestCounter(r.URL.Path, "failure", "serde_error").Inc()
		return
	}

	w.Header().Set("content-type", "application/json")
	if _, err := w.Write(resp); err != nil {
		h.logger.Error("failed to write response",
			"request_id", ctx.Value(RequestIDContextKey),
			"error", err,
		)
		h.metrics.RequestCounter(r.URL.Path, "failure", "http_error").Inc()
	} else {
		h.metrics.RequestCounter(r.URL.Path, "success").Inc()
	}
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

	resp, err := json.Marshal(transaction)
	if err != nil {
		h.logAndReply(ctx, "failed to marshal transaction", w, err)
		h.metrics.RequestCounter(r.URL.Path, "failure", "serde_error").Inc()
		return
	}

	w.Header().Set("content-type", "application/json")
	if _, err := w.Write(resp); err != nil {
		h.logger.Error("failed to write response",
			"request_id", ctx.Value(RequestIDContextKey),
			"error", err,
		)
		h.metrics.RequestCounter(r.URL.Path, "failure", "http_error").Inc()
	} else {
		h.metrics.RequestCounter(r.URL.Path, "success").Inc()
	}
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

	resp, err := json.Marshal(entities)
	if err != nil {
		h.logAndReply(ctx, "failed to marshal entities", w, err)
		h.metrics.RequestCounter(r.URL.Path, "failure", "serde_error").Inc()
		return
	}

	w.Header().Set("content-type", "application/json")
	if _, err := w.Write(resp); err != nil {
		h.logger.Error("failed to write response",
			"request_id", ctx.Value(RequestIDContextKey),
			"error", err,
		)
		h.metrics.RequestCounter(r.URL.Path, "failure", "http_error").Inc()
	} else {
		h.metrics.RequestCounter(r.URL.Path, "success").Inc()
	}
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

	resp, err := json.Marshal(entity)
	if err != nil {
		h.logAndReply(ctx, "failed to marshal entity", w, err)
		h.metrics.RequestCounter(r.URL.Path, "failure", "serde_error").Inc()
		return
	}

	w.Header().Set("content-type", "application/json")
	if _, err := w.Write(resp); err != nil {
		h.logger.Error("failed to write response",
			"request_id", ctx.Value(RequestIDContextKey),
			"error", err,
		)
		h.metrics.RequestCounter(r.URL.Path, "failure", "http_error").Inc()
	} else {
		h.metrics.RequestCounter(r.URL.Path, "success").Inc()
	}
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

	resp, err := json.Marshal(nodes)
	if err != nil {
		h.logAndReply(ctx, "failed to marshal entity nodes", w, err)
		h.metrics.RequestCounter(r.URL.Path, "failure", "serde_error").Inc()
		return
	}

	w.Header().Set("content-type", "application/json")
	if _, err := w.Write(resp); err != nil {
		h.logger.Error("failed to write response",
			"request_id", ctx.Value(RequestIDContextKey),
			"error", err,
		)
		h.metrics.RequestCounter(r.URL.Path, "failure", "http_error").Inc()
	} else {
		h.metrics.RequestCounter(r.URL.Path, "success").Inc()
	}
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

	resp, err := json.Marshal(node)
	if err != nil {
		h.logAndReply(ctx, "failed to marshal entity node", w, err)
		h.metrics.RequestCounter(r.URL.Path, "failure", "serde_error").Inc()
		return
	}

	w.Header().Set("content-type", "application/json")
	if _, err := w.Write(resp); err != nil {
		h.logger.Error("failed to write response",
			"request_id", ctx.Value(RequestIDContextKey),
			"error", err,
		)
		h.metrics.RequestCounter(r.URL.Path, "failure", "http_error").Inc()
	} else {
		h.metrics.RequestCounter(r.URL.Path, "success").Inc()
	}
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

	resp, err := json.Marshal(accounts)
	if err != nil {
		h.logAndReply(ctx, "failed to marshal accounts", w, err)
		h.metrics.RequestCounter(r.URL.Path, "failure", "serde_error").Inc()
		return
	}

	w.Header().Set("content-type", "application/json")
	if _, err := w.Write(resp); err != nil {
		h.logger.Error("failed to write response",
			"request_id", ctx.Value(RequestIDContextKey),
			"error", err,
		)
		h.metrics.RequestCounter(r.URL.Path, "failure", "http_error").Inc()
	} else {
		h.metrics.RequestCounter(r.URL.Path, "success").Inc()
	}
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

	resp, err := json.Marshal(account)
	if err != nil {
		h.logAndReply(ctx, "failed to marshal account", w, err)
		h.metrics.RequestCounter(r.URL.Path, "failure", "serde_error").Inc()
		return
	}

	w.Header().Set("content-type", "application/json")
	if _, err := w.Write(resp); err != nil {
		h.logger.Error("failed to write response",
			"request_id", ctx.Value(RequestIDContextKey),
			"error", err,
		)
		h.metrics.RequestCounter(r.URL.Path, "failure", "http_error").Inc()
	} else {
		h.metrics.RequestCounter(r.URL.Path, "success").Inc()
	}
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

	var resp []byte
	resp, err = json.Marshal(epochs)
	if err != nil {
		h.logAndReply(ctx, "failed to marshal epochs", w, err)
		h.metrics.RequestCounter(r.URL.Path, "failure", "database_error").Inc()
		return
	}

	w.Header().Set("content-type", "application/json")
	if _, err := w.Write(resp); err != nil {
		h.logger.Error("failed to write response",
			"request_id", ctx.Value(RequestIDContextKey),
			"error", err,
		)
		h.metrics.RequestCounter(r.URL.Path, "failure", "http_error").Inc()
	} else {
		h.metrics.RequestCounter(r.URL.Path, "success").Inc()
	}
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

	var resp []byte
	resp, err = json.Marshal(epoch)
	if err != nil {
		h.logAndReply(ctx, "failed to marshal epoch", w, err)
		h.metrics.RequestCounter(r.URL.Path, "failure", "database_error").Inc()
		return
	}

	w.Header().Set("content-type", "application/json")
	if _, err := w.Write(resp); err != nil {
		h.logger.Error("failed to write response",
			"request_id", ctx.Value(RequestIDContextKey),
			"error", err,
		)
		h.metrics.RequestCounter(r.URL.Path, "failure", "http_error").Inc()
	} else {
		h.metrics.RequestCounter(r.URL.Path, "success").Inc()
	}
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

	resp, err := json.Marshal(proposals)
	if err != nil {
		h.logAndReply(ctx, "failed to marshal proposals", w, err)
		h.metrics.RequestCounter(r.URL.Path, "failure", "serde_error").Inc()
		return
	}

	w.Header().Set("content-type", "application/json")
	if _, err := w.Write(resp); err != nil {
		h.logger.Error("failed to write response",
			"request_id", ctx.Value(RequestIDContextKey),
			"error", err,
		)
		h.metrics.RequestCounter(r.URL.Path, "failure", "http_error").Inc()
	} else {
		h.metrics.RequestCounter(r.URL.Path, "success").Inc()
	}
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

	resp, err := json.Marshal(proposal)
	if err != nil {
		h.logAndReply(ctx, "failed to marshal proposal", w, err)
		h.metrics.RequestCounter(r.URL.Path, "failure", "serde_error").Inc()
		return
	}

	w.Header().Set("content-type", "application/json")
	if _, err := w.Write(resp); err != nil {
		h.logger.Error("failed to write response",
			"request_id", ctx.Value(RequestIDContextKey),
			"error", err,
		)
		h.metrics.RequestCounter(r.URL.Path, "failure", "http_error").Inc()
	} else {
		h.metrics.RequestCounter(r.URL.Path, "success").Inc()
	}
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

	resp, err := json.Marshal(votes)
	if err != nil {
		h.logAndReply(ctx, "failed to marshal proposal votes", w, err)
		h.metrics.RequestCounter(r.URL.Path, "failure", "serde_error").Inc()
		return
	}

	w.Header().Set("content-type", "application/json")
	if _, err := w.Write(resp); err != nil {
		h.logger.Error("failed to write response",
			"request_id", ctx.Value(RequestIDContextKey),
			"error", err,
		)
		h.metrics.RequestCounter(r.URL.Path, "failure", "http_error").Inc()
	} else {
		h.metrics.RequestCounter(r.URL.Path, "success").Inc()
	}
}

func (h *Handler) logAndReply(ctx context.Context, msg string, w http.ResponseWriter, err error) {
	h.logger.Error(msg,
		"request_id", ctx.Value(RequestIDContextKey),
		"error", err,
	)
	if err = common.ReplyWithError(w, err); err != nil {
		h.logger.Error("failed to reply with error",
			"request_id", ctx.Value(RequestIDContextKey),
			"error", err,
		)
	}
}
