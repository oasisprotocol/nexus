// Package message implements the supported runtime messages.
package message

import (
	"github.com/oasisprotocol/oasis-core/go/common/cbor"

	governance "github.com/oasisprotocol/nexus/coreapi/v23.0/governance/api"
	registry "github.com/oasisprotocol/nexus/coreapi/v23.0/registry/api"
	staking "github.com/oasisprotocol/nexus/coreapi/v23.0/staking/api"
)

// Message is a message that can be sent by a runtime.
type Message struct {
	Staking    *StakingMessage    `json:"staking,omitempty"`
	Registry   *RegistryMessage   `json:"registry,omitempty"`
	Governance *GovernanceMessage `json:"governance,omitempty"`
}

// ValidateBasic performs basic validation of the runtime message.
// removed func

// MessagesHash returns a hash of provided runtime messages.
// removed func

// StakingMessage is a runtime message that allows a runtime to perform staking operations.
type StakingMessage struct {
	cbor.Versioned

	Transfer      *staking.Transfer      `json:"transfer,omitempty"`
	Withdraw      *staking.Withdraw      `json:"withdraw,omitempty"`
	AddEscrow     *staking.Escrow        `json:"add_escrow,omitempty"`
	ReclaimEscrow *staking.ReclaimEscrow `json:"reclaim_escrow,omitempty"`
}

// ValidateBasic performs basic validation of the runtime message.
// removed func

// RegistryMessage is a runtime message that allows a runtime to perform staking operations.
type RegistryMessage struct {
	cbor.Versioned

	UpdateRuntime *registry.Runtime `json:"update_runtime,omitempty"`
}

// ValidateBasic performs basic validation of the runtime message.
// removed func

// GovernanceMessage is a governance message that allows a runtime to perform governance operations.
type GovernanceMessage struct {
	cbor.Versioned

	CastVote       *governance.ProposalVote    `json:"cast_vote,omitempty"`
	SubmitProposal *governance.ProposalContent `json:"submit_proposal,omitempty"`
}

// ValidateBasic performs basic validation of a governance message.
// removed func
