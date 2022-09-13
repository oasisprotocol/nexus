package common

import (
	"fmt"
	"math/big"

	ethTypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/oasisprotocol/oasis-core/go/common/quantity"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/crypto/signature/secp256k1"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/modules/evm"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/types"
)

func decodeEthRawTx(body []byte, expectedChainId *int64) (*types.Transaction, error) {
	var ethTx ethTypes.Transaction
	if err := rlp.DecodeBytes(body, &ethTx); err != nil {
		return nil, fmt.Errorf("rlp decode bytes: %w", err)
	}
	var tx *types.Transaction
	if to := ethTx.To(); to != nil {
		tx = evm.NewCallTx(nil, &evm.Call{
			Address: to.Bytes(),
			Value:   ethTx.Value().Bytes(),
			Data:    ethTx.Data(),
		})
	} else {
		tx = evm.NewV1(nil).Create(ethTx.Value().Bytes(), ethTx.Data()).GetTransaction()
	}
	var expectedChainIdBI *big.Int
	if expectedChainId != nil {
		expectedChainIdBI = big.NewInt(*expectedChainId)
	}
	signer := ethTypes.LatestSignerForChainID(expectedChainIdBI)
	pubUncompressed, err := LondonSenderPub(signer, &ethTx)
	if err != nil {
		return nil, fmt.Errorf("recover signer public key: %w", err)
	}
	var sender secp256k1.PublicKey
	if err = sender.UnmarshalBinary(pubUncompressed); err != nil {
		return nil, fmt.Errorf("sdk secp256k1 public key unmarshal binary: %w", err)
	}
	// Base fee is zero. Allocate only priority fee.
	gasPrice := ethTx.GasTipCap()
	if ethTx.GasFeeCapIntCmp(gasPrice) < 0 {
		gasPrice = ethTx.GasFeeCap()
	}
	var resolvedFeeAmount quantity.Quantity
	if err = resolvedFeeAmount.FromBigInt(gasPrice); err != nil {
		return nil, fmt.Errorf("converting gas price: %w", err)
	}
	if err = resolvedFeeAmount.Mul(quantity.NewFromUint64(ethTx.Gas())); err != nil {
		return nil, fmt.Errorf("computing total fee amount: %w", err)
	}
	tx.AuthInfo.SignerInfo = append(tx.AuthInfo.SignerInfo, types.SignerInfo{
		AddressSpec: types.AddressSpec{
			Signature: &types.SignatureAddressSpec{
				Secp256k1Eth: &sender,
			},
		},
		Nonce: ethTx.Nonce(),
	})
	tx.AuthInfo.Fee.Amount = types.NewBaseUnits(resolvedFeeAmount, types.NativeDenomination)
	tx.AuthInfo.Fee.Gas = ethTx.Gas()
	return tx, nil
}
