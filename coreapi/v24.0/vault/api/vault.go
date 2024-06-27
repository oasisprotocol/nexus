package api

import (
	staking "github.com/oasisprotocol/nexus/coreapi/v24.0/staking/api"
)

// removed var statement

// State is the vault state.
type State uint8

const (
	StateSuspended = 0
	StateActive    = 1
)

// Vault contains metadata about a vault.
type Vault struct {
	// Creator is the address of the vault creator.
	Creator staking.Address `json:"creator"`
	// ID is the unique per-creator identifier of the vault.
	ID uint64 `json:"id"`
	// State is the vault state.
	State State `json:"state"`
	// Nonce is the nonce to use for the next action.
	Nonce uint64 `json:"nonce,omitempty"`

	// AdminAuthority specifies the vault's admin authority.
	AdminAuthority Authority `json:"admin_authority"`
	// SuspendAuthority specifies the vault's suspend authority.
	SuspendAuthority Authority `json:"suspend_authority"`
}

// NewVaultAddress returns the address for the vault.
// removed func

// Address returns the address for the vault.
// removed func

// IsActive returns true iff the vault is currently active (processing withdrawals).
// removed func

// AuthoritiesContain returns true iff any of the vault's authorities contain the address.
// removed func

// Authorities returns the list of all vault authorities.
// removed func

// Authority is the vault multisig authority.
type Authority struct {
	// Addresses are the addresses that can authorize an action.
	Addresses []staking.Address `json:"addresses"`
	// Threshold is the minimum number of addresses that must authorize an action.
	Threshold uint8 `json:"threshold"`
}

// Validate validates the authority configuration.
// removed func

// Contains checks whether the authority contains the given address.
// removed func

// Verify checks whether the passed addresses are sufficient to authorize an action.
// removed func

// PrettyPrint writes a pretty-printed representation of Authority to the given writer.
// removed func

// PrettyType returns a representation of Authority that can be used for pretty printing.
// removed func
