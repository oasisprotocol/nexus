// Types for storage client responses.
package client

import (
	"time"
)

// Status is the storage response for GetStatus.
type Status struct {
	LatestChainID string    `json:"latest_chain_id"`
	LatestBlock   int64     `json:"latest_block"`
	LatestUpdate  time.Time `json:"latest_update"`
}

// BlockList is the storage response for ListBlocks.
type BlockList struct {
	Blocks []Block `json:"blocks"`
}

// Block is the storage response for GetBlock.
type Block struct {
	Height    int64     `json:"height"`
	Hash      string    `json:"hash"`
	Timestamp time.Time `json:"timestamp"`
}

// TransactionList is the storage response for ListTransactions.
type TransactionList struct {
	Transactions []Transaction `json:"transactions"`
}

// Transaction is the storage response for GetTransaction.
type Transaction struct {
	Height  int64  `json:"height"`
	Hash    string `json:"hash"`
	Sender  string `json:"sender"`
	Nonce   uint64 `json:"nonce"`
	Fee     BigInt `json:"fee"`
	Method  string `json:"method"`
	Body    []byte `json:"body"`
	Success bool   `json:"success"`
}

// EntityList is the storage response for ListEntities.
type EntityList struct {
	Entities []Entity `json:"entities"`
}

// Entity is the storage response for GetEntity.
type Entity struct {
	ID      string `json:"id"`
	Address string `json:"address,omitempty"`

	Nodes []string `json:"nodes"`
}

// NodeList is the storage response for ListEntityNodes.
type NodeList struct {
	EntityID string `json:"entity_id"`
	Nodes    []Node `json:"nodes"`
}

// Node is the storage response for GetEntityNode.
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

// AccountList is the storage response for ListAccounts.
type AccountList struct {
	Accounts []Account `json:"accounts"`
}

// Account is the storage response for GetAccount.
type Account struct {
	Address                     string `json:"address"`
	Nonce                       uint64 `json:"nonce"`
	Available                   BigInt `json:"available"`
	Escrow                      BigInt `json:"escrow"`
	Debonding                   BigInt `json:"debonding"`
	DelegationsBalance          BigInt `json:"delegations_balance,omitempty"`
	DebondingDelegationsBalance BigInt `json:"debonding_delegations_balance,omitempty"`

	Allowances []Allowance `json:"allowances"`
}

// DebondingDelegationList is the storage response for ListDebondingDelegations.
type DebondingDelegationList struct {
	DebondingDelegations []DebondingDelegation `json:"debonding_delegations"`
}

// DebondingDelegation is the storage response for GetDebondingDelegation.
type DebondingDelegation struct {
	Amount           BigInt `json:"amount"`
	Shares           BigInt `json:"shares"`
	ValidatorAddress string `json:"address"`
	DebondEnd        uint64 `json:"debond_end"`
}

// DelegationList is the storage response for ListDelegations.
type DelegationList struct {
	Delegations []Delegation `json:"delegations"`
}

// Delegation is the storage response for GetDelegation.
type Delegation struct {
	Amount           BigInt `json:"amount"`
	Shares           BigInt `json:"shares"`
	ValidatorAddress string `json:"address"`
}

type Allowance struct {
	Address string `json:"address"`
	Amount  BigInt `json:"amount"`
}

// Epoch is the storage response for ListEpochs.
type EpochList struct {
	Epochs []Epoch `json:"epochs"`
}

// Epoch is the storage response for GetEpoch.
type Epoch struct {
	ID          uint64 `json:"id"`
	StartHeight uint64 `json:"start_height"`
	EndHeight   uint64 `json:"end_height,omitempty"`
}

// ProposalList is the storage response for ListProposals.
type ProposalList struct {
	Proposals []Proposal `json:"proposals"`
}

// Proposal is the storage response for GetProposal.
type Proposal struct {
	ID           uint64         `json:"id"`
	Submitter    string         `json:"submitter"`
	State        string         `json:"state"`
	Deposit      BigInt         `json:"deposit"`
	Handler      *string        `json:"handler,omitempty"`
	Target       ProposalTarget `json:"target,omitempty"`
	Epoch        *uint64        `json:"epoch,omitempty"`
	Cancels      *int64         `json:"cancels,omitempty"`
	CreatedAt    uint64         `json:"created_at"`
	ClosesAt     uint64         `json:"closes_at"`
	InvalidVotes BigInt         `json:"invalid_votes"`
}

type ProposalTarget struct {
	ConsensusProtocol        *string `json:"consensus_protocol"`
	RuntimeHostProtocol      *string `json:"runtime_host_protocol"`
	RuntimeCommitteeProtocol *string `json:"runtime_committee_protocol"`
}

// ProposalVotes is the storage response for GetProposalVotes.
type ProposalVotes struct {
	ProposalID uint64         `json:"proposal_id"`
	Votes      []ProposalVote `json:"votes"`
}

type ProposalVote struct {
	Address string `json:"address"`
	Vote    string `json:"vote"`
}

// ValidatorList is the storage response for GetValidators.
type ValidatorList struct {
	Validators []Validator `json:"validators"`
}

// Validator is the storage response for GetValidator.
type Validator struct {
	Name          string `json:"name"`
	EntityAddress string `json:"entity_address"`
	EntityID      string `json:"entity_id"`
	NodeID        string `json:"node_id"`
	Escrow        BigInt `json:"escrow"`
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

// RuntimeBlockList is the storage response for RuntimeListBlocks.
type RuntimeBlockList struct {
	Blocks []RuntimeBlock `json:"blocks"`
}

// Block is the storage response for RuntimeGetBlock.
type RuntimeBlock struct {
	Round           int64     `json:"round"`
	Hash            string    `json:"hash"`
	Timestamp       time.Time `json:"timestamp"`
	NumTransactions int       `json:"num_transactions"`
	Size            int       `json:"size_bytes"`
	GasUsed         int64     `json:"gas_used"`
}

// RuntimeTransactionList is the storage response for RuntimeTransactions.
type RuntimeTransactionList struct {
	Transactions []RuntimeTransaction `json:"transactions"`
}

// RuntimeTransaction is the storage response for RuntimeTransaction.
type RuntimeTransaction struct {
	Round     int64
	Index     int64
	Hash      string
	EthHash   *string
	Raw       []byte
	ResultRaw []byte
}

type RuntimeTokenList struct {
	Tokens []RuntimeToken `json:"tokens"`
}

type RuntimeToken struct {
	ContractAddr string `json:"contract_addr"`
	NumHolders   int64  `json:"num_holders"`
}

// TxVolumeList is the storage response for GetVolumes.
type TxVolumeList struct {
	Buckets           []TxVolume `json:"buckets"`
	BucketSizeSeconds uint32     `json:"bucket_size_seconds"`
}

// TxVolume is the daily transaction volume on the specified day.
type TxVolume struct {
	BucketStart time.Time `json:"start"`
	Volume      uint64    `json:"volume"`
}
