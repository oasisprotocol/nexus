package common

import (
	"fmt"

	"github.com/oasisprotocol/oasis-core/go/common/cbor"
	sdkTypes "github.com/oasisprotocol/oasis-sdk/client-sdk/go/types"

	"github.com/oasisprotocol/nexus/common"
)

// OpenUtxNoVerify decodes the transaction inside an UnverifiedTransaction
// without verifying the authentication. And that's okay for our use case,
// where we obtain transactions that are known to be properly authenticated
// from a trusted node.
func OpenUtxNoVerify(utx *sdkTypes.UnverifiedTransaction, minGasPrice common.BigInt) (*sdkTypes.Transaction, error) {
	if len(utx.AuthProofs) == 1 && utx.AuthProofs[0].Module != "" {
		switch utx.AuthProofs[0].Module {
		case "evm.ethereum.v0":
			tx, err := decodeEthRawTx(utx.Body, minGasPrice)
			if err != nil {
				return nil, err
			}
			return tx, nil
		default:
			return nil, fmt.Errorf("module-controlled decoding scheme %s not supported", utx.AuthProofs[0].Module)
		}
	} else {
		var tx sdkTypes.Transaction
		if err := cbor.Unmarshal(utx.Body, &tx); err != nil {
			return nil, fmt.Errorf("tx cbor unmarshal: %w", err)
		}
		if err := tx.ValidateBasic(); err != nil {
			return nil, fmt.Errorf("tx validate basic: %w", err)
		}
		return &tx, nil
	}
}
