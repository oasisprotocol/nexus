// Package eth contains utilities for EVM.
package eth

import (
	"golang.org/x/crypto/sha3"
)

var (
	TopicERC20Transfer = Keccak256([]byte("Transfer(address,address,uint256)"))
	TopicERC20Approval = Keccak256([]byte("Approval(address,address,uint256)"))
)

var ZeroEthAddr = make([]byte, 20)

func Keccak256(data []byte) []byte {
	h := sha3.NewLegacyKeccak256()
	h.Write(data)
	return h.Sum(nil)
}

func SliceEthAddress(b32 []byte) []byte {
	return b32[32-20:]
}
