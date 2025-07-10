package common

import (
	"fmt"
	"math/big"

	ethTypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/oasisprotocol/oasis-core/go/common/quantity"
	sdkClient "github.com/oasisprotocol/oasis-sdk/client-sdk/go/client"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/crypto/signature/secp256k1"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/modules/evm"
	sdkTypes "github.com/oasisprotocol/oasis-sdk/client-sdk/go/types"

	"github.com/oasisprotocol/nexus/common"
)

func decodeEthRawTx(body []byte, minGasPrice common.BigInt) (*sdkTypes.Transaction, error) {
	var ethTx ethTypes.Transaction
	if err := ethTx.UnmarshalBinary(body); err != nil {
		return nil, fmt.Errorf("rlp decode bytes: %w", err)
	}
	evmV1 := evm.NewV1(nil)
	var tb *sdkClient.TransactionBuilder
	if to := ethTx.To(); to != nil {
		tb = evmV1.Call(to.Bytes(), ethTx.Value().Bytes(), ethTx.Data())
	} else {
		tb = evmV1.Create(ethTx.Value().Bytes(), ethTx.Data())
	}

	chainIDBI := ethTx.ChainId()
	if chainIDBI == nil || chainIDBI.Cmp(big.NewInt(0)) == 0 {
		// Legacy transactions don't have a chain ID, use 1 in that case as a default.
		// https://github.com/ethereum/go-ethereum/issues/31653
		chainIDBI = big.NewInt(1)
	}
	signer := ethTypes.LatestSignerForChainID(chainIDBI)
	pubUncompressed, err := CancunSenderPub(signer, &ethTx)
	if err != nil {
		return nil, fmt.Errorf("recover signer public key: %w", err)
	}
	var sender secp256k1.PublicKey
	if err = sender.UnmarshalBinary(pubUncompressed); err != nil {
		return nil, fmt.Errorf("sdk secp256k1 public key unmarshal binary: %w", err)
	}

	var effectiveGasPrice *big.Int
	switch ethTx.Type() {
	case ethTypes.DynamicFeeTxType:
		effectiveGasPrice, err = ethTx.EffectiveGasTip(common.Ptr(minGasPrice.Int))
		if err != nil {
			return nil, fmt.Errorf("computing effective gas price: %w", err)
		}
		effectiveGasPrice.Add(effectiveGasPrice, common.Ptr(minGasPrice.Int))
	default:
		effectiveGasPrice = ethTx.GasPrice()
	}

	var resolvedFeeAmount quantity.Quantity
	if err = resolvedFeeAmount.FromBigInt(effectiveGasPrice); err != nil {
		return nil, fmt.Errorf("converting gas price: %w", err)
	}
	if err = resolvedFeeAmount.Mul(quantity.NewFromUint64(ethTx.Gas())); err != nil {
		return nil, fmt.Errorf("computing total fee amount: %w", err)
	}
	tb.AppendAuthSignature(sdkTypes.SignatureAddressSpec{Secp256k1Eth: &sender}, ethTx.Nonce())
	tb.SetFeeAmount(sdkTypes.NewBaseUnits(resolvedFeeAmount, sdkTypes.NativeDenomination))
	tb.SetFeeGas(ethTx.Gas())
	return tb.GetTransaction(), nil
}
