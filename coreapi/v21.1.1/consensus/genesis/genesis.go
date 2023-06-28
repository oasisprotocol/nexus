// Package genesis provides consensus config flags that should be part of the genesis state.
package genesis

import (
	"time"

	"github.com/oasisprotocol/oasis-core/go/common/crypto/signature"

	"github.com/oasisprotocol/nexus/coreapi/v21.1.1/consensus/api/transaction"
)

// Genesis contains various consensus config flags that should be part of the genesis state.
type Genesis struct {
	Backend    string     `json:"backend"`
	Parameters Parameters `json:"params"`
}

// Parameters are the consensus parameters.
type Parameters struct { // nolint: maligned
	TimeoutCommit      time.Duration `json:"timeout_commit"`
	SkipTimeoutCommit  bool          `json:"skip_timeout_commit"`
	EmptyBlockInterval time.Duration `json:"empty_block_interval"`

	MaxTxSize       uint64          `json:"max_tx_size"`
	MaxBlockSize    uint64          `json:"max_block_size"`
	MaxBlockGas     transaction.Gas `json:"max_block_gas"`
	MaxEvidenceSize uint64          `json:"max_evidence_size"`

	// StateCheckpointInterval is the expected state checkpoint interval (in blocks).
	StateCheckpointInterval uint64 `json:"state_checkpoint_interval"`
	// StateCheckpointNumKept is the expected minimum number of state checkpoints to keep.
	StateCheckpointNumKept uint64 `json:"state_checkpoint_num_kept,omitempty"`
	// StateCheckpointChunkSize is the chunk size parameter for checkpoint creation.
	StateCheckpointChunkSize uint64 `json:"state_checkpoint_chunk_size,omitempty"`

	// GasCosts are the base transaction gas costs.
	GasCosts transaction.Costs `json:"gas_costs,omitempty"`

	// PublicKeyBlacklist is the network-wide public key blacklist.
	PublicKeyBlacklist []signature.PublicKey `json:"public_key_blacklist,omitempty"`
}

const (
	// GasOpTxByte is the gas operation identifier for costing each transaction byte.
	GasOpTxByte transaction.Op = "tx_byte"
)

// SanityCheck does basic sanity checking on the genesis state.
// removed func
