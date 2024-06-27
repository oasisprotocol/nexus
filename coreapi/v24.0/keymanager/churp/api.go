package churp

import (
	"github.com/oasisprotocol/nexus/coreapi/v24.0/consensus/api/transaction"
	"github.com/oasisprotocol/oasis-core/go/common"
)

const (
	// ModuleName is the module name for CHURP extension.
	ModuleName = "keymanager/churp"
)

// removed var block

const (
	// GasOpCreate is the gas operation identifier for creation costs.
	GasOpCreate transaction.Op = "create"
	// GasOpUpdate is the gas operation identifier for update costs.
	GasOpUpdate transaction.Op = "update"
	// GasOpApply is the gas operation identifier for application costs.
	GasOpApply transaction.Op = "apply"
	// GasOpConfirm is the gas operation identifier for confirmation costs.
	GasOpConfirm transaction.Op = "confirm"
)

// DefaultGasCosts are the "default" gas costs for operations.
// removed var statement

// DefaultConsensusParameters are the "default" consensus parameters.
// removed var statement

const (
	// StakeClaimScheme is the stake claim template used for creating
	// new CHURP schemes.
	StakeClaimScheme = "keymanager.churp.Scheme.%s.%d"
)

// StakeClaim generates a new stake claim identifier for a specific
// scheme creation.
// removed func

// StakeThresholds returns the staking thresholds.
// removed func

// NewCreateTx creates a new create transaction.
// removed func

// NewUpdateTx creates a new update transaction.
// removed func

// NewApplyTx creates a new apply transaction.
// removed func

// NewConfirmTx creates a new confirm transaction.
// removed func

// StatusQuery is a status query by CHURP and runtime ID.
type StatusQuery struct {
	Height    int64            `json:"height"`
	RuntimeID common.Namespace `json:"runtime_id"`
	ChurpID   uint8            `json:"churp_id"`
}
