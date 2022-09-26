package common

import (
	"bytes"
	"fmt"

	"github.com/oasisprotocol/oasis-core/go/common/cbor"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/modules/accounts"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/modules/consensusaccounts"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/modules/core"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/modules/evm"
	sdkTypes "github.com/oasisprotocol/oasis-sdk/client-sdk/go/types"
)

type CallHandler struct {
	AccountsTransfer          func(body *accounts.Transfer) error
	ConsensusAccountsDeposit  func(body *consensusaccounts.Deposit) error
	ConsensusAccountsWithdraw func(body *consensusaccounts.Withdraw) error
	EvmCreate                 func(body *evm.Create, ok *[]byte) error
	EvmCall                   func(body *evm.Call, ok *[]byte) error
}

func VisitCall(call *sdkTypes.Call, result *sdkTypes.CallResult, handler *CallHandler) error {
	switch call.Method {
	case "accounts.Transfer":
		if handler.AccountsTransfer != nil {
			var body accounts.Transfer
			if err := cbor.Unmarshal(call.Body, &body); err != nil {
				return fmt.Errorf("unmarshal accounts transfer: %w", err)
			}
			if err := handler.AccountsTransfer(&body); err != nil {
				return fmt.Errorf("accounts transfer: %w", err)
			}
		}
	case "consensus.Deposit":
		if handler.ConsensusAccountsDeposit != nil {
			var body consensusaccounts.Deposit
			if err := cbor.Unmarshal(call.Body, &body); err != nil {
				return fmt.Errorf("unmarshal consensus accounts deposit: %w", err)
			}
			if err := handler.ConsensusAccountsDeposit(&body); err != nil {
				return fmt.Errorf("consensus accounts deposit: %w", err)
			}
		}
	case "consensus.Withdraw":
		if handler.ConsensusAccountsWithdraw != nil {
			var body consensusaccounts.Withdraw
			if err := cbor.Unmarshal(call.Body, &body); err != nil {
				return fmt.Errorf("unmarshal consensus accounts withdraw: %w", err)
			}
			if err := handler.ConsensusAccountsWithdraw(&body); err != nil {
				return fmt.Errorf("consensus accounts withdraw: %w", err)
			}
		}
	case "evm.Create":
		if handler.EvmCreate != nil {
			var body evm.Create
			if err := cbor.Unmarshal(call.Body, &body); err != nil {
				return fmt.Errorf("unmarshal evm create: %w", err)
			}
			var okP *[]byte
			if !result.IsUnknown() && result.IsSuccess() {
				var ok []byte
				if err := cbor.Unmarshal(result.Ok, &ok); err != nil {
					return fmt.Errorf("unmarshal evm create result: %w", err)
				}
				okP = &ok
			}
			if err := handler.EvmCreate(&body, okP); err != nil {
				return fmt.Errorf("evm create: %w", err)
			}
		}
	case "evm.Call":
		if handler.EvmCall != nil {
			var body evm.Call
			if err := cbor.Unmarshal(call.Body, &body); err != nil {
				return fmt.Errorf("unmarshal evm call: %w", err)
			}
			var okP *[]byte
			if !result.IsUnknown() && result.IsSuccess() {
				var ok []byte
				if err := cbor.Unmarshal(result.Ok, &ok); err != nil {
					return fmt.Errorf("unmarshal evm create result: %w", err)
				}
				okP = &ok
			}
			if err := handler.EvmCall(&body, okP); err != nil {
				return fmt.Errorf("evm call: %w", err)
			}
		}
	}
	return nil
}

type SdkEventHandler struct {
	Core              func(event *core.Event) error
	Accounts          func(event *accounts.Event) error
	ConsensusAccounts func(event *consensusaccounts.Event) error
	Evm               func(event *evm.Event) error
}

