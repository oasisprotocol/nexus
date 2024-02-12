package evmabibackfill

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"

	ethCommon "github.com/ethereum/go-ethereum/common"
	sdkEVM "github.com/oasisprotocol/oasis-sdk/client-sdk/go/modules/evm"

	"github.com/oasisprotocol/nexus/analyzer"
	"github.com/oasisprotocol/nexus/analyzer/evmabi"
	"github.com/oasisprotocol/nexus/analyzer/item"
	"github.com/oasisprotocol/nexus/analyzer/queries"
	"github.com/oasisprotocol/nexus/analyzer/runtime"
	apiTypes "github.com/oasisprotocol/nexus/api/v1/types"
	"github.com/oasisprotocol/nexus/common"
	"github.com/oasisprotocol/nexus/config"
	"github.com/oasisprotocol/nexus/log"
	"github.com/oasisprotocol/nexus/storage"
)

const (
	evmAbiAnalyzerPrefix = "evm_abi_"
)

type processor struct {
	runtime common.Runtime
	target  storage.TargetStorage
	logger  *log.Logger

	currRound uint64
}

var _ item.ItemProcessor[*abiEncodedItem] = (*processor)(nil)

type abiEncodedTx struct {
	TxHash         string
	TxData         []byte
	TxRevertReason *string
}

type abiEncodedEvent struct {
	Round     uint64
	TxIndex   *int
	EventBody sdkEVM.Event
}

type abiEncodedItem struct {
	Tx           *abiEncodedTx
	Event        *abiEncodedEvent
	ContractAddr string
	Abi          json.RawMessage
}

type abiEncodedArg struct {
	Name    string      `json:"name"`
	EvmType string      `json:"evm_type"`
	Value   interface{} `json:"value"`
}

func NewAnalyzer(
	runtime common.Runtime,
	cfg config.ItemBasedAnalyzerConfig,
	target storage.TargetStorage,
	logger *log.Logger,
) (analyzer.Analyzer, error) {
	logger = logger.With("analyzer", evmAbiAnalyzerPrefix+runtime)
	p := &processor{
		runtime,
		target,
		logger,
		0,
	}
	return item.NewAnalyzer[*abiEncodedItem](
		evmAbiAnalyzerPrefix+string(runtime),
		cfg,
		p,
		target,
		logger,
	)
}

func (p *processor) GetItems(ctx context.Context, limit uint64) ([]*abiEncodedItem, error) {
	// There are two types of data we process using a contract abi: transactions and events.
	// Within a transaction, we process the call data and the revert reason. Since they are
	// colocated in the same table we can fetch them using a single query.
	var items []*abiEncodedItem
	eventRows, err := p.target.Query(ctx, queries.RuntimeEvmEvents, p.runtime, p.currRound, limit)
	if err != nil {
		return nil, fmt.Errorf("querying evs: %w", err)
	}
	defer eventRows.Close()
	firstEv := true
	for eventRows.Next() {
		var ev abiEncodedEvent
		var item abiEncodedItem
		item.Event = &ev
		if err = eventRows.Scan(
			&ev.Round,
			&ev.TxIndex,
			&ev.EventBody,
		); err != nil {
			return nil, fmt.Errorf("scanning verified contract event: %w", err)
		}
		if firstEv {
			p.currRound = ev.Round - 100 // margin of safety
			firstEv = false
		}
		items = append(items, &item)
	}
	return items, nil
}

