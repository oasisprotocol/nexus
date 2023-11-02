// Package api defines the interface exporting the upgrade infrastructure's functionality.
package api

import (
	"github.com/oasisprotocol/oasis-core/go/common/cbor"
	"github.com/oasisprotocol/oasis-core/go/common/version"

	beacon "github.com/oasisprotocol/nexus/coreapi/v23.0/beacon/api"
)

const (
	// ModuleName is the upgrade module name.
	ModuleName = "upgrade"

	// LogEventIncompatibleBinary is a log event value that signals the currently running version
	// of the binary is incompatible with the upgrade.
	LogEventIncompatibleBinary = "upgrade/incompatible-binary"
	// LogEventStartupUpgrade is a log event value that signals the startup upgrade handler was
	// called.
	LogEventStartupUpgrade = "upgrade/startup-upgrade"
	// LogEventConsensusUpgrade is a log event value that signals the consensus upgrade handler was
	// called.
	LogEventConsensusUpgrade = "upgrade/consensus-upgrade"
)

// UpgradeStage is used in the upgrade descriptor to store completed stages.
type UpgradeStage uint64

const (
	// UpgradeStageStartup is the startup upgrade stage, executed at the beginning of node startup.
	UpgradeStageStartup UpgradeStage = 1

	// UpgradeStageConsensus is the upgrade stage carried out during consensus events.
	UpgradeStageConsensus UpgradeStage = 2

	upgradeStageLast = UpgradeStageConsensus

	// InvalidUpgradeHeight means the upgrade epoch hasn't been reached yet.
	InvalidUpgradeHeight = int64(0)

	// LatestDescriptorVersion is the latest upgrade descriptor version that should be used for
	// descriptors.
	LatestDescriptorVersion = 1

	// MinDescriptorVersion is the minimum descriptor version that is allowed.
	MinDescriptorVersion = 1
	// MaxDescriptorVersion is the maximum descriptor version that is allowed.
	MaxDescriptorVersion = LatestDescriptorVersion

	// LatestPendingUpgradeVersion is the latest pending upgrade struct version.
	LatestPendingUpgradeVersion = 1

	// MinUpgradeHandlerLength is the minimum length of upgrade handler's name.
	MinUpgradeHandlerLength = 3
	// MaxUpgradeHandlerLength is the maximum length of upgrade handler's name.
	MaxUpgradeHandlerLength = 32

	// MinUpgradeEpoch is the minimum upgrade epoch.
	MinUpgradeEpoch = beacon.EpochTime(1)
	// MaxUpgradeEpoch is the maximum upgrade epoch.
	MaxUpgradeEpoch = beacon.EpochInvalid - 1
)

// removed var block

// HandlerName is the name of the upgrade descriptor handler.
type HandlerName string

// ValidateBasic does basic validation checks of the upgrade descriptor handler name.
// removed func

// Descriptor describes an upgrade.
type Descriptor struct { // nolint: maligned
	cbor.Versioned

	// Handler is the name of the upgrade handler.
	Handler HandlerName `json:"handler"`
	// Target is upgrade's target version.
	Target version.ProtocolVersions `json:"target"`
	// Epoch is the epoch at which the upgrade should happen.
	Epoch beacon.EpochTime `json:"epoch"`
}

// Equals compares descriptors for equality.
// removed func

// ValidateBasic does basic validation checks of the upgrade descriptor.
// removed func

// EnsureCompatible checks if currently running binary is compatible with
// the upgrade descriptor.
// removed func

// PrettyPrint writes a pretty-printed representation of Descriptor to the given
// writer.
// removed func

// PrettyType returns a representation of Descriptor that can be used for pretty
// printing.
// removed func

// PendingUpgrade describes a currently pending upgrade and includes the
// submitted upgrade descriptor.
type PendingUpgrade struct {
	cbor.Versioned

	// Descriptor is the upgrade descriptor describing the upgrade.
	Descriptor *Descriptor `json:"descriptor"`

	// UpgradeHeight is the height at which the upgrade epoch was reached
	// (or InvalidUpgradeHeight if it hasn't been reached yet).
	UpgradeHeight int64 `json:"upgrade_height"`

	// LastCompletedStage is the last upgrade stage that was successfully completed.
	LastCompletedStage UpgradeStage `json:"last_completed_stage"`
}

// IsCompleted checks if all upgrade stages were already completed.
// removed func

// HasAnyStages checks if any stages were completed at all.
// removed func

// HasStage checks if a given stage has been completed or not.
// removed func

// PushStage marks the given stage as completed.
// removed func

// Backend defines the interface for upgrade managers.
// removed interface
