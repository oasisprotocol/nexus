package api

import (
	"github.com/oasisprotocol/nexus/coreapi/v24.0/consensus/api/transaction"
	staking "github.com/oasisprotocol/nexus/coreapi/v24.0/staking/api"
	"github.com/oasisprotocol/oasis-core/go/common/cbor"
)

// removed var statement

// PendingAction is an action waiting for authorizations in order to be executed.
type PendingAction struct {
	// Nonce is the action nonce.
	Nonce uint64 `json:"nonce"`
	// AuthorizedBy contains the addresses that have authorized the action.
	AuthorizedBy []staking.Address `json:"authorized_by"`
	// Action is the pending action itself.
	Action Action `json:"action"`
}

// ContainsAuthorizationFrom returns true iff the given address is among the action authorizers.
// removed func

// Action is a vault action.
type Action struct {
	// Suspend is the suspend action.
	Suspend *ActionSuspend `json:"suspend,omitempty"`
	// Resume is the resume action.
	Resume *ActionResume `json:"resume,omitempty"`
	// ExecuteMessage is the execute message action.
	ExecuteMessage *ActionExecuteMessage `json:"execute_msg,omitempty"`
	// UpdateWithdrawPolicy is the withdraw policy update action.
	UpdateWithdrawPolicy *ActionUpdateWithdrawPolicy `json:"update_withdraw_policy,omitempty"`
	// UpdateAuthority is the authority update action.
	UpdateAuthority *ActionUpdateAuthority `json:"update_authority,omitempty"`
}

// Validate validates the given action.
// removed func

// Equal returns true iff one action is equal to another.
// removed func

// Authorities returns the authorities of the given vault that can authorize this action.
// removed func

// IsAuthorized returns true iff the given address is authorized to execute this action.
// removed func

// PrettyPrint writes a pretty-printed representation of Action to the given writer.
// removed func

// PrettyType returns a representation of Action that can be used for pretty printing.
// removed func

// ActionSuspend is the action to suspend the vault.
type ActionSuspend struct{}

// Authorities returns the authorities of the given vault that can authorize this action.
// removed func

// ActionResume is the action to suspend the vault.
type ActionResume struct{}

// Authorities returns the authorities of the given vault that can authorize this action.
// removed func

// ActionExecuteMessage is the action to execute a message on behalf of the vault. The message is
// dispatched as if the vault originated a transaction.
type ActionExecuteMessage struct {
	// Method is the method that should be called.
	Method transaction.MethodName `json:"method"`
	// Body is the method call body.
	Body cbor.RawMessage `json:"body,omitempty"`
}

// Validate validates the given action.
// removed func

// Authorities returns the authorities of the given vault that can authorize this action.
// removed func

// PrettyPrintBody writes a pretty-printed representation of the message body to the given writer.
// removed func

// PrettyPrint writes a pretty-printed representation of ActionExecuteMessage to the given writer.
// removed func

// PrettyType returns a representation of ActionExecuteMessage that can be used for pretty printing.
// removed func

// PrettyActionExecuteMessage is used for pretty-printing execute message actions so that the actual
// content is displayed instead of the binary blob.
//
// It should only be used for pretty printing.
type PrettyActionExecuteMessage struct {
	Method transaction.MethodName `json:"method"`
	Body   interface{}            `json:"body,omitempty"`
}

// ActionUpdateWithdrawPolicy is the action to update the withdraw policy for a given address.
type ActionUpdateWithdrawPolicy struct {
	// Address is the address the policy update is for.
	Address staking.Address `json:"address"`
	// Policy is the new withdraw policy.
	Policy WithdrawPolicy `json:"policy"`
}

// Validate validates the given action.
// removed func

// Authorities returns the authorities of the given vault that can authorize this action.
// removed func

// PrettyPrint writes a pretty-printed representation of ActionUpdateWithdrawPolicy to the given
// writer.
// removed func

// PrettyType returns a representation of ActionUpdateWithdrawPolicy that can be used for pretty
// printing.
// removed func

// ActionUpdateAuthority is the action to update one of the vault authorities.
type ActionUpdateAuthority struct {
	// AdminAuthority is the new admin authority. If the field is nil no update should be done.
	AdminAuthority *Authority `json:"admin_authority,omitempty"`
	// SuspendAuthority is the new suspend authority. If the field is nil no update should be done.
	SuspendAuthority *Authority `json:"suspend_authority,omitempty"`
}

// Validate validates the given action.
// removed func

// Authorities returns the authorities of the given vault that can authorize this action.
// removed func

// Apply applies the authority update to the given vault.
// removed func

// PrettyPrint writes a pretty-printed representation of ActionUpdateAuthority to the given
// writer.
// removed func

// PrettyType returns a representation of ActionUpdateAuthority that can be used for pretty
// printing.
// removed func
