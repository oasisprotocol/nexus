// Package api implements the vault backend API.
package api

import (
	"github.com/oasisprotocol/nexus/coreapi/v24.0/consensus/api/transaction"
	staking "github.com/oasisprotocol/nexus/coreapi/v24.0/staking/api"
)

const (
	// ModuleName is a unique module name for the vault module.
	ModuleName = "vault"
)

// removed var block

// Backend is a vault implementation.
// removed interface

// VaultQuery is a query for data about a given vault.
type VaultQuery struct {
	// Height is the query height.
	Height int64 `json:"height"`
	// Address is the vault address.
	Address staking.Address `json:"address"`
}

// AddressQuery is a query for data about a given address for the given vault.
type AddressQuery struct {
	// Height is the query height.
	Height int64 `json:"height"`
	// Vault is the vault address.
	Vault staking.Address `json:"vault"`
	// Address is the queried address.
	Address staking.Address `json:"address"`
}

// Genesis is the initial vault state for use in the genesis block.
type Genesis struct {
	// Parameters are the genesis consensus parameters.
	Parameters ConsensusParameters `json:"params"`

	// Vaults are the vaults.
	Vaults []*Vault `json:"vaults,omitempty"`
	// States are the per vault per-address states.
	States map[staking.Address]map[staking.Address]*AddressState `json:"states,omitempty"`
	// PendingActions are the per-vault pending actions.
	PendingActions map[staking.Address][]*PendingAction `json:"pending_actions,omitempty"`
}

// ConsensusParameters are the vault consensus parameters.
type ConsensusParameters struct {
	// Enabled specifies whether the vault service is enabled.
	Enabled bool `json:"enabled,omitempty"`

	// MaxAuthorityAddresses is the maximum number of addresses that can be configured for each
	// authority.
	MaxAuthorityAddresses uint8 `json:"max_authority_addresses,omitempty"`

	// GasCosts are the vault transaction gas costs.
	GasCosts transaction.Costs `json:"gas_costs,omitempty"`
}

// DefaultConsensusParameters are the default vault consensus parameters.
// removed var statement

// ConsensusParameterChanges are allowed vault consensus parameter changes.
type ConsensusParameterChanges struct {
	// MaxAuthorityAddresses is the new maximum number of addresses that can be configured for each
	// authority.
	MaxAuthorityAddresses *uint8 `json:"max_authority_addresses,omitempty"`

	// GasCosts are the new gas costs.
	GasCosts transaction.Costs `json:"gas_costs,omitempty"`
}

// Apply applies changes to the given consensus parameters.
// removed func
