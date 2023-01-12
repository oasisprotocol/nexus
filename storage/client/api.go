package client

import (
	"time"

	"github.com/oasisprotocol/oasis-core/go/common/crypto/signature"
	governance "github.com/oasisprotocol/oasis-core/go/governance/api"
	staking "github.com/oasisprotocol/oasis-core/go/staking/api"
	"github.com/oasisprotocol/oasis-indexer/common"
)

type BigInt = common.BigInt

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
	Rel    *staking.Address
	MinFee *BigInt
	MaxFee *BigInt
	Code   *int64
}

type TransactionRequest struct {
	TxHash *string
}

type EventsRequest struct {
	Block   *int64
	TxIndex *int32
	TxHash  *string
	Rel     *staking.Address
	Type    *string
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
	MinAvailable    *BigInt
	MaxAvailable    *BigInt
	MinEscrow       *BigInt
	MaxEscrow       *BigInt
	MinDebonding    *BigInt
	MaxDebonding    *BigInt
	MinTotalBalance *BigInt
	MaxTotalBalance *BigInt
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
