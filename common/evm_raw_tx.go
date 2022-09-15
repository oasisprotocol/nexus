package common

import (
	"fmt"

	ethTypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/oasisprotocol/oasis-core/go/common/quantity"
	sdkClient "github.com/oasisprotocol/oasis-sdk/client-sdk/go/client"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/crypto/signature/secp256k1"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/modules/evm"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/types"
)

func decodeEthRawTx(body []byte, expectedChainId uint64) (*types.Transaction, error) {
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
	chainIdBI := ethTx.ChainId()
	if !chainIdBI.IsUint64() || chainIdBI.Uint64() != expectedChainId {
		return nil, fmt.Errorf("chain ID %v, expected %v", chainIdBI, expectedChainId)
	}
	signer := ethTypes.LatestSignerForChainID(chainIdBI)
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
	tb.AppendAuthSignature(types.SignatureAddressSpec{Secp256k1Eth: &sender}, ethTx.Nonce())
	tb.SetFeeAmount(types.NewBaseUnits(resolvedFeeAmount, types.NativeDenomination))
	tb.SetFeeGas(ethTx.Gas())
	return tb.GetTransaction(), nil
}
