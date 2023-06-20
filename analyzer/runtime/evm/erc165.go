package evm

import (
	"context"
	"encoding/hex"
	"fmt"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/oasisprotocol/oasis-core/go/common/errors"

	"github.com/oasisprotocol/nexus/analyzer/evmabi"
	"github.com/oasisprotocol/nexus/log"
	"github.com/oasisprotocol/nexus/storage/oasis/nodeapi"
)

func InterfaceID(abi *abi.ABI) []byte {
	id := []byte{0, 0, 0, 0}
	for _, m := range abi.Methods {
		id[0] ^= m.ID[0]
		id[1] ^= m.ID[1]
		id[2] ^= m.ID[2]
		id[3] ^= m.ID[3]
	}
	return id
}

var (
	InvalidInterfaceID             = []byte{0xff, 0xff, 0xff, 0xff}
	ERC165InterfaceID              = InterfaceID(evmabi.ERC165)
	ERC721InterfaceID              = InterfaceID(evmabi.ERC721)
	ERC721TokenReceiverInterfaceID = InterfaceID(evmabi.ERC721TokenReceiver)
	ERC721MetadataInterfaceID      = InterfaceID(evmabi.ERC721Metadata)
	ERC721EnumerableInterfaceID    = InterfaceID(evmabi.ERC721Enumerable)
)

const ERC165GasLimit uint64 = 30_000

func callERC165SupportsInterface(ctx context.Context, source nodeapi.RuntimeApiLite, round uint64, contractEthAddr []byte, interfaceID []byte) (bool, error) {
	var result bool
	// go-ethereum insists on array and not slice for fixed types like bytes4
	// here.
	var interfaceIDFixed [4]byte
	copy(interfaceIDFixed[:], interfaceID)
	if err := evmCallWithABICustom(ctx, source, round, DefaultGasPrice, ERC165GasLimit, DefaultCaller, contractEthAddr, DefaultValue, evmabi.ERC165, &result, "supportsInterface", interfaceIDFixed); err != nil {
		return false, err
	}
	return result, nil
}

// detectInterfaceOK checks using ERC-165 and returns (result, ok, err). If a
// nondeterministic error occurs, it returns err != nil. If a deterministic
// error occurs, it returns ok = false, err = nil. If the check succeeds, it
// returns ok = true, err = nil.
func detectInterfaceOK(ctx context.Context, logger *log.Logger, source nodeapi.RuntimeApiLite, round uint64, contractEthAddr []byte, interfaceID []byte) (bool, bool, error) {
	result, err := callERC165SupportsInterface(ctx, source, round, contractEthAddr, interfaceID)
	if err != nil {
		if !errors.Is(err, EVMDeterministicError{}) {
			return false, false, err
		}
		logDeterministicError(logger, round, contractEthAddr, "ERC165", "supportsInterface", err,
			"interface_id", hex.EncodeToString(interfaceID),
		)
		return false, false, nil
	}
	return result, true, nil
}

func detectERC165(ctx context.Context, logger *log.Logger, source nodeapi.RuntimeApiLite, round uint64, contractEthAddr []byte) (bool, error) {
	// https://eips.ethereum.org/EIPS/eip-165#how-to-detect-if-a-contract-implements-erc-165
	supportsERC165, ok, err := detectInterfaceOK(ctx, logger, source, round, contractEthAddr, ERC165InterfaceID)
	if err != nil {
		return false, fmt.Errorf("checking ERC165: %w", err)
	}
	if !ok || !supportsERC165 {
		return false, nil
	}
	supportsInvalid, ok, err := detectInterfaceOK(ctx, logger, source, round, contractEthAddr, InvalidInterfaceID)
	if err != nil {
		return false, fmt.Errorf("checking invalid interface: %w", err)
	}
	if !ok || supportsInvalid {
		return false, nil
	}
	return true, nil
}

func detectInterface(ctx context.Context, logger *log.Logger, source nodeapi.RuntimeApiLite, round uint64, contractEthAddr []byte, interfaceID []byte) (bool, error) {
	result, ok, err := detectInterfaceOK(ctx, logger, source, round, contractEthAddr, interfaceID)
	if err != nil {
		return false, err
	}
	return ok && result, nil
}
