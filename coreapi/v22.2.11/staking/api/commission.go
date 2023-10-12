package api

import (
	beacon "github.com/oasisprotocol/nexus/coreapi/v22.2.11/beacon/api"
	"github.com/oasisprotocol/oasis-core/go/common/quantity"
)

// commissionRateDenominatorExponent is the commission rate denominator's
// base-10 exponent.
//
// NOTE: Setting it to 5 means commission rates are denominated in 1000ths of a
// percent.
const commissionRateDenominatorExponent uint8 = 5

// removed var block

// CommissionScheduleRules controls how commission schedule rates and rate
// bounds are allowed to be changed.
type CommissionScheduleRules struct {
	// Epoch period when commission rates are allowed to be changed (e.g.
	// setting it to 3 means they can be changed every third epoch).
	RateChangeInterval beacon.EpochTime `json:"rate_change_interval,omitempty"`
	// Number of epochs a commission rate bound change must specified in advance.
	RateBoundLead beacon.EpochTime `json:"rate_bound_lead,omitempty"`
	// Maximum number of commission rate steps a commission schedule can specify.
	MaxRateSteps uint16 `json:"max_rate_steps,omitempty"`
	// Maximum number of commission rate bound steps a commission schedule can specify.
	MaxBoundSteps uint16 `json:"max_bound_steps,omitempty"`
}

// CommissionRateStep sets a commission rate and its starting time.
type CommissionRateStep struct {
	// Epoch when the commission rate will go in effect.
	Start beacon.EpochTime `json:"start,omitempty"`
	// Commission rate numerator. The rate is this value divided by CommissionRateDenominator.
	Rate quantity.Quantity `json:"rate,omitempty"`
}

// PrettyPrint writes a pretty-printed representation of CommissionRateStep to
// the given writer.
// removed func

// PrettyType returns a representation of CommissionRateStep that can be used
// for pretty printing.
// removed func

// CommissionRateBoundStep sets a commission rate bound (i.e. the minimum and
// maximum commission rate) and its starting time.
type CommissionRateBoundStep struct {
	// Epoch when the commission rate bound will go in effect.
	Start beacon.EpochTime `json:"start,omitempty"`
	// Minimum commission rate numerator. The minimum rate is this value divided by CommissionRateDenominator.
	RateMin quantity.Quantity `json:"rate_min,omitempty"`
	// Maximum commission rate numerator. The maximum rate is this value divided by CommissionRateDenominator.
	RateMax quantity.Quantity `json:"rate_max,omitempty"`
}

// PrettyPrint writes a pretty-printed representation of CommissionRateBoundStep
// to the given writer.
// removed func

// PrettyType returns a representation of CommissionRateBoundStep that can be
// used for pretty printing.
// removed func

// CommissionSchedule defines a list of commission rates and commission rate
// bounds and their starting times.
type CommissionSchedule struct {
	// List of commission rates and their starting times.
	Rates []CommissionRateStep `json:"rates,omitempty"`
	// List of commission rate bounds and their starting times.
	Bounds []CommissionRateBoundStep `json:"bounds,omitempty"`
}

// PrettyPrint writes a pretty-printed representation of CommissionSchedule to
// the given writer.
// removed func

// PrettyType returns a representation of CommissionSchedule that can be used
// for pretty printing.
// removed func

// removed func

// validateNondegenerate detects degenerate steps.
// removed func

// validateAmendmentAcceptable apply policy for "when" changes can be made, for CommissionSchedules that are amendments.
// removed func

// Prune discards past steps that aren't in effect anymore.
// removed func

// amend changes the schedule to use new given steps, replacing steps that are fully covered in the amendment.
// removed func

// validateWithinBound detects rates out of bound.
// removed func

// PruneAndValidateForGenesis gets a schedule ready for use in the genesis document.
// Returns an error if there is a validation failure. If it does, the schedule may be pruned already.
// removed func

// AmendAndPruneAndValidate applies a proposed amendment to a valid schedule.
// Returns an error if there is a validation failure. If it does, the schedule may be amended and pruned already.
// removed func

// CurrentRate returns the rate at the latest rate step that has started or nil if no step has started.
func (cs *CommissionSchedule) CurrentRate(now beacon.EpochTime) *quantity.Quantity {
	var latestStartedStep *CommissionRateStep
	for i := range cs.Rates {
		step := &cs.Rates[i]
		if step.Start > now {
			break
		}
		latestStartedStep = step
	}
	if latestStartedStep == nil {
		return nil
	}
	return &latestStartedStep.Rate
}

// removed func
