// Types for storage client responses.
package client

import (
	api "github.com/oasisprotocol/oasis-indexer/api/v1/types"
)

// Status is the storage response for GetStatus.
type Status = api.Status

// BlockList is the storage response for ListBlocks.
type BlockList = api.BlockList

// Block is the storage response for GetBlock.
type Block = api.Block

// TransactionList is the storage response for ListTransactions.
type TransactionList = api.TransactionList

// Transaction is the storage response for GetTransaction.
type Transaction = api.Transaction

// EventsList is the storage response for ListEvents.
type EventList = api.ConsensusEventList

// Event is a consensus event.
type Event = api.ConsensusEvent

// EntityList is the storage response for ListEntities.
type EntityList = api.EntityList

// Entity is the storage response for GetEntity.
type Entity = api.Entity

// NodeList is the storage response for ListEntityNodes.
type NodeList = api.NodeList

// Node is the storage response for GetEntityNode.
type Node = api.Node

// AccountList is the storage response for ListAccounts.
type AccountList = api.AccountList

// Account is the storage response for GetAccount.
type Account = api.Account

// DebondingDelegationList is the storage response for ListDebondingDelegations.
type DebondingDelegationList = api.DebondingDelegationList

// DebondingDelegation is the storage response for GetDebondingDelegation.
type DebondingDelegation = api.DebondingDelegation

// DelegationList is the storage response for ListDelegations.
type DelegationList = api.DelegationList

// Delegation is the storage response for GetDelegation.
type Delegation = api.Delegation

type Allowance = api.Allowance

// Epoch is the storage response for ListEpochs.
type EpochList = api.EpochList

// Epoch is the storage response for GetEpoch.
type Epoch = api.Epoch

// ProposalList is the storage response for ListProposals.
type ProposalList = api.ProposalList

// Proposal is the storage response for GetProposal.
type Proposal = api.Proposal

type ProposalTarget = api.ProposalTarget

// ProposalVotes is the storage response for GetProposalVotes.
type ProposalVotes = api.ProposalVotes

type ProposalVote = api.ProposalVote

// ValidatorList is the storage response for GetValidators.
type ValidatorList = api.ValidatorList

// Validator is the storage response for GetValidator.
type Validator = api.Validator

// ValidatorMedia is the metadata for a validator.
type ValidatorMedia = api.ValidatorMedia

// ValidatorCommissionBound is the commission bound for a validator.
type ValidatorCommissionBound = api.ValidatorCommissionBound

// RuntimeBlockList is the storage response for RuntimeListBlocks.
type RuntimeBlockList = api.RuntimeBlockList

// Block is the storage response for RuntimeGetBlock.
type RuntimeBlock = api.RuntimeBlock

// RuntimeTransactionList is the storage response for RuntimeTransactions.
type (
	RuntimeTransactionList               = api.RuntimeTransactionList
	RuntimeTransaction                   = api.RuntimeTransaction
	RuntimeTransactionEncryptionEnvelope = api.RuntimeTransactionEncryptionEnvelope
	TxError                              = api.TxError
)

// RuntimeEventList is the storage response for RuntimeEvents.
type RuntimeEventList = api.RuntimeEventList

type RuntimeEvent = api.RuntimeEvent

type RuntimeEventType = api.RuntimeEventType

// RuntimeStatus is the storage response for RuntimeStatus.
type RuntimeStatus = api.RuntimeStatus

// Types that are a part of the storage response for GetRuntimeAccount.
type (
	AddressPreimage                = api.AddressPreimage
	AddressDerivationContext       = api.AddressDerivationContext
	RuntimeSdkBalance              = api.RuntimeSdkBalance
	RuntimeEvmBalance              = api.RuntimeEvmBalance
	RuntimeEvmContract             = api.RuntimeEvmContract
	RuntimeEvmContractVerification = api.RuntimeEvmContractVerification
)

type RuntimeAccount = api.RuntimeAccount

type AccountStats = api.AccountStats

type EvmTokenList = api.EvmTokenList

type EvmToken = api.EvmToken

// TxVolumeList is the storage response for GetVolumes.
type TxVolumeList = api.TxVolumeList

// TxVolume is the daily transaction volume on the specified day.
type TxVolume = api.TxVolume

// DailyActiveAccountsList is the storage response for GetDailyActiveAccounts.
type DailyActiveAccountsList = api.ActiveAccountsList
