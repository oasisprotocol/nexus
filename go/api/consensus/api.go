// Package consensus defines API handlers for the Oasis Indexer API,
// for consensus-layer data.
package api

import (
	"encoding/json"
	"net/http"
)

// GetBlocks gets a list of consensus blocks.
func GetBlocks(w http.ResponseWriter, r *http.Request) {
	// TODO
	return
}

// GetBlock gets a consensus block.
func GetBlock(w http.ResponseWriter, r *http.Request) {
	// TODO
	return
}

// GetBeacon gets the consensus beacon state.
func GetBeacon(w http.ResponseWriter, r *http.Request) {
	// TODO
	return
}

// GetBlockEvents gets events in a consensus block.
func GetBlockEvents(w http.ResponseWriter, r *http.Request) {
	// TODO
	return
}

// GetTransactions gets a list of consensus transactions.
func GetTransactions(w http.ResponseWriter, r *http.Request) {
	// TODO
	return
}

// GetTransaction gets a consensus transaction.
func GetTransaction(w http.ResponseWriter, r *http.Request) {
	// TODO
	return
}

// GetTransactionEvents gets a list of events in a consensus transaction.
func GetTransactionEvents(w http.ResponseWriter, r *http.Request) {
	// TODO
	return
}

// GetEpochs gets a list of epochs.
func GetEpochs(w http.ResponseWriter, r *http.Request) {
	// TODO
	return
}

// GetEpoch gets an epoch.
func GetEpoch(w http.ResponseWriter, r *http.Request) {
	// TODO
	return
}

// GetValidators gets a list of consensus validators.
func GetValidators(w http.ResponseWriter, r *http.Request) {
	// TODO
	return
}

// GetCommittees gets a list of runtime committees.
func GetCommittees(w http.ResponseWriter, r *http.Request) {
	// TODO
	return
}

// GetProposals gets a list of governance proposals.
func GetProposals(w http.ResponseWriter, r *http.Request) {
	// TODO
	return
}

// GetProposal gets a governance proposal.
func GetProposal(w http.ResponseWriter, r *http.Request) {
	// TODO
	return
}

// GetEntities gets a list of registered entitites.
func GetEntities(w http.ResponseWriter, r *http.Request) {
	// TODO
	return
}

// GetEntity gets a registered entity.
func GetEntity(w http.ResponseWriter, r *http.Request) {
	// TODO
	return
}

// GetEntityNodes gets a list of nodes for a registered entity.
func GetEntityNodes(w http.ResponseWriter, r *http.Request) {
	// TODO
	return
}

// GetEntityNode gets a node for a registered entity.
func GetEntityNode(w http.ResponseWriter, r *http.Request) {
	// TODO
	return
}

// GetAccounts gets a list of accounts in the consensus ledger.
func GetAccounts(w http.ResponseWriter, r *http.Request) {
	// TODO
	return
}

// GetAccount gets an account in the consensus ledger.
func GetAccount(w http.ResponseWriter, r *http.Request) {
	// TODO
	return
}

// APIStatus is the Oasis Indexer indexing status.
type APIStatus struct {
	// CurrentHeight is the maximum height the indexer has finished indexing,
	// and is ready for querying.
	CurrentHeight int64
}

// GetStatus gets the latest indexing status of the Oasis Indexer.
func GetStatus(w http.ResponseWriter, r *http.Request) {
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
