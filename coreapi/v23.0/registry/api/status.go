package api

import (
	"github.com/oasisprotocol/oasis-core/go/common"
	"github.com/oasisprotocol/oasis-core/go/common/crypto/signature"

	beacon "github.com/oasisprotocol/nexus/coreapi/v23.0/beacon/api"
)

// FreezeForever is an epoch that can be used to freeze a node for
// all (practical) time.
const FreezeForever beacon.EpochTime = 0xffffffffffffffff

// NodeStatus is live status of a node.
type NodeStatus struct {
	// ExpirationProcessed is a flag specifying whether the node expiration
	// has already been processed.
	//
	// If you want to check whether a node has expired, check the node
	// descriptor directly instead of this flag.
	ExpirationProcessed bool `json:"expiration_processed"`
	// FreezeEndTime is the epoch when a frozen node can become unfrozen.
	//
	// After the specified epoch passes, this flag needs to be explicitly
	// cleared (set to zero) in order for the node to become unfrozen.
	FreezeEndTime beacon.EpochTime `json:"freeze_end_time"`
	// ElectionEligibleAfter specifies the epoch after which a node is
	// eligible to be included in non-validator committee elections.
	//
	// Note: A value of 0 is treated unconditionally as "ineligible".
	ElectionEligibleAfter beacon.EpochTime `json:"election_eligible_after"`
	// Faults is a set of fault records for nodes that are experiencing
	// liveness failures when participating in specific committees.
	Faults map[common.Namespace]*Fault `json:"faults,omitempty"`
}

// IsFrozen returns true if the node is currently frozen (prevented
// from being considered in scheduling decisions).
// removed func

// Unfreeze makes the node unfrozen.
// removed func

// RecordFailure records a liveness failure in the epoch preceding the specified epoch.
// removed func

// RecordSuccess records success in the epoch preceding the specified epoch.
// removed func

// IsSuspended checks whether the node is suspended in the given epoch.
// removed func

// Fault is used to track the state of nodes that are experiencing liveness failures.
type Fault struct {
	// Failures is the number of times a node has been declared faulty.
	Failures uint8 `json:"failures,omitempty"`
	// SuspendedUntil specifies the epoch number until the node is not eligible for being scheduled
	// into the committee for which it is deemed faulty.
	SuspendedUntil beacon.EpochTime `json:"suspended_until,omitempty"`
}

// RecordFailure records a liveness failure in the epoch preceding the specified epoch.
// removed func

// RecordSuccess records success in the epoch preceding the specified epoch.
// removed func

// removed func

// IsSuspended checks whether the node is suspended in the given epoch.
// removed func

// UnfreezeNode is a request to unfreeze a frozen node.
type UnfreezeNode struct {
	NodeID signature.PublicKey `json:"node_id"`
}
