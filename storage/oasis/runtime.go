package oasis

import (
	"context"
	"fmt"

	config "github.com/oasisprotocol/oasis-sdk/client-sdk/go/config"
	connection "github.com/oasisprotocol/oasis-sdk/client-sdk/go/connection"
	runtimeSignature "github.com/oasisprotocol/oasis-sdk/client-sdk/go/crypto/signature"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/modules/evm"
	sdkTypes "github.com/oasisprotocol/oasis-sdk/client-sdk/go/types"

	"github.com/oasisprotocol/oasis-indexer/storage"
)

// RuntimeClient is a client to a ParaTime.
type RuntimeClient struct {
	client  connection.RuntimeClient
	network *config.Network

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
	block, err := rc.client.GetBlock(ctx, round)
	if err != nil {
		return nil, err
	}

	transactionsWithResults, err := rc.client.GetTransactionsWithResults(ctx, round)
	if err != nil {
		return nil, err
	}

	return &storage.RuntimeBlockData{
		Round:                   round,
		BlockHeader:             block,
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
		if network.ChainContext != rc.network.ChainContext {
			continue
		}
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
