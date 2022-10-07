package client

import (
	"time"

	"github.com/oasisprotocol/oasis-core/go/common/crypto/signature"
	governance "github.com/oasisprotocol/oasis-core/go/governance/api"
	staking "github.com/oasisprotocol/oasis-core/go/staking/api"
)

type ContextKey string

const (
	// ChainIDContextKey is used to set the relevant chain ID
	// in a request context.
	ChainIDContextKey ContextKey = "chain_id"
	// RequestIDContextKey is used to set a request id for tracing
	// in a request context.
	RequestIDContextKey ContextKey = "request_id"
)

type BlocksRequest struct {
	From   *int64
	To     *int64
	After  *time.Time
	Before *time.Time
}

type BlockRequest struct {
	Height *int64
}

type TransactionsRequest struct {
	Block  *int64
	Method *string
	Sender *staking.Address
	MinFee *int64
	MaxFee *int64
	Code   *int64
}

type TransactionRequest struct {
	TxHash *string
}

type EntityRequest struct {
	EntityID *signature.PublicKey
}

type EntityNodesRequest struct {
	EntityID *signature.PublicKey
}

type EntityNodeRequest struct {
	EntityID *signature.PublicKey
	NodeID   *signature.PublicKey
}

type AccountsRequest struct {
	MinAvailable    *int64
	MaxAvailable    *int64
	MinEscrow       *int64
	MaxEscrow       *int64
	MinDebonding    *int64
	MaxDebonding    *int64
	MinTotalBalance *int64
	MaxTotalBalance *int64
}

type AccountRequest struct {
	Address *staking.Address
}

type DelegationsRequest struct {
	Address *staking.Address
}

type DebondingDelegationsRequest struct {
	Address *staking.Address
}

type EpochRequest struct {
	Epoch *int64
}

type ProposalsRequest struct {
	Submitter *staking.Address
	State     *governance.ProposalState
}

type ProposalRequest struct {
	ProposalID *uint64
}

type ProposalVotesRequest struct {
	ProposalID *uint64
}

type ValidatorRequest struct {
	EntityID *signature.PublicKey
}
