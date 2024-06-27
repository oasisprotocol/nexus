package api

import (
	"github.com/oasisprotocol/oasis-core/go/common/quantity"
)

// AddressState is the state stored for the given address.
type AddressState struct {
	// WithdrawPolicy is the active withdraw policy.
	WithdrawPolicy WithdrawPolicy `json:"withdraw_policy"`

	// CurrentBucket specifies the interval we are currently doing accounting for.
	CurrentBucket uint64 `json:"bucket"`
	// CurrentAmount specifies the amount already withdrawn in the current interval.
	CurrentAmount quantity.Quantity `json:"amount"`
}

// UpdateWithdrawPolicy updates the withdraw policy to a new policy together with any internal
// accounting adjustments.
// removed func

// AuthorizeWithdrawal performs withdrawal authorization. In case withdrawal is allowed, the state
// is also updated to reflect the additional withdrawal.
// removed func

// WithdrawPolicy is the per-address withdraw policy.
type WithdrawPolicy struct {
	// LimitAmount is the maximum amount of tokens that may be withdrawn in the given interval.
	LimitAmount quantity.Quantity `json:"limit_amount"`
	// LimitInterval is the interval (in blocks) when the limit amount resets.
	LimitInterval uint64 `json:"limit_interval"`
}

// IsDisabled returns true iff the policy is disabled and no withdrawal is allowed.
// removed func

// Validate validates the withdrawal policy.
// removed func

// PrettyPrint writes a pretty-printed representation of WithdrawPolicy to the given writer.
// removed func

// PrettyType returns a representation of WithdrawPolicy that can be used for pretty printing.
// removed func
