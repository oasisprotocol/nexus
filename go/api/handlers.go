package api

import (
	"encoding/json"
	"net/http"
)

// GetBlocks gets a list of consensus blocks.
func (h *Handler) GetBlocks(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "endpoint unimplemented. stay tuned!", http.StatusNotImplemented)
}

// GetBlock gets a consensus block.
func (h *Handler) GetBlock(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "endpoint unimplemented. stay tuned!", http.StatusNotImplemented)
}

// GetBeacon gets the consensus beacon state.
func (h *Handler) GetBeacon(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "endpoint unimplemented. stay tuned!", http.StatusNotImplemented)
}

// GetBlockEvents gets events in a consensus block.
func (h *Handler) GetBlockEvents(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "endpoint unimplemented. stay tuned!", http.StatusNotImplemented)
}

// GetTransactions gets a list of consensus transactions.
func (h *Handler) GetTransactions(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "endpoint unimplemented. stay tuned!", http.StatusNotImplemented)
}

// GetTransaction gets a consensus transaction.
func (h *Handler) GetTransaction(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "endpoint unimplemented. stay tuned!", http.StatusNotImplemented)
}

// GetTransactionEvents gets a list of events in a consensus transaction.
func (h *Handler) GetTransactionEvents(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "endpoint unimplemented. stay tuned!", http.StatusNotImplemented)
}

// GetEntities gets a list of registered entitites.
func (h *Handler) GetEntities(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "endpoint unimplemented. stay tuned!", http.StatusNotImplemented)
}

// GetEntity gets a registered entity.
func (h *Handler) GetEntity(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "endpoint unimplemented. stay tuned!", http.StatusNotImplemented)
}

// GetEntityNodes gets a list of nodes for a registered entity.
func (h *Handler) GetEntityNodes(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "endpoint unimplemented. stay tuned!", http.StatusNotImplemented)
}

// GetEntityNode gets a node for a registered entity.
func (h *Handler) GetEntityNode(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "endpoint unimplemented. stay tuned!", http.StatusNotImplemented)
}

// GetAccounts gets a list of accounts in the consensus ledger.
func (h *Handler) GetAccounts(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "endpoint unimplemented. stay tuned!", http.StatusNotImplemented)
}

// GetAccount gets an account in the consensus ledger.
func (h *Handler) GetAccount(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "endpoint unimplemented. stay tuned!", http.StatusNotImplemented)
}

// GetEpochs gets a list of epochs.
func (h *Handler) GetEpochs(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "endpoint unimplemented. stay tuned!", http.StatusNotImplemented)
}

// GetEpoch gets an epoch.
func (h *Handler) GetEpoch(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "endpoint unimplemented. stay tuned!", http.StatusNotImplemented)
}

// GetValidators gets a list of consensus validators.
func (h *Handler) GetValidators(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "endpoint unimplemented. stay tuned!", http.StatusNotImplemented)
}

// GetCommittees gets a list of runtime committees.
func (h *Handler) GetCommittees(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "endpoint unimplemented. stay tuned!", http.StatusNotImplemented)
}

// GetProposals gets a list of governance proposals.
func (h *Handler) GetProposals(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "endpoint unimplemented. stay tuned!", http.StatusNotImplemented)
}

// GetProposal gets a governance proposal.
func (h *Handler) GetProposal(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "endpoint unimplemented. stay tuned!", http.StatusNotImplemented)
}

// GetStatus gets the latest indexing status of the Oasis Indexer.
func (h *Handler) GetStatus(w http.ResponseWriter, r *http.Request) {
	status := APIStatus{
		CurrentHeight: 8048956,
	}

	var resp []byte
	resp, err := json.Marshal(status)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("content-type", "application/json")
	w.Write(resp)
}

// GetMetadata gets metadata for the Oasis Indexer API.
func (h *Handler) GetMetadata(w http.ResponseWriter, r *http.Request) {
	var resp []byte
	resp, err := json.Marshal(latestMetadata)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("content-type", "application/json")
	w.Write(resp)
}
