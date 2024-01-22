package evmabibackfill

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
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

// Transaction data is canonically represented as a byte array. However,
// the transaction body is stored as a JSONB column in postgres, which
// causes the tx body->>data to be returned as a base64-encoded string
// enclosed by escaped double quote characters.
func cleanTxData(raw string) ([]byte, error) {
	s := strings.TrimPrefix(strings.TrimSuffix(raw, "\""), "\"")
	return base64.StdEncoding.DecodeString(s)
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
		var rawTxData string
		var tx abiEncodedTx
		var item abiEncodedItem
		item.Tx = &tx
		if err = txRows.Scan(
			&item.ContractAddr,
			&item.Abi,
			&tx.TxHash,
			&rawTxData,
			&tx.TxRevertReason,
		); err != nil {
			return nil, fmt.Errorf("scanning verified contract tx: %w", err)
		}
		if tx.TxData, err = cleanTxData(rawTxData); err != nil {
			return nil, fmt.Errorf("error decoding tx data from db: %w", err)
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

func (p *processor) ProcessItem(ctx context.Context, batch *storage.QueryBatch, item *abiEncodedItem) error {
	// Unmarshal abi
	contractAbi, err := abi.JSON(bytes.NewReader(item.Abi))
	if err != nil {
		return fmt.Errorf("error unmarshalling abi: %w", err)
	}
	// Parse data
	if item.Event != nil { //nolint:nestif // has complex nested blocks (complexity: 12) (nestif)
		abiEvent, abiEventArgs, err := abiparse.ParseEvent(item.Event.EventBody.Topics, item.Event.EventBody.Data, &contractAbi)
		if err != nil {
			queueIncompatibleEventUpdate(batch, p.runtime, item.Event.Round, item.Event.TxIndex)
			p.logger.Warn("error processing event using abi", "contract address", item.ContractAddr, "err", err)
			return nil
		}
		eventArgs, err := marshalArgs(abiEvent.Inputs, abiEventArgs)
		if err != nil {
			queueIncompatibleEventUpdate(batch, p.runtime, item.Event.Round, item.Event.TxIndex)
			p.logger.Warn("error processing event args using abi", "contract address", item.ContractAddr, "err", err)
			return nil
		}

		batch.Queue(
			queries.RuntimeEventEvmParsedFieldsUpdate,
			p.runtime,
			item.Event.Round,
			item.Event.TxIndex,
			abiEvent.RawName,
			eventArgs,
			abiEvent.ID,
		)
	} else if item.Tx != nil {
		method, abiTxArgs, err := abiparse.ParseData(item.Tx.TxData, &contractAbi)
		if err != nil {
			queueIncompatibleTxUpdate(batch, p.runtime, item.Tx.TxHash)
			p.logger.Warn("error processing tx using abi", "contract address", item.ContractAddr, "err", err)
			return nil
		}
		txArgs, err := marshalArgs(method.Inputs, abiTxArgs)
		if err != nil {
			queueIncompatibleTxUpdate(batch, p.runtime, item.Tx.TxHash)
			p.logger.Warn("error processing tx args using abi", "contract address", item.ContractAddr, "err", err)
			return nil
		}
		var abiErrName string
		var abiErr *abi.Error
		var abiErrArgs []interface{}
		var errArgs []*abiEncodedArg
		if item.Tx.TxRevertReason != nil {
			txrr, err := cleanTxRevertReason(*item.Tx.TxRevertReason)
			if err != nil {
				// This is most likely an older tx with a plaintext revert reason, such
				// as "reverted: Ownable: caller is not the owner". In this case, we do
				// not parse the error with the abi, but we still update the tx table with
				// the method and args.
				batch.Queue(
					queries.RuntimeTransactionEvmParsedFieldsUpdate,
					p.runtime,
					item.Tx.TxHash,
					method.RawName,
					txArgs,
					nil, // error name
					nil, // error args
				)
				p.logger.Info("encountered likely old-style reverted transaction", "revert reason", item.Tx.TxRevertReason, "tx hash", item.Tx.TxHash, "contract address", item.ContractAddr, "err", err)
				return nil
			}
			abiErr, abiErrArgs, err = abiparse.ParseError(txrr, &contractAbi)
			if err != nil || abiErr == nil {
				queueIncompatibleTxUpdate(batch, p.runtime, item.Tx.TxHash)
				p.logger.Warn("error processing tx error using abi", "contract address", item.ContractAddr, "err", err)
				return nil
			}
			abiErrName = runtime.TxRevertErrPrefix + abiErr.Name + prettyPrintArgs(abiErrArgs)
			errArgs, err = marshalArgs(abiErr.Inputs, abiErrArgs)
			if err != nil {
				queueIncompatibleTxUpdate(batch, p.runtime, item.Tx.TxHash)
				p.logger.Warn("error processing tx error args", "contract address", item.ContractAddr, "err", err)
				return nil
			}
		}
		batch.Queue(
			queries.RuntimeTransactionEvmParsedFieldsUpdate,
			p.runtime,
			item.Tx.TxHash,
			method.RawName,
			txArgs,
			abiErrName,
			errArgs,
		)
	}

	return nil
}

// If abi processing of an event fails, it is likely due to incompatible
// data+abi, which is the event's fault. We thus mark the incompatible
// event as processed and continue.
//
// Note that if the abi is ever updated, we may need to revisit these events.
func queueIncompatibleEventUpdate(batch *storage.QueryBatch, runtime common.Runtime, round uint64, txIndex *int) {
	batch.Queue(
		queries.RuntimeEventEvmParsedFieldsUpdate,
		runtime,
		round,
		txIndex,
		nil, // event name
		nil, // event args
		nil, // event signature
	)
}

// If abi processing of an transaction fails, it is likely due to incompatible
// data+abi, which is the transaction's fault. We thus mark the incompatible
// transaction as processed and continue.
//
// Note that if the abi is ever updated, we may need to revisit these txs.
func queueIncompatibleTxUpdate(batch *storage.QueryBatch, runtime common.Runtime, txHash string) {
	batch.Queue(
		queries.RuntimeTransactionEvmParsedFieldsUpdate,
		runtime,
		txHash,
		nil, // method name
		nil, // method args
		nil, // error name
		nil, // error args
	)
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
