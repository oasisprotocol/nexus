package nodeapi

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc"

	coreCommon "github.com/oasisprotocol/oasis-core/go/common"
	"github.com/oasisprotocol/oasis-core/go/common/cbor"
	"github.com/oasisprotocol/oasis-core/go/common/crypto/hash"
	roothash "github.com/oasisprotocol/oasis-core/go/roothash/api/block"
	coreRuntimeClient "github.com/oasisprotocol/oasis-core/go/runtime/client/api"
	cobaltRoothash "github.com/oasisprotocol/oasis-indexer/coreapi/v21.1.1/roothash/api/block"
	connection "github.com/oasisprotocol/oasis-sdk/client-sdk/go/connection"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/modules/evm"
)

// Implementation of `RuntimeApiLite` that supports all versions of the node ABI
// and all versions of the oasis-sdk ABI. (SDK events are CBOR-encoded according
// to the SDK ABI, then embedded into the node's `Event` type, which is fetched
// using the node ABI.)
//
// There are very few differences in the ABIs that RuntimeApiLite cares about,
// so we implement support for all (= both) versions here, using trial-and-error
// decoding where needed, rather than creating a new RuntimeApiLite
// implementation for every ABI version combination.
type UniversalRuntimeApiLite struct {
	runtimeID coreCommon.Namespace

	// A raw gRPC connection to the node. Used for fetching raw CBOR-encoded
	// responses for RPCs whose encodings changed over time, and this class
	// needs to handle the various formats/types.
	grpcConn *grpc.ClientConn

	// An oasis-sdk managed connection to the node. Used for RPCs that have
	// had a stable ABI over time. That is the majority of them, and oasis-sdk
	// provides nontrivial wrappers/parsing around raw RPC responses, making
	// this preferable to raw gRPC.
	sdkClient *connection.RuntimeClient
}

var _ RuntimeApiLite = (*UniversalRuntimeApiLite)(nil)

func NewUniversalRuntimeApiLite(runtimeID coreCommon.Namespace, grpcConn *grpc.ClientConn, sdkClient *connection.RuntimeClient) *UniversalRuntimeApiLite {
	return &UniversalRuntimeApiLite{
		runtimeID: runtimeID,
		grpcConn:  grpcConn,
		sdkClient: sdkClient,
	}
}

func (rc *UniversalRuntimeApiLite) GetBlockHeader(ctx context.Context, round uint64) (*RuntimeBlockHeader, error) {
	// Fetch the raw CBOR first, decode later.
	var rsp cbor.RawMessage
	if err := rc.grpcConn.Invoke(ctx, "/oasis-core.RuntimeClient/GetBlock", &coreRuntimeClient.GetBlockRequest{
		RuntimeID: rc.runtimeID,
		Round:     round,
	}, &rsp); err != nil {
		return nil, err
	}

	// Try parsing the GetBlock response against two ABIs: Cobalt, and post-Cobalt.
	var header RuntimeBlockHeader
	var block roothash.Block
	var cobaltBlock cobaltRoothash.Block
	if err := cbor.Unmarshal(rsp, &block); err == nil {
		// This is a post-Cobalt block.
		header = RuntimeBlockHeader{
			Version:        block.Header.Version,
			Namespace:      block.Header.Namespace,
			Round:          block.Header.Round,
			Timestamp:      time.Unix(int64(block.Header.Timestamp), 0 /* nanos */),
			Hash:           block.Header.EncodedHash(),
			PreviousHash:   block.Header.PreviousHash,
			IORoot:         block.Header.IORoot,
			StateRoot:      block.Header.StateRoot,
			MessagesHash:   block.Header.MessagesHash,
			InMessagesHash: block.Header.InMessagesHash,
		}
	} else if err := cbor.Unmarshal(rsp, &cobaltBlock); err == nil {
		// This is a Cobalt block.
		header = RuntimeBlockHeader{
			Version:        cobaltBlock.Header.Version,
			Namespace:      cobaltBlock.Header.Namespace,
			Round:          cobaltBlock.Header.Round,
			Timestamp:      time.Unix(int64(cobaltBlock.Header.Timestamp), 0 /* nanos */),
			Hash:           cobaltBlock.Header.EncodedHash(),
			PreviousHash:   cobaltBlock.Header.PreviousHash,
			IORoot:         cobaltBlock.Header.IORoot,
			StateRoot:      cobaltBlock.Header.StateRoot,
			MessagesHash:   cobaltBlock.Header.MessagesHash,
			InMessagesHash: hash.Hash{}, // Absent in Cobalt.
		}
	} else {
		return nil, fmt.Errorf("unsupported runtime block structure: %w %x", err, rsp)
	}
	return &header, nil
}

func (rc *UniversalRuntimeApiLite) GetTransactionsWithResults(ctx context.Context, round uint64) ([]RuntimeTransactionWithResults, error) {
	rsp, err := rc.sdkClient.GetTransactionsWithResults(ctx, round)
	if err != nil {
		return nil, err
	}

	// Convert to indexer-internal type.
	txrs := make([]RuntimeTransactionWithResults, len(rsp))
	for i, txr := range rsp {
		txrs[i] = (RuntimeTransactionWithResults)(*txr)
	}

	return txrs, nil
}

func (rc *UniversalRuntimeApiLite) GetEventsRaw(ctx context.Context, round uint64) ([]RuntimeEvent, error) {
	rsp, err := rc.sdkClient.GetEventsRaw(ctx, round)
	if err != nil {
		return nil, err
	}

	// Convert to indexer-internal type.
	evs := make([]RuntimeEvent, len(rsp))
	for i, ev := range rsp {
		evs[i] = (RuntimeEvent)(*ev)
	}

	return evs, nil
}

func (rc *UniversalRuntimeApiLite) EVMSimulateCall(ctx context.Context, round uint64, gasPrice []byte, gasLimit uint64, caller []byte, address []byte, value []byte, data []byte) ([]byte, error) {
	return evm.NewV1(rc.sdkClient).SimulateCall(ctx, round, gasPrice, gasLimit, caller, address, value, data)
}
