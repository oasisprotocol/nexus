// Package api implements the staking backend API.
package api

import (
	"fmt"

	"github.com/oasisprotocol/oasis-core/go/common/crypto/hash"
	"github.com/oasisprotocol/oasis-core/go/common/quantity"
	beacon "github.com/oasisprotocol/oasis-indexer/coreapi/v21.1.1/beacon/api"
	"github.com/oasisprotocol/oasis-indexer/coreapi/v21.1.1/consensus/api/transaction"
)

const (
	// ModuleName is a unique module name for the staking module.
	ModuleName = "staking"

	// LogEventGeneralAdjustment is a log event value that signals adjustment
	// of an account's general balance due to a roothash message.
	LogEventGeneralAdjustment = "staking/general_adjustment"
)

// removed var block

// Backend is a staking implementation.
// removed interface

// ThresholdQuery is a threshold query.
type ThresholdQuery struct {
	Height int64         `json:"height"`
	Kind   ThresholdKind `json:"kind"`
}

// OwnerQuery is an owner query.
type OwnerQuery struct {
	Height int64   `json:"height"`
	Owner  Address `json:"owner"`
}

// AllowanceQuery is an allowance query.
type AllowanceQuery struct {
	Height      int64   `json:"height"`
	Owner       Address `json:"owner"`
	Beneficiary Address `json:"beneficiary"`
}

// TransferEvent is the event emitted when stake is transferred, either by a
// call to Transfer or Withdraw.
type TransferEvent struct {
	From   Address           `json:"from"`
	To     Address           `json:"to"`
	Amount quantity.Quantity `json:"amount"`
}

// BurnEvent is the event emitted when stake is destroyed via a call to Burn.
type BurnEvent struct {
	Owner  Address           `json:"owner"`
	Amount quantity.Quantity `json:"amount"`
}

// EscrowEvent is an escrow event.
type EscrowEvent struct {
	Add     *AddEscrowEvent     `json:"add,omitempty"`
	Take    *TakeEscrowEvent    `json:"take,omitempty"`
	Reclaim *ReclaimEscrowEvent `json:"reclaim,omitempty"`
}

// Event signifies a staking event, returned via GetEvents.
type Event struct {
	Height int64     `json:"height,omitempty"`
	TxHash hash.Hash `json:"tx_hash,omitempty"`

	Transfer        *TransferEvent        `json:"transfer,omitempty"`
	Burn            *BurnEvent            `json:"burn,omitempty"`
	Escrow          *EscrowEvent          `json:"escrow,omitempty"`
	AllowanceChange *AllowanceChangeEvent `json:"allowance_change,omitempty"`
}

// AddEscrowEvent is the event emitted when stake is transferred into an escrow
// account.
type AddEscrowEvent struct {
	Owner     Address           `json:"owner"`
	Escrow    Address           `json:"escrow"`
	Amount    quantity.Quantity `json:"amount"`
	NewShares quantity.Quantity `json:"new_shares"` // Only emitted at _some_ heights; introduced in oasis-core v21.2.0 mid-Cobalt.
}

// TakeEscrowEvent is the event emitted when stake is taken from an escrow
// account (i.e. stake is slashed).
type TakeEscrowEvent struct {
	Owner  Address           `json:"owner"`
	Amount quantity.Quantity `json:"amount"`
}

// ReclaimEscrowEvent is the event emitted when stake is reclaimed from an
// escrow account back into owner's general account.
type ReclaimEscrowEvent struct {
	Owner  Address           `json:"owner"`
	Escrow Address           `json:"escrow"`
	Amount quantity.Quantity `json:"amount"`
	Shares quantity.Quantity `json:"shares"` // Only emitted at _some_ heights; introduced in oasis-core v21.2.0 mid-Cobalt.
}

// AllowanceChangeEvent is the event emitted when allowance is changed for a beneficiary.
type AllowanceChangeEvent struct { // nolint: maligned
	Owner        Address           `json:"owner"`
	Beneficiary  Address           `json:"beneficiary"`
	Allowance    quantity.Quantity `json:"allowance"`
	Negative     bool              `json:"negative,omitempty"`
	AmountChange quantity.Quantity `json:"amount_change"`
}

