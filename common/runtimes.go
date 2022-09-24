package common

import (
	"fmt"

	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/crypto/signature"
	sdkTypes "github.com/oasisprotocol/oasis-sdk/client-sdk/go/types"
)

func VerifyUtx(sigContext signature.Context, utx *sdkTypes.UnverifiedTransaction) (*sdkTypes.Transaction, error) {
	if len(utx.AuthProofs) == 1 && utx.AuthProofs[0].Module != "" {
		switch utx.AuthProofs[0].Module {
		case "evm.ethereum.v0":
			tx, err := decodeEthRawTx(utx.Body, 42262)
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
