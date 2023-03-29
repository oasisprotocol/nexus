package common

import (
	"fmt"
	"math/big"

	"github.com/oasisprotocol/oasis-core/go/common/quantity"
	sdkTypes "github.com/oasisprotocol/oasis-sdk/client-sdk/go/types"
)

func StringifyNativeDenomination(amount *sdkTypes.BaseUnits) (string, error) {
	if amount.Denomination != sdkTypes.NativeDenomination {
		return "", fmt.Errorf("denomination '%s' expecting native denomination '%s'", amount.Denomination, sdkTypes.NativeDenomination)
	}
	return amount.Amount.String(), nil
}

func StringifyBytes(value []byte) string {
	return new(big.Int).SetBytes(value).String()
}

func QuantityFromBytes(value []byte) quantity.Quantity {
	q := *quantity.NewQuantity()
	_ = q.FromBigInt(new(big.Int).SetBytes(value))
	return q
}
