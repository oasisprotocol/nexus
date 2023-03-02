package transaction

import (
	"github.com/oasisprotocol/oasis-core/go/common/quantity"
)

// removed var block

// Gas is the consensus gas representation.
type Gas uint64

// Fee is the consensus transaction fee the sender wishes to pay for
// operations which require a fee to be paid to validators.
type Fee struct {
	// Amount is the fee amount to be paid.
	Amount quantity.Quantity `json:"amount"`
	// Gas is the maximum gas that a transaction can use.
	Gas Gas `json:"gas"`
}

// PrettyPrint writes a pretty-printed representation of the fee to the given
// writer.
// removed func

// PrettyType returns a representation of Fee that can be used for pretty
// printing.
// removed func

// GasPrice returns the gas price implied by the amount and gas.
// removed func

// Costs defines gas costs for different operations.
type Costs map[Op]Gas

// Op identifies an operation that requires gas to run.
type Op string