// Transfer is a stake transfer.
type Transfer struct {
	To     Address           `json:"to"`
	Amount quantity.Quantity `json:"amount"`
}

// PrettyPrint writes a pretty-printed representation of Transfer to the given
// writer.
// removed func

// PrettyType returns a representation of Transfer that can be used for pretty
// printing.
// removed func

// NewTransferTx creates a new transfer transaction.
// removed func

// Burn is a stake burn (destruction).
type Burn struct {
	Amount quantity.Quantity `json:"amount"`
}

// PrettyPrint writes a pretty-printed representation of Burn to the given
// writer.
// removed func

// PrettyType returns a representation of Burn that can be used for pretty
// printing.
// removed func

// NewBurnTx creates a new burn transaction.
// removed func

// Escrow is a stake escrow.
type Escrow struct {
	Account Address           `json:"account"`
	Amount  quantity.Quantity `json:"amount"`
}

// PrettyPrint writes a pretty-printed representation of Escrow to the given
// writer.
// removed func

// PrettyType returns a representation of Escrow that can be used for pretty
// printing.
// removed func

// NewAddEscrowTx creates a new add escrow transaction.
// removed func

// ReclaimEscrow is a reclamation of stake from an escrow.
type ReclaimEscrow struct {
	Account Address           `json:"account"`
	Shares  quantity.Quantity `json:"shares"`
}

// PrettyPrint writes a pretty-printed representation of ReclaimEscrow to the
// given writer.
// removed func

// PrettyType returns a representation of Transfer that can be used for pretty
// printing.
// removed func

// NewReclaimEscrowTx creates a new reclaim escrow transaction.
// removed func

// AmendCommissionSchedule is an amendment to a commission schedule.
type AmendCommissionSchedule struct {
	Amendment CommissionSchedule `json:"amendment"`
}

// PrettyPrint writes a pretty-printed representation of AmendCommissionSchedule
// to the given writer.
// removed func

// PrettyType returns a representation of AmendCommissionSchedule that can be
// used for pretty printing.
// removed func

// NewAmendCommissionScheduleTx creates a new amend commission schedule transaction.
// removed func

// Allow is a beneficiary allowance configuration.
type Allow struct {
	Beneficiary  Address           `json:"beneficiary"`
	Negative     bool              `json:"negative,omitempty"`
	AmountChange quantity.Quantity `json:"amount_change"`
}

// PrettyPrint writes a pretty-printed representation of Allow to the given writer.
// removed func

// PrettyType returns a representation of Allow that can be used for pretty printing.
// removed func

// NewAllowTx creates a new beneficiary allowance configuration transaction.
// removed func

// Withdraw is a withdrawal from an account.
type Withdraw struct {
	From   Address           `json:"from"`
	Amount quantity.Quantity `json:"amount"`
}

// PrettyPrint writes a pretty-printed representation of Withdraw to the given writer.
// removed func

// PrettyType returns a representation of Withdraw that can be used for pretty printing.
// removed func

// NewWithdrawTx creates a new beneficiary allowance configuration transaction.
// removed func

// SharePool is a combined balance of several entries, the relative sizes
// of which are tracked through shares.
type SharePool struct {
	Balance     quantity.Quantity `json:"balance,omitempty"`
	TotalShares quantity.Quantity `json:"total_shares,omitempty"`
}

// PrettyPrint writes a pretty-printed representation of SharePool to the given
// writer.
// removed func

// PrettyType returns a representation of SharePool that can be used for pretty
// printing.
// removed func

// sharesForStake computes the amount of shares for the given amount of base units.
// removed func

// Deposit moves stake into the combined balance, raising the shares.
// If an error occurs, the pool and affected accounts are left in an invalid state.
// removed func

// StakeForShares computes the amount of base units for the given amount of shares.
// removed func

// Withdraw moves stake out of the combined balance, reducing the shares.
// If an error occurs, the pool and affected accounts are left in an invalid state.
// removed func

// ThresholdKind is the kind of staking threshold.
type ThresholdKind int

