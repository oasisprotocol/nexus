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
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/modules/rofl"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/modules/roflmarket"
	sdkTypes "github.com/oasisprotocol/oasis-sdk/client-sdk/go/types"

	"github.com/oasisprotocol/nexus/analyzer/evmabi"
	"github.com/oasisprotocol/nexus/analyzer/runtime/abiparse"
	"github.com/oasisprotocol/nexus/analyzer/util"
	"github.com/oasisprotocol/nexus/common"
	"github.com/oasisprotocol/nexus/storage/oasis/nodeapi"
)

type CallHandler struct {
	AccountsTransfer               func(body *accounts.Transfer) error
	ConsensusAccountsDeposit       func(body *consensusaccounts.Deposit) error
	ConsensusAccountsWithdraw      func(body *consensusaccounts.Withdraw) error
	ConsensusAccountsDelegate      func(body *consensusaccounts.Delegate) error
	ConsensusAccountsUndelegate    func(body *consensusaccounts.Undelegate) error
	EVMCreate                      func(body *evm.Create, ok *[]byte) error
	EVMCall                        func(body *evm.Call, ok *[]byte) error
	RoflCreate                     func(body *rofl.Create) error
	RoflUpdate                     func(body *rofl.Update) error
	RoflRemove                     func(body *rofl.Remove) error
	RoflRegister                   func(body *rofl.Register) error
	RoflMarketProviderCreate       func(body *roflmarket.ProviderCreate) error
	RoflMarketProviderUpdate       func(body *roflmarket.ProviderUpdate) error
	RoflMarketProviderUpdateOffers func(body *roflmarket.ProviderUpdateOffers) error
	RoflMarketProviderRemove       func(body *roflmarket.ProviderRemove) error
	RoflMarketInstanceCreate       func(body *roflmarket.InstanceCreate) error
	RoflMarketInstanceTopUp        func(body *roflmarket.InstanceTopUp) error
	RoflMarketInstanceCancel       func(body *roflmarket.InstanceCancel) error
	RoflMarketInstanceExecuteCmds  func(body *roflmarket.InstanceExecuteCmds) error
	RoflMarketInstanceChangeAdmin  func(body *roflmarket.InstanceChangeAdmin) error
	UnknownMethod                  func(methodName string) error // Invoked for a tx call that doesn't map to any of the above method names.
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
					return fmt.Errorf("unmarshal evm call result: %w", err)
				}
				okP = &ok
			}
			if err := handler.EVMCall(&body, okP); err != nil {
				return fmt.Errorf("evm call: %w", err)
			}
		}
	case "rofl.Create":
		if handler.RoflCreate != nil {
			var body rofl.Create
			if err := cbor.Unmarshal(call.Body, &body); err != nil {
				return fmt.Errorf("unmarshal rofl create: %w", err)
			}
			if err := handler.RoflCreate(&body); err != nil {
				return fmt.Errorf("rofl create: %w", err)
			}
		}
	case "rofl.Update":
		if handler.RoflUpdate != nil {
			var body rofl.Update
			if err := cbor.Unmarshal(call.Body, &body); err != nil {
				return fmt.Errorf("unmarshal rofl update: %w", err)
			}
			if err := handler.RoflUpdate(&body); err != nil {
				return fmt.Errorf("rofl update: %w", err)
			}
		}
	case "rofl.Remove":
		if handler.RoflRemove != nil {
			var body rofl.Remove
			if err := cbor.Unmarshal(call.Body, &body); err != nil {
				return fmt.Errorf("unmarshal rofl remove: %w", err)
			}
			if err := handler.RoflRemove(&body); err != nil {
				return fmt.Errorf("rofl remove: %w", err)
			}
		}
	case "rofl.Register":
		if handler.RoflRegister != nil {
			var body rofl.Register
			if err := cbor.Unmarshal(call.Body, &body); err != nil {
				return fmt.Errorf("unmarshal rofl register: %w", err)
			}
			if err := handler.RoflRegister(&body); err != nil {
				return fmt.Errorf("rofl register: %w", err)
			}
		}
	case "roflmarket.ProviderCreate":
		if handler.RoflMarketProviderCreate != nil {
			var body roflmarket.ProviderCreate
			if err := cbor.Unmarshal(call.Body, &body); err != nil {
				return fmt.Errorf("unmarshal rofl market provider create: %w", err)
			}
			if err := handler.RoflMarketProviderCreate(&body); err != nil {
				return fmt.Errorf("rofl market provider create: %w", err)
			}
		}
	case "roflmarket.ProviderUpdate":
		if handler.RoflMarketProviderUpdate != nil {
			var body roflmarket.ProviderUpdate
			if err := cbor.Unmarshal(call.Body, &body); err != nil {
				return fmt.Errorf("unmarshal rofl market provider update: %w", err)
			}
			if err := handler.RoflMarketProviderUpdate(&body); err != nil {
				return fmt.Errorf("rofl market provider update: %w", err)
			}
		}
	case "roflmarket.ProviderUpdateOffers":
		if handler.RoflMarketProviderUpdateOffers != nil {
			var body roflmarket.ProviderUpdateOffers
			if err := cbor.Unmarshal(call.Body, &body); err != nil {
				return fmt.Errorf("unmarshal rofl market provider update offers: %w", err)
			}
			if err := handler.RoflMarketProviderUpdateOffers(&body); err != nil {
				return fmt.Errorf("rofl market provider update offers: %w", err)
			}
		}
	case "roflmarket.ProviderRemove":
		if handler.RoflMarketProviderRemove != nil {
			var body roflmarket.ProviderRemove
			if err := cbor.Unmarshal(call.Body, &body); err != nil {
				return fmt.Errorf("unmarshal rofl market provider remove: %w", err)
			}
			if err := handler.RoflMarketProviderRemove(&body); err != nil {
				return fmt.Errorf("rofl market provider remove: %w", err)
			}
		}
	case "roflmarket.InstanceCreate":
		if handler.RoflMarketInstanceCreate != nil {
			var body roflmarket.InstanceCreate
			if err := cbor.Unmarshal(call.Body, &body); err != nil {
				return fmt.Errorf("unmarshal rofl market instance create: %w", err)
			}
			if err := handler.RoflMarketInstanceCreate(&body); err != nil {
				return fmt.Errorf("rofl market instance create: %w", err)
			}
		}
	case "roflmarket.InstanceTopUp":
		if handler.RoflMarketInstanceTopUp != nil {
			var body roflmarket.InstanceTopUp
			if err := cbor.Unmarshal(call.Body, &body); err != nil {
				return fmt.Errorf("unmarshal rofl market instance top up: %w", err)
			}
			if err := handler.RoflMarketInstanceTopUp(&body); err != nil {
				return fmt.Errorf("rofl market instance top up: %w", err)
			}
		}
	case "roflmarket.InstanceCancel":
		if handler.RoflMarketInstanceCancel != nil {
			var body roflmarket.InstanceCancel
			if err := cbor.Unmarshal(call.Body, &body); err != nil {
				return fmt.Errorf("unmarshal rofl market instance cancel: %w", err)
			}
			if err := handler.RoflMarketInstanceCancel(&body); err != nil {
				return fmt.Errorf("rofl market instance cancel: %w", err)
			}
		}
	case "roflmarket.InstanceExecuteCmds":
		if handler.RoflMarketInstanceExecuteCmds != nil {
			var body roflmarket.InstanceExecuteCmds
			if err := cbor.Unmarshal(call.Body, &body); err != nil {
				return fmt.Errorf("unmarshal rofl market instance execute cmds: %w", err)
			}
			if err := handler.RoflMarketInstanceExecuteCmds(&body); err != nil {
				return fmt.Errorf("rofl market instance execute cmds: %w", err)
			}
		}
	case "roflmarket.InstanceChangeAdmin":
		if handler.RoflMarketInstanceChangeAdmin != nil {
			var body roflmarket.InstanceChangeAdmin
			if err := cbor.Unmarshal(call.Body, &body); err != nil {
				return fmt.Errorf("unmarshal rofl market instance change admin: %w", err)
			}
			if err := handler.RoflMarketInstanceChangeAdmin(&body); err != nil {
				return fmt.Errorf("rofl market instance change admin: %w", err)
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
	Core              func(event *core.Event, eventTxHash *string, eventIdx int) error
	Accounts          func(event *accounts.Event, eventTxHash *string, eventIdx int) error
	ConsensusAccounts func(event *consensusaccounts.Event, eventTxHash *string, eventIdx int) error
	EVM               func(event *evm.Event, eventTxHash *string, eventIdx int) error
	Rofl              func(event *rofl.Event, eventTxHash *string, eventIdx int) error
	RoflMarket        func(event *roflmarket.Event, eventTxHash *string, eventIdx int) error
}

func VisitSdkEvent(event *nodeapi.RuntimeEvent, handler *SdkEventHandler, eventIdx int) (int, error) {
	var txHash *string
	if event.TxHash != nil && event.TxHash.String() != util.ZeroTxHash {
		txHash = common.Ptr(event.TxHash.String())
	}

	if handler.Core != nil {
		coreEvents, err := DecodeCoreEvent(event)
		if err != nil {
			return 0, fmt.Errorf("decode core: %w", err)
		}
		for i := range coreEvents {
			if err = handler.Core(&coreEvents[i], txHash, eventIdx+i); err != nil {
				return 0, fmt.Errorf("decoded event %d core: %w", i, err)
			}
			eventIdx++
		}
	}
	if handler.Accounts != nil {
		accountEvents, err := DecodeAccountsEvent(event)
		if err != nil {
			return 0, fmt.Errorf("decode accounts: %w", err)
		}
		for i := range accountEvents {
			if err = handler.Accounts(&accountEvents[i], txHash, eventIdx); err != nil {
				return 0, fmt.Errorf("decoded event %d accounts: %w", i, err)
			}
			eventIdx++
		}
	}
	if handler.ConsensusAccounts != nil {
		consensusAccountsEvents, err := DecodeConsensusAccountsEvent(event)
		if err != nil {
			return 0, fmt.Errorf("decode consensus accounts: %w", err)
		}
		for i := range consensusAccountsEvents {
			if err = handler.ConsensusAccounts(&consensusAccountsEvents[i], txHash, eventIdx); err != nil {
				return 0, fmt.Errorf("decoded event %d consensus accounts: %w", i, err)
			}
			eventIdx++
		}
	}
	if handler.EVM != nil {
		evmEvents, err := DecodeEVMEvent(event)
		if err != nil {
			return 0, fmt.Errorf("decode evm: %w", err)
		}
		for i := range evmEvents {
			if err = handler.EVM(&evmEvents[i], txHash, eventIdx); err != nil {
				return 0, fmt.Errorf("decoded event %d evm: %w", i, err)
			}
			eventIdx++
		}
	}
	if handler.Rofl != nil {
		roflEvents, err := DecodeRoflEvent(event)
		if err != nil {
			return 0, fmt.Errorf("decode rofl: %w", err)
		}
		for i := range roflEvents {
			if err = handler.Rofl(&roflEvents[i], txHash, eventIdx); err != nil {
				return 0, fmt.Errorf("decoded event %d rofl: %w", i, err)
			}
			eventIdx++
		}
	}
	if handler.RoflMarket != nil {
		roflMarketEvents, err := DecodeRoflMarketEvent(event)
		if err != nil {
			return 0, fmt.Errorf("decode rofl market: %w", err)
		}
		for i := range roflMarketEvents {
			if err = handler.RoflMarket(&roflMarketEvents[i], txHash, eventIdx); err != nil {
				return 0, fmt.Errorf("decoded event %d rofl market: %w", i, err)
			}
			eventIdx++
		}
	}
	return eventIdx, nil
}

func VisitSdkEvents(events []nodeapi.RuntimeEvent, handler *SdkEventHandler) error {
	var err error
	var idx int
	for i := range events {
		if idx, err = VisitSdkEvent(&events[i], handler, idx); err != nil {
			return fmt.Errorf("event %d: %w; raw event: %+v", i, err, events[i])
		}
	}
	return nil
}

type EVMEventHandler struct {
	ERC20Transfer                func(from ethCommon.Address, to ethCommon.Address, value *big.Int) error
	ERC20Approval                func(owner ethCommon.Address, spender ethCommon.Address, value *big.Int) error
	ERC721Transfer               func(from ethCommon.Address, to ethCommon.Address, tokenID *big.Int) error
	ERC721Approval               func(owner ethCommon.Address, approved ethCommon.Address, tokenID *big.Int) error
	ERC721ApprovalForAll         func(owner ethCommon.Address, operator ethCommon.Address, approved bool) error
	IUniswapV2FactoryPairCreated func(token0 ethCommon.Address, token1 ethCommon.Address, pair ethCommon.Address, allPairsLength *big.Int) error
	IUniswapV2PairMint           func(sender ethCommon.Address, amount0 *big.Int, amount1 *big.Int) error
	IUniswapV2PairBurn           func(sender ethCommon.Address, amount0 *big.Int, amount1 *big.Int, to ethCommon.Address) error
	IUniswapV2PairSwap           func(sender ethCommon.Address, amount0In *big.Int, amount1In *big.Int, amount0Out *big.Int, amount1Out *big.Int, to ethCommon.Address) error
	IUniswapV2PairSync           func(reserve0 *big.Int, reserve1 *big.Int) error
	// `owner` wrapped/deposited runtime's native token (ROSE) into the wrapper contract (creating WROSE).
	// `value` ROSE is transferred from `owner` to the wrapper (= event-emitting contract). Caller's WROSE balance increases by `value`.
	WROSEDeposit func(owner ethCommon.Address, value *big.Int) error
	// `owner` unwrapped/withdrew runtime's native token (ROSE) into the wrapper contract (burning WROSE).
	// Caller's WROSE balance decreases by `value`. `value` ROSE is transferred from the wrapper (= event-emitting contract) to `owner`.
	WROSEWithdrawal func(owner ethCommon.Address, value *big.Int) error
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

func VisitEVMEvent(event *evm.Event, handler *EVMEventHandler) error { //nolint:gocyclo
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
				return fmt.Errorf("handle erc20 transfer: %w", err)
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
				return fmt.Errorf("handle erc20 approval: %w", err)
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
				return fmt.Errorf("handle erc721 transfer: %w", err)
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
				return fmt.Errorf("handle erc721 approval: %w", err)
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
				return fmt.Errorf("handle erc721 approval for all: %w", err)
			}
		}
	case eventMatches(event, evmabi.IUniswapV2Factory.Events["PairCreated"]):
		if handler.IUniswapV2FactoryPairCreated != nil {
			_, args, err := abiparse.ParseEvent(event.Topics, event.Data, evmabi.IUniswapV2Factory)
			if err != nil {
				return fmt.Errorf("parse uniswap v2 factory pair created: %w", err)
			}
			if err = handler.IUniswapV2FactoryPairCreated(
				args[0].(ethCommon.Address),
				args[1].(ethCommon.Address),
				args[2].(ethCommon.Address),
				args[3].(*big.Int),
			); err != nil {
				return fmt.Errorf("handle uniswap v2 factory pair created: %w", err)
			}
		}
	case eventMatches(event, evmabi.IUniswapV2Pair.Events["Mint"]):
		if handler.IUniswapV2PairMint != nil {
			_, args, err := abiparse.ParseEvent(event.Topics, event.Data, evmabi.IUniswapV2Pair)
			if err != nil {
				return fmt.Errorf("parse uniswap v2 pair mint: %w", err)
			}
			if err = handler.IUniswapV2PairMint(
				args[0].(ethCommon.Address),
				args[1].(*big.Int),
				args[2].(*big.Int),
			); err != nil {
				return fmt.Errorf("handle uniswap v2 pair mint: %w", err)
			}
		}
	case eventMatches(event, evmabi.IUniswapV2Pair.Events["Burn"]):
		if handler.IUniswapV2PairBurn != nil {
			_, args, err := abiparse.ParseEvent(event.Topics, event.Data, evmabi.IUniswapV2Pair)
			if err != nil {
				return fmt.Errorf("parse uniswap v2 pair burn: %w", err)
			}
			if err = handler.IUniswapV2PairBurn(
				args[0].(ethCommon.Address),
				args[1].(*big.Int),
				args[2].(*big.Int),
				args[3].(ethCommon.Address),
			); err != nil {
				return fmt.Errorf("handle uniswap v2 pair burn: %w", err)
			}
		}
	case eventMatches(event, evmabi.IUniswapV2Pair.Events["Swap"]):
		if handler.IUniswapV2PairSwap != nil {
			_, args, err := abiparse.ParseEvent(event.Topics, event.Data, evmabi.IUniswapV2Pair)
			if err != nil {
				return fmt.Errorf("parse uniswap v2 pair swap: %w", err)
			}
			if err = handler.IUniswapV2PairSwap(
				args[0].(ethCommon.Address),
				args[1].(*big.Int),
				args[2].(*big.Int),
				args[3].(*big.Int),
				args[4].(*big.Int),
				args[5].(ethCommon.Address),
			); err != nil {
				return fmt.Errorf("handle uniswap v2 pair swap: %w", err)
			}
		}
	case eventMatches(event, evmabi.IUniswapV2Pair.Events["Sync"]):
		if handler.IUniswapV2PairSync != nil {
			_, args, err := abiparse.ParseEvent(event.Topics, event.Data, evmabi.IUniswapV2Pair)
			if err != nil {
				return fmt.Errorf("parse uniswap v2 pair sync: %w", err)
			}
			if err = handler.IUniswapV2PairSync(
				args[0].(*big.Int),
				args[1].(*big.Int),
			); err != nil {
				return fmt.Errorf("handle uniswap v2 pair sync: %w", err)
			}
		}
	// Signature: 0xe1fffcc4923d04b559f4d29a8bfc6cda04eb5b0d3c460751c2402c5c5cc9109c (hex) or 4f/8xJI9BLVZ9NKai/xs2gTrWw08RgdRwkAsXFzJEJw= (base64)
	case eventMatches(event, evmabi.WROSE.Events["Deposit"]):
		if handler.WROSEDeposit != nil {
			_, args, err := abiparse.ParseEvent(event.Topics, event.Data, evmabi.WROSE)
			if err != nil {
				return fmt.Errorf("parse wrose deposit: %w", err)
			}
			if err = handler.WROSEDeposit(
				args[0].(ethCommon.Address),
				args[1].(*big.Int),
			); err != nil {
				return fmt.Errorf("handle wrose deposit: %w", err)
			}
		}
	// Signature: 0x7fcf532c15f0a6db0bd6d0e038bea71d30d808c7d98cb3bf7268a95bf5081b65 (hex) or f89TLBXwptsL1tDgOL6nHTDYCMfZjLO/cmipW/UIG2U= (base64)
	case eventMatches(event, evmabi.WROSE.Events["Withdrawal"]):
		if handler.WROSEWithdrawal != nil {
			_, args, err := abiparse.ParseEvent(event.Topics, event.Data, evmabi.WROSE)
			if err != nil {
				return fmt.Errorf("parse wrose withdrawal: %w", err)
			}
			if err = handler.WROSEWithdrawal(
				args[0].(ethCommon.Address),
				args[1].(*big.Int),
			); err != nil {
				return fmt.Errorf("handle wrose withdrawal: %w", err)
			}
		}
	}
	return nil
}