// Attempts to parse the raw event body into the event name, args, and signature
// as defined by the abi of the contract that emitted this event.
func (p *processor) parseEvent(ev *abiEncodedEvent) (*string, []*abiEncodedArg, *ethCommon.Hash) {
	var evmLogName *string
	var evmLogSignature *ethCommon.Hash
	var evmLogParams []*abiEncodedArg
	if err := runtime.VisitEVMEvent(&ev.EventBody, &runtime.EVMEventHandler{
		ERC20Transfer: func(fromECAddr ethCommon.Address, toECAddr ethCommon.Address, value *big.Int) error {
			evmLogName = common.Ptr(apiTypes.Erc20Transfer)
			evmLogSignature = common.Ptr(ethCommon.BytesToHash(ev.EventBody.Topics[0]))
			evmLogParams = []*abiEncodedArg{
				{
					Name:    "from",
					EvmType: "address",
					Value:   fromECAddr,
				},
				{
					Name:    "to",
					EvmType: "address",
					Value:   toECAddr,
				},
				{
					Name:    "value",
					EvmType: "uint256",
					// JSON supports encoding big integers, but many clients (javascript, jq, etc.)
					// will incorrectly parse them as floats. So we encode uint256 as a string instead.
					Value: value.String(),
				},
			}
			return nil
		},
		ERC20Approval: func(ownerECAddr ethCommon.Address, spenderECAddr ethCommon.Address, value *big.Int) error {
			evmLogName = common.Ptr(apiTypes.Erc20Approval)
			evmLogSignature = common.Ptr(ethCommon.BytesToHash(ev.EventBody.Topics[0]))
			evmLogParams = []*abiEncodedArg{
				{
					Name:    "owner",
					EvmType: "address",
					Value:   ownerECAddr,
				},
				{
					Name:    "spender",
					EvmType: "address",
					Value:   spenderECAddr,
				},
				{
					Name:    "value",
					EvmType: "uint256",
					// JSON supports encoding big integers, but many clients (javascript, jq, etc.)
					// will incorrectly parse them as floats. So we encode uint256 as a string instead.
					Value: value.String(),
				},
			}
			return nil
		},
		ERC721Transfer: func(fromECAddr ethCommon.Address, toECAddr ethCommon.Address, tokenID *big.Int) error {
			evmLogName = common.Ptr(evmabi.ERC721.Events["Transfer"].Name)
			evmLogSignature = common.Ptr(ethCommon.BytesToHash(ev.EventBody.Topics[0]))
			evmLogParams = []*abiEncodedArg{
				{
					Name:    "from",
					EvmType: "address",
					Value:   fromECAddr,
				},
				{
					Name:    "to",
					EvmType: "address",
					Value:   toECAddr,
				},
				{
					Name:    "tokenID",
					EvmType: "uint256",
					// JSON supports encoding big integers, but many clients (javascript, jq, etc.)
					// will incorrectly parse them as floats. So we encode uint256 as a string instead.
					Value: tokenID.String(),
				},
			}
			return nil
		},
		ERC721Approval: func(ownerECAddr ethCommon.Address, approvedECAddr ethCommon.Address, tokenID *big.Int) error {
			evmLogName = common.Ptr(evmabi.ERC721.Events["Approval"].Name)
			evmLogSignature = common.Ptr(ethCommon.BytesToHash(ev.EventBody.Topics[0]))
			evmLogParams = []*abiEncodedArg{
				{
					Name:    "owner",
					EvmType: "address",
					Value:   ownerECAddr,
				},
				{
					Name:    "approved",
					EvmType: "address",
					Value:   approvedECAddr,
				},
				{
					Name:    "tokenID",
					EvmType: "uint256",
					// JSON supports encoding big integers, but many clients (javascript, jq, etc.)
					// will incorrectly parse them as floats. So we encode uint256 as a string instead.
					Value: tokenID.String(),
				},
			}
			return nil
		},
		ERC721ApprovalForAll: func(ownerECAddr ethCommon.Address, operatorECAddr ethCommon.Address, approved bool) error {
			evmLogName = common.Ptr(evmabi.ERC721.Events["ApprovalForAll"].Name)
			evmLogSignature = common.Ptr(ethCommon.BytesToHash(ev.EventBody.Topics[0]))
			evmLogParams = []*abiEncodedArg{
				{
					Name:    "owner",
					EvmType: "address",
					Value:   ownerECAddr,
				},
				{
					Name:    "operator",
					EvmType: "address",
					Value:   operatorECAddr,
				},
				{
					Name:    "approved",
					EvmType: "bool",
					Value:   approved,
				},
			}
			return nil
		},
		WROSEDeposit: func(ownerECAddr ethCommon.Address, amount *big.Int) error {
			evmLogName = common.Ptr(evmabi.WROSE.Events["Deposit"].Name)
			evmLogSignature = common.Ptr(ethCommon.BytesToHash(ev.EventBody.Topics[0]))
			evmLogParams = []*abiEncodedArg{
				{
					Name:    "dst",
					EvmType: "address",
					Value:   ownerECAddr,
				},
				{
					Name:    "wad",
					EvmType: "uint256",
					// JSON supports encoding big integers, but many clients (javascript, jq, etc.)
					// will incorrectly parse them as floats. So we encode uint256 as a string instead.
					Value: amount.String(),
				},
			}
			return nil
		},
		WROSEWithdrawal: func(ownerECAddr ethCommon.Address, amount *big.Int) error {
			evmLogName = common.Ptr(evmabi.WROSE.Events["Withdrawal"].Name)
			evmLogSignature = common.Ptr(ethCommon.BytesToHash(ev.EventBody.Topics[0]))
			evmLogParams = []*abiEncodedArg{
				{
					Name:    "src",
					EvmType: "address",
					Value:   ownerECAddr,
				},
				{
					Name:    "wad",
					EvmType: "uint256",
					// JSON supports encoding big integers, but many clients (javascript, jq, etc.)
					// will incorrectly parse them as floats. So we encode uint256 as a string instead.
					Value: amount.String(),
				},
			}
			return nil
		},
	}); err != nil {
		p.logger.Error("error processing event", "round", ev.Round, "tx_index", ev.TxIndex, "err", err)
		return nil, nil, nil
	}

	return evmLogName, evmLogParams, evmLogSignature
}

func (p *processor) ProcessItem(ctx context.Context, batch *storage.QueryBatch, item *abiEncodedItem) error {
	if item.Event != nil {
		eventName, eventArgs, eventSig := p.parseEvent(item.Event)
		batch.Queue(
			queries.RuntimeEventEvmParsedFieldsUpdate,
			p.runtime,
			item.Event.Round,
			item.Event.TxIndex,
			item.Event.EventBody,
			eventName,
			eventArgs,
			eventSig,
		)
	}

	return nil
}

func (p *processor) QueueLength(ctx context.Context) (int, error) {
	var evQueueLength int
	// We limit the event count for performance reasons since the query requires a join of the transactions and events tables.
	if err := p.target.QueryRow(ctx, fmt.Sprintf("SELECT COUNT(*) FROM (%s) subquery", queries.RuntimeEvmEvents), p.runtime, p.currRound, 1000).Scan(&evQueueLength); err != nil {
		return 0, fmt.Errorf("querying number of verified abi events: %w", err)
	}
	return evQueueLength, nil
}
