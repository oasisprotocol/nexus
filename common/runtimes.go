package common

import (
	"fmt"

	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/crypto/signature"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/types"
)

func VerifyUtx(sigContext signature.Context, utx *types.UnverifiedTransaction) (*types.Transaction, error) {
	if len(utx.AuthProofs) == 1 && utx.AuthProofs[0].Module != "" {
		switch utx.AuthProofs[0].Module {
		case "evm.ethereum.v0":
			var chainId int64 = 42262
			tx, err := decodeEthRawTx(utx.Body, &chainId)
			if err != nil {
				return nil, err
			}
			return tx, nil
		default:
			return nil, fmt.Errorf("module-controlled decoding scheme %s not supported", utx.AuthProofs[0].Module)
		}
	} else {
		tx, err := utx.Verify(sigContext)
		if err != nil {
			return nil, fmt.Errorf("verify: %w", err)
		}
		return tx, nil
	}
}
