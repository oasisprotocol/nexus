package main

import (
	"fmt"

	ethTypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rlp"
	sdkTypes "github.com/oasisprotocol/oasis-sdk/client-sdk/go/types"
)

// todo: move to common
func decodeRaw(body []byte, expectedChainId *uint64) (*sdkTypes.Transaction, error) {
	var ethTx ethTypes.Transaction
	if err := rlp.DecodeBytes(body, &ethTx); err != nil {
		return nil, fmt.Errorf("rlp decode bytes: %w", err)
	}
	// todo: verify signature
	// todo: assemble sdk tx, see raw_tx.rs
}
