package abiparse

import (
	"fmt"
	"reflect"

	"github.com/ethereum/go-ethereum/accounts/abi"
	ethCommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
)

// evmPreMarshal converts v to a type that gives us the JSON serialization that we like:
// - large integers are JSON strings instead of JSON numbers
// - byte array types are JSON strings of base64 instead of JSON arrays of numbers
// Contrived dot for godot linter: .
func evmPreMarshal(v interface{}, t abi.Type) interface{} {
	switch t.T {
	case abi.IntTy, abi.UintTy:
		if t.Size > 32 {
			return fmt.Sprint(v)
		}
	case abi.SliceTy, abi.ArrayTy:
		rv := reflect.ValueOf(v)
		slice := make([]interface{}, 0, rv.Len())
		for i := 0; i < rv.Len(); i++ {
			slice = append(slice, evmPreMarshal(rv.Index(i).Interface(), *t.Elem))
		}
		return slice
	case abi.TupleTy:
		rv := reflect.ValueOf(v)
		m := map[string]interface{}{}
		for i, fieldName := range t.TupleRawNames {
			m[fieldName] = evmPreMarshal(rv.Field(i).Interface(), *t.TupleElems[i])
		}
	case abi.FixedBytesTy, abi.FunctionTy:
		c := reflect.New(t.GetType()).Elem()
		c.Set(reflect.ValueOf(v))
		return hexutil.Encode(c.Bytes())
	case abi.BytesTy:
		return hexutil.Encode(reflect.ValueOf(v).Bytes())
	}
	return v
}

func ParseData(data []byte, contractABI *abi.ABI) (*abi.Method, []interface{}, error) {
	if len(data) < 4 {
		return nil, nil, fmt.Errorf("data (%dB) too short to have method ID", len(data))
	}
	methodID := data[:4]
	packedArgs := data[4:]
	method, err := contractABI.MethodById(methodID)
	if err != nil {
		return nil, nil, fmt.Errorf("contract ABI MethodById: %w", err)
	}
	args, err := method.Inputs.Unpack(packedArgs)
	if err != nil {
		return nil, nil, fmt.Errorf("method inputs Unpack: %w", err)
	}
	return method, args, nil
}

func ParseEvent(topics [][]byte, data []byte, contractABI *abi.ABI) (*abi.Event, []interface{}, error) {
	if len(topics) < 1 {
		return nil, nil, fmt.Errorf("topics (%d) too short to have event signature", len(topics))
	}
	event, err := contractABI.EventByID(ethCommon.BytesToHash(topics[0]))
	if err != nil {
		return nil, nil, fmt.Errorf("contract ABI EventByID: %w", err)
	}
	args, err := event.Inputs.Unpack(data)
	if err != nil {
		return nil, nil, fmt.Errorf("event inputs Unpack: %w", err)
	}
	return event, args, nil
}
