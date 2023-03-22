package oasis

import (
	"context"
	"fmt"

	"google.golang.org/grpc"

	"github.com/oasisprotocol/oasis-core/go/common/cbor"
	"github.com/oasisprotocol/oasis-core/go/common/crypto/hash"
	roothash "github.com/oasisprotocol/oasis-core/go/roothash/api/block"
	coreRuntimeClient "github.com/oasisprotocol/oasis-core/go/runtime/client/api"
	cobaltRoothash "github.com/oasisprotocol/oasis-indexer/coreapi/v21.1.1/roothash/api/block"
	config "github.com/oasisprotocol/oasis-sdk/client-sdk/go/config"
	connection "github.com/oasisprotocol/oasis-sdk/client-sdk/go/connection"
	runtimeSignature "github.com/oasisprotocol/oasis-sdk/client-sdk/go/crypto/signature"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/modules/evm"
	sdkTypes "github.com/oasisprotocol/oasis-sdk/client-sdk/go/types"

	"github.com/oasisprotocol/oasis-indexer/storage"
)

// RuntimeClient is a client to a ParaTime.
type RuntimeClient struct {
	client   connection.RuntimeClient
	grpcConn *grpc.ClientConn
	network  *config.Network

	info  *sdkTypes.RuntimeInfo
	rtCtx runtimeSignature.Context
}

// AllData returns all relevant data to the given round.
func (rc *RuntimeClient) AllData(ctx context.Context, round uint64) (*storage.RuntimeAllData, error) {
	blockData, err := rc.BlockData(ctx, round)
	if err != nil {
		return nil, err
	}
	rawEvents, err := rc.GetEventsRaw(ctx, round)
	if err != nil {
		return nil, err
	}

	data := storage.RuntimeAllData{
		Round:     round,
		BlockData: blockData,
		RawEvents: rawEvents,
	}
	return &data, nil
}

// BlockData gets block data in the specified round.
func (rc *RuntimeClient) BlockData(ctx context.Context, round uint64) (*storage.RuntimeBlockData, error) {
	// TODO: Extract this into a RuntimeApiLite interface.
	// There are very few differences in the ABIs that RuntimeApiLite cares about, so we implement
	// support for all (= both) versions here, in this trial-and-error fashion, rather than
	// creating a new RuntimeApiLite implementation for every node version.

	// Try parsing the GetBlock response against two ABIs: Cobalt, and post-Cobalt.
	var rsp cbor.RawMessage
	if err := rc.grpcConn.Invoke(ctx, "/oasis-core.RuntimeClient/GetBlock", &coreRuntimeClient.GetBlockRequest{
		RuntimeID: rc.info.ID,
		Round:     round,
	}, &rsp); err != nil {
		return nil, err
	}
	var block roothash.Block
	var cobaltBlock cobaltRoothash.Block
	if err := cbor.Unmarshal(rsp, &block); err != nil {
		// This is a post-Cobalt block.
		// We use the same type internally to represent blocks; no further action needed.
	} else if err := cbor.Unmarshal(rsp, &cobaltBlock); err != nil {
		// This is a Cobalt block. Convert it to our internal format (= post-Cobalt block).
		block = roothash.Block{
			Header: roothash.Header{
				Version:        cobaltBlock.Header.Version,
				Namespace:      cobaltBlock.Header.Namespace,
				Round:          cobaltBlock.Header.Round,
				Timestamp:      roothash.Timestamp(cobaltBlock.Header.Timestamp),
				HeaderType:     roothash.HeaderType(cobaltBlock.Header.HeaderType), // We assume a backwards-compatible enum.
				IORoot:         cobaltBlock.Header.IORoot,
				StateRoot:      cobaltBlock.Header.StateRoot,
				MessagesHash:   cobaltBlock.Header.MessagesHash,
				InMessagesHash: hash.Hash{}, // Absent in Cobalt.
			},
		}
	} else {
		return nil, fmt.Errorf("unsupported runtime block structure: %#v", rsp)
	}

	transactionsWithResults, err := rc.client.GetTransactionsWithResults(ctx, round)
	if err != nil {
		return nil, err
	}

	return &storage.RuntimeBlockData{
		BlockHeader:             &block.Header,
		TransactionsWithResults: transactionsWithResults,
	}, nil
}

func (rc *RuntimeClient) GetEventsRaw(ctx context.Context, round uint64) ([]*sdkTypes.Event, error) {
	return rc.client.GetEventsRaw(ctx, round)
}

func (rc *RuntimeClient) EVMSimulateCall(ctx context.Context, round uint64, gasPrice []byte, gasLimit uint64, caller []byte, address []byte, value []byte, data []byte) ([]byte, error) {
	return evm.NewV1(rc.client).SimulateCall(ctx, round, gasPrice, gasLimit, caller, address, value, data)
}

// Name returns the name of the client, for the RuntimeSourceStorage interface.
func (rc *RuntimeClient) Name() string {
	paratimeName := "unknown"
	for _, network := range config.DefaultNetworks.All {
		for pName := range network.ParaTimes.All {
			if pName == rc.info.ID.String() {
				paratimeName = pName
				break
			}
		}

		if paratimeName != "unknown" {
			break
		}
	}
	return fmt.Sprintf("%s_runtime", paratimeName)
}

func (rc *RuntimeClient) nativeTokenSymbol() string {
	for _, network := range config.DefaultNetworks.All {
		// Iterate over all networks and find the one that contains the runtime.
		// Any network will do; we assume that paratime IDs are unique across networks.
		for _, paratime := range network.ParaTimes.All {
			if paratime.ID == rc.info.ID.Hex() {
				return paratime.Denominations[config.NativeDenominationKey].Symbol
			}
		}
	}
	panic("Cannot find native token symbol for runtime")
}

func (rc *RuntimeClient) StringifyDenomination(d sdkTypes.Denomination) string {
	if d.IsNative() {
		return rc.nativeTokenSymbol()
	}

	return d.String()
}