func VisitSdkEvent(event *sdkTypes.Event, handler *SdkEventHandler) error {
	if handler.Core != nil {
		coreEvents, err := core.DecodeEvent(event)
		if err != nil {
			return fmt.Errorf("decode core: %w", err)
		}
		for i, coreEvent := range coreEvents {
			coreEventCast, ok := coreEvent.(*core.Event)
			if !ok {
				return fmt.Errorf("decoded event %d could not cast to core.Event", i)
			}
			if err = handler.Core(coreEventCast); err != nil {
				return fmt.Errorf("decoded event %d core: %w", i, err)
			}
		}
	}
	if handler.Accounts != nil {
		accountEvents, err := accounts.DecodeEvent(event)
		if err != nil {
			return fmt.Errorf("decode accounts: %w", err)
		}
		for i, accountEvent := range accountEvents {
			accountEventCast, ok := accountEvent.(*accounts.Event)
			if !ok {
				return fmt.Errorf("decoded event %d could not cast to accounts.Event", i)
			}
			if err = handler.Accounts(accountEventCast); err != nil {
				return fmt.Errorf("decoded event %d accounts: %w", i, err)
			}
		}
	}
	if handler.ConsensusAccounts != nil {
		consensusAccountsEvents, err := consensusaccounts.DecodeEvent(event)
		if err != nil {
			return fmt.Errorf("decode consensus accounts: %w", err)
		}
		for i, consensusAccountsEvent := range consensusAccountsEvents {
			consensusAccountsEventCast, ok := consensusAccountsEvent.(*consensusaccounts.Event)
			if !ok {
				return fmt.Errorf("decoded event %d could not cast to consensusaccounts.Event", i)
			}
			if err = handler.ConsensusAccounts(consensusAccountsEventCast); err != nil {
				return fmt.Errorf("decoded event %d consensus accounts: %w", i, err)
			}
		}
	}
	if handler.Evm != nil {
		evmEvents, err := evm.DecodeEvent(event)
		if err != nil {
			return fmt.Errorf("decode evm: %w", err)
		}
		for i, evmEvent := range evmEvents {
			evmEventCast, ok := evmEvent.(*evm.Event)
			if !ok {
				return fmt.Errorf("decoded event %d could not cast to evm.Event", i)
			}
			if err = handler.Evm(evmEventCast); err != nil {
				return fmt.Errorf("decoded event %d evm: %w", i, err)
			}
		}
	}
	return nil
}

func VisitSdkEvents(events []*sdkTypes.Event, handler *SdkEventHandler) error {
	for i, event := range events {
		if err := VisitSdkEvent(event, handler); err != nil {
			return fmt.Errorf("event %d: %w", i, err)
		}
	}
	return nil
}

type EvmEventHandler struct {
	Erc20Transfer func(fromEthAddr []byte, toEthAddr []byte, amountU256 []byte) error
	Erc20Approval func(ownerEthAddr []byte, spenderEthAddr []byte, amountU256 []byte) error
}

func VisitEvmEvent(event *evm.Event, handler *EvmEventHandler) error {
	if len(event.Topics) >= 1 {
		switch {
		case bytes.Equal(event.Topics[0], TopicErc20Transfer) && len(event.Topics) == 3:
			if handler.Erc20Transfer != nil {
				if err := handler.Erc20Transfer(
					SliceEthAddress(event.Topics[1]),
					SliceEthAddress(event.Topics[2]),
					event.Data,
				); err != nil {
					return fmt.Errorf("erc20 transfer: %w", err)
				}
			}
		case bytes.Equal(event.Topics[0], TopicErc20Approval) && len(event.Topics) == 3:
			if handler.Erc20Approval != nil {
				if err := handler.Erc20Approval(
					SliceEthAddress(event.Topics[1]),
					SliceEthAddress(event.Topics[2]),
					event.Data,
				); err != nil {
					return fmt.Errorf("erc20 approval: %w", err)
				}
			}
		}
	}
	return nil
}
