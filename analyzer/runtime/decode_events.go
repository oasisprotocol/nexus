package runtime

import (
	"fmt"

	"github.com/oasisprotocol/oasis-core/go/common/cbor"
	"github.com/oasisprotocol/nexus/storage/oasis/nodeapi"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/modules/accounts"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/modules/consensusaccounts"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/modules/core"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/modules/evm"
)

func DecodeCoreEvent(event *nodeapi.RuntimeEvent) ([]core.Event, error) {
	if event.Module != core.ModuleName {
		return nil, nil
	}
	var events []core.Event
	switch event.Code {
	case core.GasUsedEventCode:
		var evs []*core.GasUsedEvent
		if err := unmarshalSingleOrArray(event.Value, &evs); err != nil {
			return nil, fmt.Errorf("decode core gas used event value: %w", err)
		}
		for _, ev := range evs {
			events = append(events, core.Event{GasUsed: ev})
		}
	default:
		return nil, fmt.Errorf("invalid core event code: %v", event.Code)
	}
	return events, nil
}

func DecodeAccountsEvent(event *nodeapi.RuntimeEvent) ([]accounts.Event, error) {
	if event.Module != accounts.ModuleName {
		return nil, nil
	}
	var events []accounts.Event
	switch event.Code {
	case accounts.TransferEventCode:
		var evs []*accounts.TransferEvent
		if err := unmarshalSingleOrArray(event.Value, &evs); err != nil {
			return nil, fmt.Errorf("decode account transfer event value: %w", err)
		}
		for _, ev := range evs {
			events = append(events, accounts.Event{Transfer: ev})
		}
	case accounts.BurnEventCode:
		var evs []*accounts.BurnEvent
		if err := unmarshalSingleOrArray(event.Value, &evs); err != nil {
			return nil, fmt.Errorf("decode account burn event value: %w", err)
		}
		for _, ev := range evs {
			events = append(events, accounts.Event{Burn: ev})
		}
	case accounts.MintEventCode:
		var evs []*accounts.MintEvent
		if err := unmarshalSingleOrArray(event.Value, &evs); err != nil {
			return nil, fmt.Errorf("decode account mint event value: %w", err)
		}
		for _, ev := range evs {
			events = append(events, accounts.Event{Mint: ev})
		}
	default:
		return nil, fmt.Errorf("invalid accounts event code: %v", event.Code)
	}
	return events, nil
}

func DecodeConsensusAccountsEvent(event *nodeapi.RuntimeEvent) ([]consensusaccounts.Event, error) {
	if event.Module != consensusaccounts.ModuleName {
		return nil, nil
	}
	var events []consensusaccounts.Event
	switch event.Code {
	case consensusaccounts.DepositEventCode:
		var evs []*consensusaccounts.DepositEvent
		if err := unmarshalSingleOrArray(event.Value, &evs); err != nil {
			return nil, fmt.Errorf("decode consensus accounts deposit event value: %w", err)
		}
		for _, ev := range evs {
			events = append(events, consensusaccounts.Event{Deposit: ev})
		}
	case consensusaccounts.WithdrawEventCode:
		var evs []*consensusaccounts.WithdrawEvent
		if err := unmarshalSingleOrArray(event.Value, &evs); err != nil {
			return nil, fmt.Errorf("decode consensus accounts withdraw event value: %w", err)
		}
		for _, ev := range evs {
			events = append(events, consensusaccounts.Event{Withdraw: ev})
		}
	default:
		return nil, fmt.Errorf("invalid consensus accounts event code: %v", event.Code)
	}
	return events, nil
}

func DecodeEVMEvent(event *nodeapi.RuntimeEvent) ([]evm.Event, error) {
	if event.Module != evm.ModuleName {
		return nil, nil
	}
	var events []evm.Event
	switch event.Code {
	case 1: // There's a single event code, and it doesn't have an associated constant.
		if err := unmarshalSingleOrArray(event.Value, &events); err != nil {
			return nil, fmt.Errorf("evm event value unmarshal failed: %w", err)
		}
	default:
		return nil, fmt.Errorf("invalid evm event code: %v", event.Code)
	}
	return events, nil
}

// unmarshalSingleOrArray tries to interpret `data` as an array of `T`.
// Failing that, it interprets it as a single `T`, and returns a slice
// of size 1 containing that `T`.
func unmarshalSingleOrArray[T any](data []byte, dst *[]T) error {
	if err := cbor.Unmarshal(data, &dst); err != nil {
		var single T
		if err := cbor.Unmarshal(data, &single); err != nil {
			return fmt.Errorf("unmarshal single or array: %w", err)
		}
		*dst = []T{single}
	}
	return nil
}
