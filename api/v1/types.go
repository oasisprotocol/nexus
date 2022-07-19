// Types for API responses.
package v1

import (
	"time"
)

// Status is the API response for GetStatus.
type Status struct {
	LatestChainID string    `json:"latest_chain_id"`
	LatestBlock   int64     `json:"latest_block"`
	LatestUpdate  time.Time `json:"latest_update"`
}

// BlockList is the API response for ListBlocks.
type BlockList struct {
	Blocks []Block `json:"blocks"`
}

// Block is the API response for GetBlock.
type Block struct {
	Height    int64     `json:"height"`
	Hash      string    `json:"hash"`
	Timestamp time.Time `json:"timestamp"`
}

// TransactionList is the API response for ListTransactions.
type TransactionList struct {
	Transactions []Transaction `json:"transactions"`
}

// Transaction is the API response for GetTransaction.
type Transaction struct {
	Height  int64  `json:"height"`
	Hash    string `json:"hash"`
	Sender  string `json:"sender"`
	Nonce   uint64 `json:"nonce"`
	Fee     uint64 `json:"fee"`
	Method  string `json:"method"`
	Body    []byte `json:"body"`
	Success bool   `json:"success"`
}

// EntityList is the API response for ListEntities.
type EntityList struct {
	Entities []Entity `json:"entities"`
}

// Entity is the API response for GetEntity.
type Entity struct {
	ID      string `json:"id"`
	Address string `json:"address,omitempty"`

	Nodes []string `json:"nodes"`
}

// NodeList is the API response for ListEntityNodes.
type NodeList struct {
	EntityID string `json:"entity_id"`
	Nodes    []Node `json:"nodes"`
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

// AccountList is the API response for ListAccounts.
type AccountList struct {
	Accounts []Account `json:"accounts"`
}

// Account is the API response for GetAccount.
type Account struct {
	Address   string `json:"address"`
	Nonce     uint64 `json:"nonce"`
	Available uint64 `json:"available"`
	Escrow    uint64 `json:"escrow"`
	Debonding uint64 `json:"debonding"`
	Total     uint64 `json:"total"`

	Allowances []Allowance `json:"allowances"`
}

// DebondingDelegationList is the API response for ListDebondingDelegations.
type DebondingDelegationList struct {
	DebondingDelegations []DebondingDelegation `json:"debonding_delegations"`
}

// DebondingDelegation is the API response for GetDebondingDelegation.
type DebondingDelegation struct {
	Amount           uint64 `json:"amount"`
	Shares           uint64 `json:"shares"`
	ValidatorAddress string `json:"address"`
	DebondEnd        uint64 `json:"debond_end"`
}

// DelegationList is the API response for ListDelegations.
type DelegationList struct {
	Delegations []Delegation `json:"delegations"`
}

// Delegation is the API response for GetDelegation.
type Delegation struct {
	Amount           uint64 `json:"amount"`
	Shares           uint64 `json:"shares"`
	ValidatorAddress string `json:"address"`
}

type Allowance struct {
	Address string `json:"address"`
	Amount  uint64 `json:"amount"`
}

// Epoch is the API response for ListEpochs.
type EpochList struct {
	Epochs []Epoch `json:"epochs"`
}

// Epoch is the API response for GetEpoch.
type Epoch struct {
	ID          uint64 `json:"id"`
	StartHeight uint64 `json:"start_height"`
	EndHeight   uint64 `json:"end_height,omitempty"`
}

// ProposalList is the API response for ListProposals.
type ProposalList struct {
	Proposals []Proposal `json:"proposals"`
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

// ProposalVotes is the API response for GetProposalVotes.
type ProposalVotes struct {
	ProposalID uint64         `json:"proposal_id"`
	Votes      []ProposalVote `json:"votes"`
}

type ProposalVote struct {
	Address string `json:"address"`
	Vote    string `json:"vote"`
}

// Validators is the API response for GetValidators.
type ValidatorList struct {
	Validators []Validator `json:"validators"`
}

// Validator is the API response for GetValidator.
type Validator struct {
	Name          string `json:"name"`
	EntityAddress string `json:"entity_address"`
	EntityID      string `json:"entity_id"`
	NodeID        string `json:"node_id"`
	Escrow        uint64 `json:"escrow"`
	// If "true", entity is part of validator set (top <scheduler.params.max_validators> by stake).
	Active bool `json:"active"`
	// If "true", an entity has a node that is registered for being a validator, node is up to date, and has successfully registered itself. However, it may or may not be part of validator set (top <scheduler.params.max_validators> by stake).
	Status                 bool                     `json:"status"`
	Media                  ValidatorMedia           `json:"media"`
	CurrentRate            uint64                   `json:"current_rate"`
	CurrentCommissionBound ValidatorCommissionBound `json:"current_commission_bound"`
}

// ValidatorMedia is the metadata for a validator.
type ValidatorMedia struct {
	WebsiteLink  string `json:"url"`
	EmailAddress string `json:"email"`
	TwitterAcc   string `json:"twitter"`
	TgChat       string `json:"tg"`
	Logotype     string `json:"logotype"`
	Name         string `json:"name"`
}

// ValidatorCommissionBound is the commission bound for a validator.
type ValidatorCommissionBound struct {
	Lower      uint64 `json:"lower"`
	Upper      uint64 `json:"upper"`
	EpochStart uint64 `json:"epoch_start"`
	EpochEnd   uint64 `json:"epoch_end"`
}