const (
	KindEntity            ThresholdKind = 0
	KindNodeValidator     ThresholdKind = 1
	KindNodeCompute       ThresholdKind = 2
	KindNodeStorage       ThresholdKind = 3
	KindNodeKeyManager    ThresholdKind = 4
	KindRuntimeCompute    ThresholdKind = 5
	KindRuntimeKeyManager ThresholdKind = 6

	KindMax = KindRuntimeKeyManager

	KindEntityName            = "entity"
	KindNodeValidatorName     = "node-validator"
	KindNodeComputeName       = "node-compute"
	KindNodeStorageName       = "node-storage"
	KindNodeKeyManagerName    = "node-keymanager"
	KindRuntimeComputeName    = "runtime-compute"
	KindRuntimeKeyManagerName = "runtime-keymanager"
)

// String returns the string representation of a ThresholdKind.
func (k ThresholdKind) String() string {
	switch k {
	case KindEntity:
		return KindEntityName
	case KindNodeValidator:
		return KindNodeValidatorName
	case KindNodeCompute:
		return KindNodeComputeName
	case KindNodeStorage:
		return KindNodeStorageName
	case KindNodeKeyManager:
		return KindNodeKeyManagerName
	case KindRuntimeCompute:
		return KindRuntimeComputeName
	case KindRuntimeKeyManager:
		return KindRuntimeKeyManagerName
	default:
		return "[unknown threshold kind]"
	}
}

// MarshalText encodes a ThresholdKind into text form.
func (k ThresholdKind) MarshalText() ([]byte, error) {
	return []byte(k.String()), nil
}

// UnmarshalText decodes a text slice into a ThresholdKind.
func (k *ThresholdKind) UnmarshalText(text []byte) error {
	switch string(text) {
	case KindEntityName:
		*k = KindEntity
	case KindNodeValidatorName:
		*k = KindNodeValidator
	case KindNodeComputeName:
		*k = KindNodeCompute
	case KindNodeStorageName:
		*k = KindNodeStorage
	case KindNodeKeyManagerName:
		*k = KindNodeKeyManager
	case KindRuntimeComputeName:
		*k = KindRuntimeCompute
	case KindRuntimeKeyManagerName:
		*k = KindRuntimeKeyManager
	default:
		return fmt.Errorf("%w: %s", fmt.Errorf("invalid threshold"), string(text))
	}
	return nil
}

// StakeClaim is a unique stake claim identifier.
type StakeClaim string

// StakeThreshold is a stake threshold as used in the stake accumulator.
type StakeThreshold struct {
	// Global is a reference to a global stake threshold.
	Global *ThresholdKind `json:"global,omitempty"`
	// Constant is the value for a specific threshold.
	Constant *quantity.Quantity `json:"const,omitempty"`
}

// String returns a string representation of a stake threshold.
func (st StakeThreshold) String() string {
	switch {
	case st.Global != nil:
		return fmt.Sprintf("<global: %s>", *st.Global)
	case st.Constant != nil:
		return fmt.Sprintf("<constant: %s>", st.Constant)
	default:
		return "<malformed>"
	}
}

// PrettyPrint writes a pretty-printed representation of StakeThreshold to the
// given writer.
// removed func

// PrettyType returns a representation of StakeThreshold that can be used for
// pretty printing.
// removed func

// Equal compares vs another stake threshold for equality.
// removed func

// Value returns the value of the stake threshold.
// removed func

// GlobalStakeThreshold creates a new global StakeThreshold.
// removed func

// GlobalStakeThresholds creates a new list of global StakeThresholds.
// removed func

// StakeAccumulator is a per-escrow-account stake accumulator.
type StakeAccumulator struct {
	// Claims are the stake claims that must be satisfied at any given point. Adding a new claim is
	// only possible if all of the existing claims plus the new claim is satisfied.
	Claims map[StakeClaim][]StakeThreshold `json:"claims,omitempty"`
}

// PrettyPrint writes a pretty-printed representation of StakeAccumulator to the
// given writer.
// removed func

// PrettyType returns a representation of StakeAccumulator that can be used for
// pretty printing.
// removed func

// AddClaimUnchecked adds a new claim without checking its validity.
// removed func

// RemoveClaim removes a given stake claim.
//
// It is an error if the stake claim does not exist.
// removed func

// TotalClaims computes the total amount of stake claims in the accumulator.
// removed func

