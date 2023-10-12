package api

import (
	roothash "github.com/oasisprotocol/nexus/coreapi/v22.2.11/roothash/api"
	"github.com/oasisprotocol/oasis-core/go/common"
	"github.com/oasisprotocol/oasis-core/go/common/crypto/hash"
)

const (
	// ModuleName is the runtime client module name.
	ModuleName = "runtime/client"

	// RoundLatest is a special round number always referring to the latest round.
	RoundLatest = roothash.RoundLatest
)

// removed var block

// RuntimeClient is the runtime client interface.
// removed interface

// SubmitTxResult is the raw result of submitting a transaction for processing.
type SubmitTxResult struct {
	Error  error
	Result *SubmitTxMetaResponse
}

// SubmitTxRequest is a SubmitTx request.
type SubmitTxRequest struct {
	RuntimeID common.Namespace `json:"runtime_id"`
	Data      []byte           `json:"data"`
}

// SubmitTxMetaResponse is the SubmitTxMeta response.
type SubmitTxMetaResponse struct {
	// Output is the transaction output.
	Output []byte `json:"data,omitempty"`
	// Round is the roothash round in which the transaction was executed.
	Round uint64 `json:"round,omitempty"`
	// BatchOrder is the order of the transaction in the execution batch.
	BatchOrder uint32 `json:"batch_order,omitempty"`

	// CheckTxError is the CheckTx error in case transaction failed the transaction check.
	CheckTxError RuntimeHostError `json:"check_tx_error,omitempty"`
}

// RuntimeHostError is a message body representing an error.
// NOTE: RENAMED from "Error" and imported from github.com/oasisprotocol/oasis-core/go/runtime/host/protocol
// as a manual step when vendoring oasis-core v22.2.11.
type RuntimeHostError struct {
	Module  string `json:"module,omitempty"`
	Code    uint32 `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}

// CheckTxRequest is a CheckTx request.
type CheckTxRequest struct {
	RuntimeID common.Namespace `json:"runtime_id"`
	Data      []byte           `json:"data"`
}

// GetBlockRequest is a GetBlock request.
type GetBlockRequest struct {
	RuntimeID common.Namespace `json:"runtime_id"`
	Round     uint64           `json:"round"`
}

// GetTransactionsRequest is a GetTransactions request.
type GetTransactionsRequest struct {
	RuntimeID common.Namespace `json:"runtime_id"`
	Round     uint64           `json:"round"`
}

// TransactionWithResults is a transaction with its raw result and emitted events.
type TransactionWithResults struct {
	Tx     []byte        `json:"tx"`
	Result []byte        `json:"result"`
	Events []*PlainEvent `json:"events,omitempty"`
}

// GetEventsRequest is a GetEvents request.
type GetEventsRequest struct {
	RuntimeID common.Namespace `json:"runtime_id"`
	Round     uint64           `json:"round"`
}

// Event is an event emitted by a runtime in the form of a runtime transaction tag.
//
// Key and value semantics are runtime-dependent.
type Event struct {
	Key    []byte    `json:"key"`
	Value  []byte    `json:"value"`
	TxHash hash.Hash `json:"tx_hash"`
}

// PlainEvent is an event emitted by a runtime in the form of a runtime transaction tag. It
// does not include the transaction hash.
//
// Key and value semantics are runtime-dependent.
type PlainEvent struct {
	Key   []byte `json:"key"`
	Value []byte `json:"value"`
}

// QueryRequest is a Query request.
type QueryRequest struct {
	RuntimeID common.Namespace `json:"runtime_id"`
	Round     uint64           `json:"round"`
	Method    string           `json:"method"`
	Args      []byte           `json:"args"`
}

// QueryResponse is a response to the runtime query.
type QueryResponse struct {
	Data []byte `json:"data"`
}
