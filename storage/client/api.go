package client

import (
	"math/big"
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
	// RuntimeContextKey is used to set the relevant runtime name
	// in a request context.
	RuntimeContextKey ContextKey = "runtime"
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
	MinFee *big.Int
	MaxFee *big.Int
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
	MinAvailable    *big.Int
	MaxAvailable    *big.Int
	MinEscrow       *big.Int
	MaxEscrow       *big.Int
	MinDebonding    *big.Int
	MaxDebonding    *big.Int
	MinTotalBalance *big.Int
	MaxTotalBalance *big.Int
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

type RuntimeBlocksRequest struct {
	From   *int64
	To     *int64
	After  *time.Time
	Before *time.Time
}

type RuntimeTransactionsRequest struct {
	Block *int64
}

type RuntimeTokensRequest struct{}
