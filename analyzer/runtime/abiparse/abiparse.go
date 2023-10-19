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

// ParseData parses call data into the method and its arguments.
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

func ParseResult(result []byte, method *abi.Method) ([]interface{}, error) {
	args, err := method.Outputs.Unpack(result)
	if err != nil {
		return nil, fmt.Errorf("method outputs Unpack: %w", err)
	}
	return args, nil
}

func parseOneSimple(data []byte, t abi.Type) (interface{}, error) {
	oneIn := abi.Arguments{abi.Argument{Type: t}}
	oneArg, err := oneIn.Unpack(data)
	if err != nil {
		return nil, err
	}
	return oneArg[0], nil
}

func ParseEvent(topics [][]byte, data []byte, contractABI *abi.ABI) (*abi.Event, []interface{}, error) {
	if len(topics) < 1 {
		return nil, nil, fmt.Errorf("topics (%d) too short to have event signature", len(topics))
	}
	topicsEC := make([]ethCommon.Hash, 0, len(topics))
	for _, topic := range topics {
		topicsEC = append(topicsEC, ethCommon.BytesToHash(topic))
	}
	event, err := contractABI.EventByID(topicsEC[0])
	if err != nil {
		return nil, nil, fmt.Errorf("contract ABI EventByID: %w", err)
	}

	argsFromData, err := event.Inputs.Unpack(data)
	if err != nil {
		return nil, nil, fmt.Errorf("event inputs Unpack: %w", err)
	}

	args := make([]interface{}, len(event.Inputs))
	nextTopicIndex := 0
	if !event.Anonymous {
		// Non-anonymous events use the hash of the event signature as the
		// first topic value. That topic is implicit; the value is not
		// associated with any argument, so skip over it.
		nextTopicIndex = 1
	}
	nextDataIndex := 0

	for i, in := range event.Inputs {
		if in.Indexed {
			switch in.Type.T {
			case abi.StringTy, abi.SliceTy, abi.ArrayTy, abi.TupleTy, abi.BytesTy:
				// https://docs.soliditylang.org/en/latest/abi-spec.html
				// > However, for all “complex” types or types of dynamic
				// > length, including all arrays, string, bytes and structs,
				// > EVENT_INDEXED_ARGS will contain the Keccak hash of a
				// > special in-place encoded value ..., rather than the
				// > encoded value directly.
				// Make a little wrapper to help viewers know it's only the hash.
				args[i] = map[string]ethCommon.Hash{"hash": topicsEC[nextTopicIndex]}
				nextTopicIndex++
			default:
				// > For all types of length at most 32 bytes, the
				// > EVENT_INDEXED_ARGS array contains the value directly,
				// > padded or sign-extended (for signed integers) to 32
				// > bytes, just as for regular ABI encoding.
				args[i], err = parseOneSimple(topics[nextTopicIndex], in.Type)
				if err != nil {
					return nil, nil, fmt.Errorf("event input %d topic %d Unpack: %w", i, nextTopicIndex, err)
				}
				nextTopicIndex++
			}
		} else {
			args[i] = argsFromData[nextDataIndex]
			nextDataIndex++
		}
	}
	return event, args, nil
}

func ParseError(data []byte, contractABI *abi.ABI) (*abi.Error, []interface{}, error) {
	if len(data) < 4 {
		return nil, nil, fmt.Errorf("data (%dB) too short to have error ID", len(data))
	}
	var errorID [4]byte
	copy(errorID[:], data[:4])
	packedArgs := data[4:]
	abiError, err := contractABI.ErrorByID(errorID)
	if err != nil {
		return nil, nil, fmt.Errorf("contract ABI ErrorById: %w", err)
	}
	args, err := abiError.Inputs.Unpack(packedArgs)
	if err != nil {
		return nil, nil, fmt.Errorf("error inputs Unpack: %w", err)
	}
	return abiError, args, nil
}
