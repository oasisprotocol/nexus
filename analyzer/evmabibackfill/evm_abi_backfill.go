package evmabibackfill

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	ethCommon "github.com/ethereum/go-ethereum/common"
	sdkEVM "github.com/oasisprotocol/oasis-sdk/client-sdk/go/modules/evm"

	"github.com/oasisprotocol/nexus/analyzer"
	"github.com/oasisprotocol/nexus/analyzer/item"
	"github.com/oasisprotocol/nexus/analyzer/queries"
	"github.com/oasisprotocol/nexus/analyzer/runtime"
	"github.com/oasisprotocol/nexus/analyzer/runtime/abiparse"
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
	txRows, err := p.target.Query(ctx, queries.RuntimeEvmVerifiedContractTxs, p.runtime, limit)
	if err != nil {
		return nil, fmt.Errorf("querying verified contract txs: %w", err)
	}
	defer txRows.Close()
	for txRows.Next() {
		var tx abiEncodedTx
		var item abiEncodedItem
		item.Tx = &tx
		if err = txRows.Scan(
			&item.ContractAddr,
			&item.Abi,
			&tx.TxHash,
			&tx.TxData,
			&tx.TxRevertReason,
		); err != nil {
			return nil, fmt.Errorf("scanning verified contract tx: %w", err)
		}
		items = append(items, &item)
	}
	// Short circuit.
	if len(items) == int(limit) {
		return items, nil
	}
	eventRows, err := p.target.Query(ctx, queries.RuntimeEvmVerifiedContractEvents, p.runtime, int(limit)-len(items))
	if err != nil {
		return nil, fmt.Errorf("querying verified contract evs: %w", err)
	}
	defer eventRows.Close()
	for eventRows.Next() {
		var ev abiEncodedEvent
		var item abiEncodedItem
		item.Event = &ev
		if err = eventRows.Scan(
			&item.ContractAddr,
			&item.Abi,
			&ev.Round,
			&ev.TxIndex,
			&ev.EventBody,
		); err != nil {
			return nil, fmt.Errorf("scanning verified contract event: %w", err)
		}
		items = append(items, &item)
	}
	return items, nil
}

// Transaction revert reasons for failed evm transactions have been encoded
// differently over the course of Oasis history. Older transaction revert
// reasons were returned as one of
// - "reverted: Incorrect premium amount"
// - "reverted: base64(up to 1024 bytes of revert data)"
//
// Note that if the revert reason was longer than 1024 bytes it was truncated.
// Newer transaction revert reasons are returned as
// - "reverted: base64(revert data)"
//
// In all cases the revert reason has the "reverted: " prefix, which we first
// strip. We then attempt to base64-decode the remaining string to recover
// the error message. It should be noted that if the b64 decoding fails, it's
// likely an older error message.
//
// See the docstring of tryParseErrorMessage in analyzer/runtime/extract.go
// for more info.
func cleanTxRevertReason(raw string) ([]byte, error) {
	s := strings.TrimPrefix(raw, runtime.TxRevertErrPrefix)
	return base64.StdEncoding.DecodeString(s)
}

// Attempts to parse the raw event body into the event name, args, and signature
// as defined by the abi of the contract that emitted this event.
func (p *processor) parseEvent(ev *abiEncodedEvent, contractAbi abi.ABI) (*string, []*abiEncodedArg, *ethCommon.Hash, error) {
	abiEvent, abiEventArgs, err := abiparse.ParseEvent(ev.EventBody.Topics, ev.EventBody.Data, &contractAbi)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("error processing event using abi: %w", err)
	}
	eventArgs, err := marshalArgs(abiEvent.Inputs, abiEventArgs)
	if err != nil {
		p.logger.Warn("error processing event args using abi", "err", err)
		return nil, nil, nil, fmt.Errorf("error processing event args using abi: %w", err)
	}

	return &abiEvent.Name, eventArgs, &abiEvent.ID, nil
}

// Attempts to parse the raw evm.Call transaction data into the transaction
// method name and arguments as defined by the abi of the contract that was called.
func (p *processor) parseTxCall(tx *abiEncodedTx, contractAbi abi.ABI) (*string, []*abiEncodedArg, error) {
	method, abiTxArgs, err := abiparse.ParseData(tx.TxData, &contractAbi)
	if err != nil {
		return nil, nil, fmt.Errorf("error processing tx using abi: %w", err)
	}
	txArgs, err := marshalArgs(method.Inputs, abiTxArgs)
	if err != nil {
		return nil, nil, fmt.Errorf("error processing tx args using abi: %w", err)
	}

	return &method.RawName, txArgs, nil
}

