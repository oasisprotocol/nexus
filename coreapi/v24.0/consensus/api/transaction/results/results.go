package results

import (
	governance "github.com/oasisprotocol/nexus/coreapi/v24.0/governance/api"
	registry "github.com/oasisprotocol/nexus/coreapi/v24.0/registry/api"
	roothash "github.com/oasisprotocol/nexus/coreapi/v24.0/roothash/api"
	staking "github.com/oasisprotocol/nexus/coreapi/v24.0/staking/api"
)

// Event is a consensus service event that may be emitted during processing of
// a transaction.
type Event struct {
	Staking    *staking.Event    `json:"staking,omitempty"`
	Registry   *registry.Event   `json:"registry,omitempty"`
	RootHash   *roothash.Event   `json:"roothash,omitempty"`
	Governance *governance.Event `json:"governance,omitempty"`
}

// Error is a transaction execution error.
type Error struct {
	Module  string `json:"module,omitempty"`
	Code    uint32 `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}

// Result is a transaction execution result.
type Result struct {
	Error  Error    `json:"error"`
	Events []*Event `json:"events"`
}

// IsSuccess returns true if transaction execution was successful.
// removed func
