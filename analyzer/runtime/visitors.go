package runtime

import (
	"bytes"
	"fmt"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/oasisprotocol/oasis-core/go/common/cbor"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/modules/accounts"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/modules/consensusaccounts"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/modules/core"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/modules/evm"
	sdkTypes "github.com/oasisprotocol/oasis-sdk/client-sdk/go/types"

	"github.com/oasisprotocol/nexus/analyzer/evmabi"
	evmCommon "github.com/oasisprotocol/nexus/analyzer/uncategorized"
	"github.com/oasisprotocol/nexus/storage/oasis/nodeapi"
)

type CallHandler struct {
	AccountsTransfer          func(body *accounts.Transfer) error
	ConsensusAccountsDeposit  func(body *consensusaccounts.Deposit) error
	ConsensusAccountsWithdraw func(body *consensusaccounts.Withdraw) error
	EVMCreate                 func(body *evm.Create, ok *[]byte) error
	EVMCall                   func(body *evm.Call, ok *[]byte) error
}

//nolint:nestif
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
		if handler.EVMCreate != nil {
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
			if err := handler.EVMCreate(&body, okP); err != nil {
				return fmt.Errorf("evm create: %w", err)
			}
		}
	case "evm.Call":
		if handler.EVMCall != nil {
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
			if err := handler.EVMCall(&body, okP); err != nil {
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
	EVM               func(event *evm.Event) error
}

func VisitSdkEvent(event *nodeapi.RuntimeEvent, handler *SdkEventHandler) error {
	if handler.Core != nil {
		coreEvents, err := DecodeCoreEvent(event)
		if err != nil {
			return fmt.Errorf("decode core: %w", err)
		}
		for i := range coreEvents {
			if err = handler.Core(&coreEvents[i]); err != nil {
				return fmt.Errorf("decoded event %d core: %w", i, err)
			}
		}
	}
	if handler.Accounts != nil {
		accountEvents, err := DecodeAccountsEvent(event)
		if err != nil {
			return fmt.Errorf("decode accounts: %w", err)
		}
		for i := range accountEvents {
			if err = handler.Accounts(&accountEvents[i]); err != nil {
				return fmt.Errorf("decoded event %d accounts: %w", i, err)
			}
		}
	}
	if handler.ConsensusAccounts != nil {
		consensusAccountsEvents, err := DecodeConsensusAccountsEvent(event)
		if err != nil {
			return fmt.Errorf("decode consensus accounts: %w", err)
		}
		for i := range consensusAccountsEvents {
			if err = handler.ConsensusAccounts(&consensusAccountsEvents[i]); err != nil {
				return fmt.Errorf("decoded event %d consensus accounts: %w", i, err)
			}
		}
	}
	if handler.EVM != nil {
		evmEvents, err := DecodeEVMEvent(event)
		if err != nil {
			return fmt.Errorf("decode evm: %w", err)
		}
		for i := range evmEvents {
			if err = handler.EVM(&evmEvents[i]); err != nil {
				return fmt.Errorf("decoded event %d evm: %w", i, err)
			}
		}
	}
	return nil
}

func VisitSdkEvents(events []nodeapi.RuntimeEvent, handler *SdkEventHandler) error {
	for i := range events {
		if err := VisitSdkEvent(&events[i], handler); err != nil {
			return fmt.Errorf("event %d: %w", i, err)
		}
	}
	return nil
}

type EVMEventHandler struct {
	ERC20Transfer        func(fromEthAddr []byte, toEthAddr []byte, amountU256 []byte) error
	ERC20Approval        func(ownerEthAddr []byte, spenderEthAddr []byte, amountU256 []byte) error
	ERC721Transfer       func(fromEthAddr []byte, toEthAddr []byte, tokenIDU256 []byte) error
	ERC721Approval       func(ownerEthAddr []byte, approvedEthAddr []byte, tokenIDU256 []byte) error
	ERC721ApprovalForAll func(ownerEthAddr []byte, operatorEthAddr []byte, approvedBool []byte) error
}

func eventMatches(evmEvent *evm.Event, ethEvent abi.Event) bool {
	if len(evmEvent.Topics) == 0 || !bytes.Equal(evmEvent.Topics[0], ethEvent.ID.Bytes()) {
		return false
	}
	// Thanks, ERC-721 and ERC-20 having the same signature for Transfer.
	// Check if it has the right number of topics.
	numTopics := 0
	if !ethEvent.Anonymous {
		numTopics++
	}
	for _, input := range ethEvent.Inputs {
		if input.Indexed {
			numTopics++
		}
	}
	return len(evmEvent.Topics) == numTopics
}

func VisitEVMEvent(event *evm.Event, handler *EVMEventHandler) error {
	switch {
	case eventMatches(event, evmabi.ERC20.Events["Transfer"]):
		if handler.ERC20Transfer != nil {
			if err := handler.ERC20Transfer(
				evmCommon.SliceEthAddress(event.Topics[1]),
				evmCommon.SliceEthAddress(event.Topics[2]),
				event.Data,
			); err != nil {
				return fmt.Errorf("erc20 transfer: %w", err)
			}
		}
	case eventMatches(event, evmabi.ERC20.Events["Approval"]):
		if handler.ERC20Approval != nil {
			if err := handler.ERC20Approval(
				evmCommon.SliceEthAddress(event.Topics[1]),
				evmCommon.SliceEthAddress(event.Topics[2]),
				event.Data,
			); err != nil {
				return fmt.Errorf("erc20 approval: %w", err)
			}
		}
	case eventMatches(event, evmabi.ERC721.Events["Transfer"]):
		if handler.ERC721Transfer != nil {
			if err := handler.ERC721Transfer(
				evmCommon.SliceEthAddress(event.Topics[1]),
				evmCommon.SliceEthAddress(event.Topics[2]),
				event.Topics[3],
			); err != nil {
				return fmt.Errorf("erc721 transfer: %w", err)
			}
		}
	case eventMatches(event, evmabi.ERC721.Events["Approval"]):
		if handler.ERC721Approval != nil {
			if err := handler.ERC721Approval(
				evmCommon.SliceEthAddress(event.Topics[1]),
				evmCommon.SliceEthAddress(event.Topics[2]),
				event.Topics[3],
			); err != nil {
				return fmt.Errorf("erc721 approval: %w", err)
			}
		}
	case eventMatches(event, evmabi.ERC721.Events["ApprovalForAll"]):
		if handler.ERC721ApprovalForAll != nil {
			if err := handler.ERC721ApprovalForAll(
				evmCommon.SliceEthAddress(event.Topics[1]),
				evmCommon.SliceEthAddress(event.Topics[2]),
				event.Data,
			); err != nil {
				return fmt.Errorf("erc721 approval for all: %w", err)
			}
		}
	}
	return nil
}