// GeneralAccount is a general-purpose account.
type GeneralAccount struct {
	Balance quantity.Quantity `json:"balance,omitempty"`
	Nonce   uint64            `json:"nonce,omitempty"`

	Allowances map[Address]quantity.Quantity `json:"allowances,omitempty"`
}

// PrettyPrint writes a pretty-printed representation of GeneralAccount to the
// given writer.
// removed func

// PrettyType returns a representation of GeneralAccount that can be used for
// pretty printing.
// removed func

// EscrowAccount is an escrow account the balance of which is subject to
// special delegation provisions and a debonding period.
type EscrowAccount struct {
	Active             SharePool          `json:"active,omitempty"`
	Debonding          SharePool          `json:"debonding,omitempty"`
	CommissionSchedule CommissionSchedule `json:"commission_schedule,omitempty"`
	StakeAccumulator   StakeAccumulator   `json:"stake_accumulator,omitempty"`
}

// PrettyPrint writes a pretty-printed representation of EscrowAccount to the
// given writer.
// removed func

// PrettyType returns a representation of EscrowAccount that can be used for
// pretty printing.
// removed func

// CheckStakeClaims checks whether the escrow account balance satisfies all the stake claims.
// removed func

// AddStakeClaim attempts to add a stake claim to the given escrow account.
//
// In case there is insufficient stake to cover the claim or an error occurrs, no modifications are
// made to the stake accumulator.
// removed func

// RemoveStakeClaim removes a given stake claim.
//
// It is an error if the stake claim does not exist.
// removed func

// Account is an entry in the staking ledger.
//
// The same ledger entry can hold both general and escrow accounts. Escrow
// accounts are used to hold funds delegated for staking.
type Account struct {
	General GeneralAccount `json:"general,omitempty"`
	Escrow  EscrowAccount  `json:"escrow,omitempty"`
}

// PrettyPrint writes a pretty-printed representation of Account to the given
// writer.
// removed func

// PrettyType returns a representation of Account that can be used for pretty
// printing.
// removed func

// Delegation is a delegation descriptor.
type Delegation struct {
	Shares quantity.Quantity `json:"shares"`
}

// DelegationInfo is a delegation descriptor with additional information.
//
// Additional information contains the share pool the delegation belongs to.
type DelegationInfo struct {
	Delegation
	Pool SharePool `json:"pool"`
}

// DebondingDelegation is a debonding delegation descriptor.
type DebondingDelegation struct {
	Shares        quantity.Quantity `json:"shares"`
	DebondEndTime beacon.EpochTime  `json:"debond_end"`
}

// Merge merges debonding delegations with same debond end time by summing
// the shares amounts.
// removed func

// DebondingDelegationInfo is a debonding delegation descriptor with additional
// information.
//
// Additional information contains the share pool the debonding delegation
// belongs to.
type DebondingDelegationInfo struct {
	DebondingDelegation
	Pool SharePool `json:"pool"`
}

// Genesis is the initial staking state for use in the genesis block.
type Genesis struct {
	// Parameters are the staking consensus parameters.
	Parameters ConsensusParameters `json:"params"`

	// TokenSymbol is the token's ticker symbol.
	// Only upper case A-Z characters are allowed.
	TokenSymbol string `json:"token_symbol"`
	// TokenValueExponent is the token's value base-10 exponent, i.e.
	// 1 token = 10**TokenValueExponent base units.
	TokenValueExponent uint8 `json:"token_value_exponent"`

	// TokenSupply is the network's total amount of stake in base units.
	TotalSupply quantity.Quantity `json:"total_supply"`
	// CommonPool is the network's common stake pool.
	CommonPool quantity.Quantity `json:"common_pool"`
	// LastBlockFees are the collected fees for previous block.
	LastBlockFees quantity.Quantity `json:"last_block_fees"`
	// GovernanceDeposits are network's governance deposits.
	GovernanceDeposits quantity.Quantity `json:"governance_deposits"`

	// Ledger is a map of staking accounts.
	Ledger map[Address]*Account `json:"ledger,omitempty"`

	// Delegations is a nested map of staking delegations of the form:
	// DELEGATEE-ACCOUNT-ADDRESS: DELEGATOR-ACCOUNT-ADDRESS: DELEGATION.
	Delegations map[Address]map[Address]*Delegation `json:"delegations,omitempty"`
	// DebondingDelegations is a nested map of staking delegations of the form:
	// DEBONDING-DELEGATEE-ACCOUNT-ADDRESS: DEBONDING-DELEGATOR-ACCOUNT-ADDRESS: list of DEBONDING-DELEGATIONs.
	DebondingDelegations map[Address]map[Address][]*DebondingDelegation `json:"debonding_delegations,omitempty"`
}

