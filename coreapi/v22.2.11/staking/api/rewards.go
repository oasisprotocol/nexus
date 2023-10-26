package api

import (
	"github.com/oasisprotocol/oasis-core/go/common/quantity"

	beacon "github.com/oasisprotocol/nexus/coreapi/v22.2.11/beacon/api"
)

// RewardAmountDenominator is the denominator for the reward rate.
var RewardAmountDenominator *quantity.Quantity

// RewardStep is one of the time periods in the reward schedule.
type RewardStep struct {
	Until beacon.EpochTime  `json:"until"`
	Scale quantity.Quantity `json:"scale"`
}

// removed func
