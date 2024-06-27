package api

import (
	"github.com/oasisprotocol/nexus/coreapi/v24.0/consensus/api/transaction"
	staking "github.com/oasisprotocol/nexus/coreapi/v24.0/staking/api"
)

// removed var block

// Create is a create call body.
type Create struct {
	// AdminAuthority specifies the vault's admin authority.
	AdminAuthority Authority `json:"admin_authority"`
	// SuspendAuthority specifies the vault's suspend authority.
	SuspendAuthority Authority `json:"suspend_authority"`
}

// Validate validates the create call.
// removed func

// PrettyPrint writes a pretty-printed representation of Create to the given writer.
// removed func

// PrettyType returns a representation of Create that can be used for pretty printing.
// removed func

// AuthorizeAction is an action authorization call body.
type AuthorizeAction struct {
	// Vault is the address of the target vault.
	Vault staking.Address `json:"vault"`
	// Nonce is the action nonce.
	Nonce uint64 `json:"nonce"`
	// Action is the action that should be authorized.
	Action Action `json:"action"`
}

// Validate validates the action authorization call.
// removed func

// PrettyPrint writes a pretty-printed representation of AuthorizeAction to the given writer.
// removed func

// PrettyType returns a representation of AuthorizeAction that can be used for pretty printing.
// removed func

// CancelAction is an action cancelation call body.
type CancelAction struct {
	// Vault is the address of the target vault.
	Vault staking.Address `json:"vault"`
	// Nonce is the action nonce.
	Nonce uint64 `json:"nonce"`
}

// Validate validates the action cancelation call.
// removed func

// PrettyPrint writes a pretty-printed representation of CancelAction to the given writer.
// removed func

// PrettyType returns a representation of CancelAction that can be used for pretty printing.
// removed func

// NewCreateTx creates a new vault creation transaction.
// removed func

// NewAuthorizeActionTx creates a new authorize action transaction.
// removed func

// NewCancelActionTx creates a new cancel action transaction.
// removed func

const (
	// GasOpCreate is the gas operation identifier for creating a vault.
	GasOpCreate transaction.Op = "create"
	// GasOpAuthorizeAction is the gas operation identifier for authorizing an action.
	GasOpAuthorizeAction transaction.Op = "authorize_action"
	// GasOpCancelAction is the gas operation identifier for canceling an action.
	GasOpCancelAction transaction.Op = "cancel_action"
)

// DefaultGasCosts are the "default" gas costs for operations.
// removed var statement
