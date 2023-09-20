package runtime

import (
	"bytes"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi"
	ethCommon "github.com/ethereum/go-ethereum/common"
	"github.com/oasisprotocol/oasis-core/go/common/cbor"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/modules/accounts"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/modules/consensusaccounts"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/modules/core"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/modules/evm"
	sdkTypes "github.com/oasisprotocol/oasis-sdk/client-sdk/go/types"

	"github.com/oasisprotocol/nexus/analyzer/evmabi"
	"github.com/oasisprotocol/nexus/analyzer/runtime/abiparse"
	"github.com/oasisprotocol/nexus/storage/oasis/nodeapi"
)

type CallHandler struct {
	AccountsTransfer            func(body *accounts.Transfer) error
	ConsensusAccountsDeposit    func(body *consensusaccounts.Deposit) error
	ConsensusAccountsWithdraw   func(body *consensusaccounts.Withdraw) error
	ConsensusAccountsDelegate   func(body *consensusaccounts.Delegate) error
	ConsensusAccountsUndelegate func(body *consensusaccounts.Undelegate) error
	EVMCreate                   func(body *evm.Create, ok *[]byte) error
	EVMCall                     func(body *evm.Call, ok *[]byte) error
	UnknownMethod               func(methodName string) error // Invoked for a tx call that doesn't map to any of the above method names.
}

//nolint:nestif,gocyclo
func VisitCall(call *sdkTypes.Call, result *sdkTypes.CallResult, handler *CallHandler) error {
	// List of methods: See each of the SDK modules, example for consensus_accounts:
	//   https://github.com/oasisprotocol/oasis-sdk/blob/client-sdk%2Fgo%2Fv0.6.0/client-sdk/go/modules/consensusaccounts/consensus_accounts.go#L16-L20
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
	case "consensus.Delegate":
		if handler.ConsensusAccountsDelegate != nil {
			var body consensusaccounts.Delegate
			if err := cbor.Unmarshal(call.Body, &body); err != nil {
				return fmt.Errorf("unmarshal consensus accounts delegate: %w", err)
			}
			if err := handler.ConsensusAccountsDelegate(&body); err != nil {
				return fmt.Errorf("consensus accounts delegate: %w", err)
			}
		}
	case "consensus.Undelegate":
		if handler.ConsensusAccountsUndelegate != nil {
			var body consensusaccounts.Undelegate
			if err := cbor.Unmarshal(call.Body, &body); err != nil {
				return fmt.Errorf("unmarshal consensus accounts undelegate: %w", err)
			}
			if err := handler.ConsensusAccountsUndelegate(&body); err != nil {
				return fmt.Errorf("consensus accounts undelegate: %w", err)
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
	default:
		if handler.UnknownMethod != nil {
			return handler.UnknownMethod(string(call.Method))
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
			return fmt.Errorf("event %d: %w; raw event: %+v", i, err, events[i])
		}
	}
	return nil
}

type EVMEventHandler struct {
	ERC20Transfer        func(from ethCommon.Address, to ethCommon.Address, amount *big.Int) error
	ERC20Approval        func(owner ethCommon.Address, spender ethCommon.Address, amount *big.Int) error
	ERC721Transfer       func(from ethCommon.Address, to ethCommon.Address, tokenID *big.Int) error
	ERC721Approval       func(owner ethCommon.Address, approved ethCommon.Address, tokenID *big.Int) error
	ERC721ApprovalForAll func(owner ethCommon.Address, operator ethCommon.Address, approved bool) error
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
			_, args, err := abiparse.ParseEvent(event.Topics, event.Data, evmabi.ERC20)
			if err != nil {
				return fmt.Errorf("parse erc20 transfer: %w", err)
			}
			if err = handler.ERC20Transfer(
				args[0].(ethCommon.Address),
				args[1].(ethCommon.Address),
				args[2].(*big.Int),
			); err != nil {
				return fmt.Errorf("erc20 transfer: %w", err)
			}
		}
	case eventMatches(event, evmabi.ERC20.Events["Approval"]):
		if handler.ERC20Approval != nil {
			_, args, err := abiparse.ParseEvent(event.Topics, event.Data, evmabi.ERC20)
			if err != nil {
				return fmt.Errorf("parse erc20 approval: %w", err)
			}
			if err = handler.ERC20Approval(
				args[0].(ethCommon.Address),
				args[1].(ethCommon.Address),
				args[2].(*big.Int),
			); err != nil {
				return fmt.Errorf("erc20 approval: %w", err)
			}
		}
	case eventMatches(event, evmabi.ERC721.Events["Transfer"]):
		if handler.ERC721Transfer != nil {
			_, args, err := abiparse.ParseEvent(event.Topics, event.Data, evmabi.ERC721)
			if err != nil {
				return fmt.Errorf("parse erc721 transfer: %w", err)
			}
			if err = handler.ERC721Transfer(
				args[0].(ethCommon.Address),
				args[1].(ethCommon.Address),
				args[2].(*big.Int),
			); err != nil {
				return fmt.Errorf("erc721 transfer: %w", err)
			}
		}
	case eventMatches(event, evmabi.ERC721.Events["Approval"]):
		if handler.ERC721Approval != nil {
			_, args, err := abiparse.ParseEvent(event.Topics, event.Data, evmabi.ERC721)
			if err != nil {
				return fmt.Errorf("parse erc721 approval: %w", err)
			}
			if err = handler.ERC721Approval(
				args[0].(ethCommon.Address),
				args[1].(ethCommon.Address),
				args[2].(*big.Int),
			); err != nil {
				return fmt.Errorf("erc721 approval: %w", err)
			}
		}
	case eventMatches(event, evmabi.ERC721.Events["ApprovalForAll"]):
		if handler.ERC721ApprovalForAll != nil {
			_, args, err := abiparse.ParseEvent(event.Topics, event.Data, evmabi.ERC721)
			if err != nil {
				return fmt.Errorf("parse erc721 approval for all: %w", err)
			}
			if err = handler.ERC721ApprovalForAll(
				args[0].(ethCommon.Address),
				args[1].(ethCommon.Address),
				args[2].(bool),
			); err != nil {
				return fmt.Errorf("erc721 approval for all: %w", err)
			}
		}
	}
	return nil
}