// ConsensusParameters are the staking consensus parameters.
type ConsensusParameters struct { // nolint: maligned
	Thresholds                        map[ThresholdKind]quantity.Quantity `json:"thresholds,omitempty"`
	DebondingInterval                 beacon.EpochTime                    `json:"debonding_interval,omitempty"`
	RewardSchedule                    []RewardStep                        `json:"reward_schedule,omitempty"`
	SigningRewardThresholdNumerator   uint64                              `json:"signing_reward_threshold_numerator,omitempty"`
	SigningRewardThresholdDenominator uint64                              `json:"signing_reward_threshold_denominator,omitempty"`
	CommissionScheduleRules           CommissionScheduleRules             `json:"commission_schedule_rules,omitempty"`
	Slashing                          map[SlashReason]Slash               `json:"slashing,omitempty"`
	GasCosts                          transaction.Costs                   `json:"gas_costs,omitempty"`
	MinDelegationAmount               quantity.Quantity                   `json:"min_delegation"`

	DisableTransfers       bool             `json:"disable_transfers,omitempty"`
	DisableDelegation      bool             `json:"disable_delegation,omitempty"`
	UndisableTransfersFrom map[Address]bool `json:"undisable_transfers_from,omitempty"`

	// AllowEscrowMessages can be used to allow runtimes to perform AddEscrow
	// and ReclaimEscrow via runtime messages.
	AllowEscrowMessages bool `json:"allow_escrow_messages,omitempty"`

	// MaxAllowances is the maximum number of allowances an account can have. Zero means disabled.
	MaxAllowances uint32 `json:"max_allowances,omitempty"`

	// FeeSplitWeightPropose is the proportion of block fee portions that go to the proposer.
	FeeSplitWeightPropose quantity.Quantity `json:"fee_split_weight_propose"`
	// FeeSplitWeightVote is the proportion of block fee portions that go to the validator that votes.
	FeeSplitWeightVote quantity.Quantity `json:"fee_split_weight_vote"`
	// FeeSplitWeightNextPropose is the proportion of block fee portions that go to the next block's proposer.
	FeeSplitWeightNextPropose quantity.Quantity `json:"fee_split_weight_next_propose"`

	// RewardFactorEpochSigned is the factor for a reward distributed per epoch to
	// entities that have signed at least a threshold fraction of the blocks.
	RewardFactorEpochSigned quantity.Quantity `json:"reward_factor_epoch_signed"`
	// RewardFactorBlockProposed is the factor for a reward distributed per block
	// to the entity that proposed the block.
	RewardFactorBlockProposed quantity.Quantity `json:"reward_factor_block_proposed"`
}

const (
	// GasOpTransfer is the gas operation identifier for transfer.
	GasOpTransfer transaction.Op = "transfer"
	// GasOpBurn is the gas operation identifier for burn.
	GasOpBurn transaction.Op = "burn"
	// GasOpAddEscrow is the gas operation identifier for add escrow.
	GasOpAddEscrow transaction.Op = "add_escrow"
	// GasOpReclaimEscrow is the gas operation identifier for reclaim escrow.
	GasOpReclaimEscrow transaction.Op = "reclaim_escrow"
	// GasOpAmendCommissionSchedule is the gas operation identifier for amend commission schedule.
	GasOpAmendCommissionSchedule transaction.Op = "amend_commission_schedule"
	// GasOpAllow is the gas operation identifier for allow.
	GasOpAllow transaction.Op = "allow"
	// GasOpWithdraw is the gas operation identifier for withdraw.
	GasOpWithdraw transaction.Op = "withdraw"
)
