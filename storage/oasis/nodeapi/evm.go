package nodeapi

import "github.com/oasisprotocol/oasis-core/go/common/errors"

// FallibleResponse stores the response to a node query.
// Currently only used by EvmSimulateCall. If we use this for
// other node queries with different return types, consider
// adding generics.
type FallibleResponse struct {
	Ok               []byte
	DeterministicErr *DeterministicError
}

// DeterministicError mirrors known errors that may be returned by the node. This
// struct is necessary because cbor serde requires a concrete type, not just
// the `error` interface.
type DeterministicError struct {
	msg string
}

func (e DeterministicError) Error() string {
	return e.msg
}

// TODO: can we move this to oasis-sdk/client-sdk/go/modules/evm?
const EVMModuleName = "evm"

var (
	// Known deterministic errors that may be cached
	// https://github.com/oasisprotocol/oasis-sdk/blob/runtime-sdk/v0.2.0/runtime-sdk/modules/evm/src/lib.rs#L123
	ErrSdkEVMExecutionFailed = errors.New(EVMModuleName, 2, "execution failed")
	// https://github.com/oasisprotocol/oasis-sdk/blob/runtime-sdk/v0.2.0/runtime-sdk/modules/evm/src/lib.rs#L147
	ErrSdkEVMReverted = errors.New(EVMModuleName, 8, "reverted")
)