// Attempts to parse the transaction revert reason into the error name and args
// as defined by the abi of the contract that was called.
func (p *processor) parseTxErr(tx *abiEncodedTx, contractAbi abi.ABI) (*string, []*abiEncodedArg) {
	var abiErrMsg string
	var abiErr *abi.Error
	var abiErrArgs []interface{}
	var errArgs []*abiEncodedArg
	if tx.TxRevertReason != nil {
		txrr, err := cleanTxRevertReason(*tx.TxRevertReason)
		if err != nil {
			// This is most likely an older tx with a plaintext revert reason, such
			// as "reverted: Ownable: caller is not the owner". In this case, we do
			// not parse the error with the abi.
			p.logger.Info("encountered likely old-style reverted transaction", "revert reason", tx.TxRevertReason, "tx hash", tx.TxHash, "err", err)
			return nil, nil
		}
		abiErr, abiErrArgs, err = abiparse.ParseError(txrr, &contractAbi)
		if err != nil || abiErr == nil {
			p.logger.Warn("error processing tx error using abi", "contract address", "err", err)
			return nil, nil
		}
		abiErrMsg = runtime.TxRevertErrPrefix + abiErr.Name + prettyPrintArgs(abiErrArgs)
		errArgs, err = marshalArgs(abiErr.Inputs, abiErrArgs)
		if err != nil {
			p.logger.Warn("error processing tx error args", "err", err)
			return nil, nil
		}
	}

	return &abiErrMsg, errArgs
}

func (p *processor) ProcessItem(ctx context.Context, batch *storage.QueryBatch, item *abiEncodedItem) error {
	// Unmarshal abi
	contractAbi, err := abi.JSON(bytes.NewReader(item.Abi))
	if err != nil {
		return fmt.Errorf("error unmarshalling abi: %w", err)
	}
	// Parse data
	p.logger.Debug("processing item using abi", "contract_address", item.ContractAddr)
	if item.Event != nil {
		eventName, eventArgs, eventSig, err := p.parseEvent(item.Event, contractAbi)
		if err != nil {
			p.logger.Warn("error parsing event with abi", "err", err, "contract_address", item.ContractAddr, "event_round", item.Event.Round, "event_tx_index", item.Event.TxIndex)
			// Write to the DB regardless of error so we don't keep retrying the same item.
		}
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
	} else if item.Tx != nil {
		methodName, methodArgs, err := p.parseTxCall(item.Tx, contractAbi)
		if err != nil {
			p.logger.Warn("error parsing tx with abi", "err", err, "contract_address", item.ContractAddr, "tx_hash", item.Tx.TxHash)
			// Write to the DB regardless of error so we don't keep retrying the same item.
		}
		errMsg, errArgs := p.parseTxErr(item.Tx, contractAbi)
		batch.Queue(
			queries.RuntimeTransactionEvmParsedFieldsUpdate,
			p.runtime,
			item.Tx.TxHash,
			methodName,
			methodArgs,
			errMsg,
			errArgs,
		)
	}

	return nil
}

func marshalArgs(abiArgs abi.Arguments, argVals []interface{}) ([]*abiEncodedArg, error) {
	if len(abiArgs) != len(argVals) {
		return nil, fmt.Errorf("number of args does not match abi specification")
	}
	args := []*abiEncodedArg{}
	for i, v := range argVals {
		args = append(args, &abiEncodedArg{
			Name:    abiArgs[i].Name,
			EvmType: abiArgs[i].Type.String(),
			Value:   v,
		})
	}

	return args, nil
}

func prettyPrintArgs(argVals []interface{}) string {
	var sb strings.Builder
	sb.WriteString("(")
	for i, v := range argVals {
		if i == len(argVals)-1 {
			sb.WriteString(fmt.Sprintf("%v", v))
		} else {
			sb.WriteString(fmt.Sprintf("%v,", v))
		}
	}
	sb.WriteString(")")
	return sb.String()
}

func (p *processor) QueueLength(ctx context.Context) (int, error) {
	var txQueueLength int
	if err := p.target.QueryRow(ctx, fmt.Sprintf("SELECT COUNT(*) FROM (%s) subquery", queries.RuntimeEvmVerifiedContractTxs), p.runtime, 1000).Scan(&txQueueLength); err != nil {
		return 0, fmt.Errorf("querying number of verified abi txs: %w", err)
	}
	var evQueueLength int
	// We limit the event count for performance reasons since the query requires a join of the transactions and events tables.
	if err := p.target.QueryRow(ctx, fmt.Sprintf("SELECT COUNT(*) FROM (%s) subquery", queries.RuntimeEvmVerifiedContractEvents), p.runtime, 1000).Scan(&evQueueLength); err != nil {
		return 0, fmt.Errorf("querying number of verified abi events: %w", err)
	}
	return txQueueLength + evQueueLength, nil
}
