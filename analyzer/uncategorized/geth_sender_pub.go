// Copyright 2016 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

// This file contains code from go-ethereum, adapted to recover a public key instead of an address.

//nolint:gocritic,godot
package common

import (
	"errors"
	"fmt"
	"math/big"

	ethCommon "github.com/ethereum/go-ethereum/common"
	ethTypes "github.com/ethereum/go-ethereum/core/types"
	ethCrypto "github.com/ethereum/go-ethereum/crypto"
)

// CancunSenderPub is adapted from cancunSigner.Sender
func CancunSenderPub(s ethTypes.Signer, tx *ethTypes.Transaction) ([]byte, error) {
	if tx.Type() != ethTypes.BlobTxType {
		return LondonSenderPub(s, tx)
	}
	V, R, S := tx.RawSignatureValues()
	// Blob txs are defined to use 0 and 1 as their recovery
	// id, add 27 to become equivalent to unprotected Homestead signatures.
	V = new(big.Int).Add(V, big.NewInt(27))
	if tx.ChainId().Cmp(s.ChainID()) != 0 {
		return nil, fmt.Errorf("%w: have %d want %d", ethTypes.ErrInvalidChainId, tx.ChainId(), s.ChainID())
	}
	return RecoverPlainPub(s.Hash(tx), R, S, V, true)
}

// LondonSenderPub is adapted from londonSigner.Sender
func LondonSenderPub(s ethTypes.Signer, tx *ethTypes.Transaction) ([]byte, error) {
	if tx.Type() != ethTypes.DynamicFeeTxType {
		return Eip2930SenderPub(s, tx)
	}
	V, R, S := tx.RawSignatureValues()
	// DynamicFee txs are defined to use 0 and 1 as their recovery
	// id, add 27 to become equivalent to unprotected Homestead signatures.
	V = new(big.Int).Add(V, big.NewInt(27))
	if tx.ChainId().Cmp(s.ChainID()) != 0 {
		return nil, ethTypes.ErrInvalidChainId
	}
	return RecoverPlainPub(s.Hash(tx), R, S, V, true)
}

// Eip2930SenderPub is adapted from eip2930Signer.Sender
func Eip2930SenderPub(s ethTypes.Signer, tx *ethTypes.Transaction) ([]byte, error) {
	V, R, S := tx.RawSignatureValues()
	switch tx.Type() {
	case ethTypes.LegacyTxType:
		return EIP155SenderPub(s, tx)
	case ethTypes.AccessListTxType:
		// AL txs are defined to use 0 and 1 as their recovery
		// id, add 27 to become equivalent to unprotected Homestead signatures.
		V = new(big.Int).Add(V, big.NewInt(27))
	default:
		return nil, ethTypes.ErrTxTypeNotSupported
	}
	if tx.ChainId().Cmp(s.ChainID()) != 0 {
		return nil, fmt.Errorf("%w: have %d want %d", ethTypes.ErrInvalidChainId, tx.ChainId(), s.ChainID())
	}
	return RecoverPlainPub(s.Hash(tx), R, S, V, true)
}

// EIP155SenderPub is adapted from EIP155Signer.Sender
func EIP155SenderPub(s ethTypes.Signer, tx *ethTypes.Transaction) ([]byte, error) {
	if tx.Type() != ethTypes.LegacyTxType {
		return nil, ethTypes.ErrTxTypeNotSupported
	}
	if !tx.Protected() {
		return HomesteadSenderPub(tx)
	}
	if tx.ChainId().Cmp(s.ChainID()) != 0 {
		return nil, fmt.Errorf("%w: have %d want %d", ethTypes.ErrInvalidChainId, tx.ChainId(), s.ChainID())
	}
	V, R, S := tx.RawSignatureValues()
	V = new(big.Int).Sub(V, new(big.Int).Mul(s.ChainID(), big.NewInt(2)))
	V.Sub(V, big.NewInt(8))
	return RecoverPlainPub(s.Hash(tx), R, S, V, true)
}

// HomesteadSenderPub is adapted from HomesteadSigner.Sender
func HomesteadSenderPub(tx *ethTypes.Transaction) ([]byte, error) {
	if tx.Type() != ethTypes.LegacyTxType {
		return nil, ethTypes.ErrTxTypeNotSupported
	}
	v, r, s := tx.RawSignatureValues()
	return RecoverPlainPub(ethTypes.HomesteadSigner{}.Hash(tx), r, s, v, true)
}

// RecoverPlainPub is adapted from recoverPlain
func RecoverPlainPub(sighash ethCommon.Hash, R, S, Vb *big.Int, homestead bool) ([]byte, error) {
	if Vb.BitLen() > 8 {
		return nil, ethTypes.ErrInvalidSig
	}
	V := byte(Vb.Uint64() - 27)
	if !ethCrypto.ValidateSignatureValues(V, R, S, homestead) {
		return nil, ethTypes.ErrInvalidSig
	}
	// encode the signature in uncompressed format
	r, s := R.Bytes(), S.Bytes()
	sig := make([]byte, ethCrypto.SignatureLength)
	copy(sig[32-len(r):32], r)
	copy(sig[64-len(s):64], s)
	sig[64] = V
	// recover the public key from the signature
	pub, err := ethCrypto.Ecrecover(sighash[:], sig)
	if err != nil {
		return nil, err
	}
	if len(pub) == 0 || pub[0] != 4 {
		return nil, errors.New("invalid public key")
	}
	return pub, nil
}
