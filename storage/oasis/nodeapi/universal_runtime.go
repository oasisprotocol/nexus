package nodeapi

import (
	"context"
	"fmt"
	"time"

	coreCommon "github.com/oasisprotocol/oasis-core/go/common"
	"github.com/oasisprotocol/oasis-core/go/common/cbor"
	"github.com/oasisprotocol/oasis-core/go/common/crypto/hash"
	"github.com/oasisprotocol/oasis-core/go/common/errors"
	connection "github.com/oasisprotocol/oasis-sdk/client-sdk/go/connection"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/modules/rofl"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/modules/roflmarket"
	sdkTypes "github.com/oasisprotocol/oasis-sdk/client-sdk/go/types"

	roothash "github.com/oasisprotocol/nexus/coreapi/v22.2.11/roothash/api/block"
	coreRuntimeClient "github.com/oasisprotocol/nexus/coreapi/v22.2.11/runtime/client/api"
	"github.com/oasisprotocol/nexus/storage/oasis/connections"

	common "github.com/oasisprotocol/nexus/common"
	cobaltRoothash "github.com/oasisprotocol/nexus/coreapi/v21.1.1/roothash/api/block"
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
	grpcConn connections.GrpcConn

	// An oasis-sdk managed connection to the node. Used for RPCs that have
	// had a stable ABI over time. That is the majority of them, and oasis-sdk
	// provides nontrivial wrappers/parsing around raw RPC responses, making
	// this preferable to raw gRPC.
	sdkClient *connection.RuntimeClient
}

var _ RuntimeApiLite = (*UniversalRuntimeApiLite)(nil)

func NewUniversalRuntimeApiLite(runtimeID coreCommon.Namespace, grpcConn connections.GrpcConn, sdkClient *connection.RuntimeClient) *UniversalRuntimeApiLite {
	return &UniversalRuntimeApiLite{
		runtimeID: runtimeID,
		grpcConn:  grpcConn,
		sdkClient: sdkClient,
	}
}

func (rc *UniversalRuntimeApiLite) Close() error {
	return rc.grpcConn.Close()
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

	// Convert to nexus-internal type.
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

	// Convert to nexus-internal type.
	evs := make([]RuntimeEvent, len(rsp))
	for i, ev := range rsp {
		evs[i] = (RuntimeEvent)(*ev)
	}

	return evs, nil
}

// EVMSimulateCall simulates an evm call at a given height. If the node returns a successful
// response, it is stored in `FallibleResponse.Ok`. If the node returns a deterministic
// error, eg call_reverted, the error is stored in `FallbileResponse.deterministicErr`.
// If the call fails due to a nondeterministic error, the error is returned.
// Note: FallibleResponse should _not_ store transient errors or any error that is not a
// valid node response.
func (rc *UniversalRuntimeApiLite) EVMSimulateCall(ctx context.Context, round uint64, gasPrice []byte, gasLimit uint64, caller []byte, address []byte, value []byte, data []byte) (*FallibleResponse, error) {
	res, err := rc.sdkClient.Evm.SimulateCall(ctx, round, gasPrice, gasLimit, caller, address, value, data)
	if errors.Is(err, ErrSdkEVMExecutionFailed) || errors.Is(err, ErrSdkEVMReverted) {
		return &FallibleResponse{
			DeterministicErr: &DeterministicError{
				msg: err.Error(),
			},
		}, nil
	}
	if err != nil {
		return nil, err
	}
	return &FallibleResponse{
		Ok: res,
	}, nil
}

func (rc *UniversalRuntimeApiLite) EVMGetCode(ctx context.Context, round uint64, address []byte) ([]byte, error) {
	return rc.sdkClient.Evm.Code(ctx, round, address)
}

func (rc *UniversalRuntimeApiLite) GetBalances(ctx context.Context, round uint64, addr Address) (map[sdkTypes.Denomination]common.BigInt, error) {
	nodeBalances, err := rc.sdkClient.Accounts.Balances(ctx, round, sdkTypes.Address(addr))
	if err != nil {
		return nil, err
	}
	balances := make(map[sdkTypes.Denomination]common.BigInt)
	for denom, amount := range nodeBalances.Balances {
		balances[denom] = common.BigIntFromQuantity(amount)
	}

	return balances, nil
}

func (rc *UniversalRuntimeApiLite) GetMinGasPrice(ctx context.Context, round uint64) (map[sdkTypes.Denomination]common.BigInt, error) {
	minGasPrice, err := rc.sdkClient.Core.MinGasPrice(ctx, round)
	if err != nil {
		return nil, err
	}
	mgps := make(map[sdkTypes.Denomination]common.BigInt)
	for denom, amount := range minGasPrice {
		mgps[denom] = common.BigIntFromQuantity(amount)
	}

	return mgps, nil
}

func (rc *UniversalRuntimeApiLite) RoflApp(ctx context.Context, round uint64, id AppID) (*AppConfig, error) {
	app, err := rc.sdkClient.ROFL.App(ctx, round, id)
	if err != nil {
		module, code := errors.Code(err)
		if module == rofl.ModuleName && code == 2 { // "unknown application"
			// App doesn't exist, don't return an error so that the response is cached.
			return &AppConfig{}, nil
		}
		return nil, err
	}
	return app, nil
}

func (rc *UniversalRuntimeApiLite) RoflApps(ctx context.Context, round uint64) ([]*AppConfig, error) {
	return rc.sdkClient.ROFL.Apps(ctx, round)
}

func (rc *UniversalRuntimeApiLite) RoflAppInstance(ctx context.Context, round uint64, id AppID, rak sdkTypes.PublicKey) (*Registration, error) {
	return rc.sdkClient.ROFL.AppInstance(ctx, round, id, rak)
}

func (rc *UniversalRuntimeApiLite) RoflAppInstances(ctx context.Context, round uint64, id AppID) ([]*Registration, error) {
	return rc.sdkClient.ROFL.AppInstances(ctx, round, id)
}

func (rc *UniversalRuntimeApiLite) RoflMarketProvider(ctx context.Context, round uint64, providerAddress sdkTypes.Address) (*Provider, error) {
	provider, err := rc.sdkClient.ROFLMarket.Provider(ctx, round, providerAddress)
	if err != nil {
		module, code := errors.Code(err)
		if module == roflmarket.ModuleName && code == 3 { // "provider not found"
			// Provider doesn't exist, don't return an error so that the response is cached.
			return &Provider{}, nil
		}
		return nil, err
	}
	return provider, nil
}

func (rc *UniversalRuntimeApiLite) RoflMarketProviders(ctx context.Context, round uint64) ([]*Provider, error) {
	return rc.sdkClient.ROFLMarket.Providers(ctx, round)
}

func (rc *UniversalRuntimeApiLite) RoflMarketOffer(ctx context.Context, round uint64, providerAddress sdkTypes.Address, offerID OfferID) (*Offer, error) {
	offer, err := rc.sdkClient.ROFLMarket.Offer(ctx, round, providerAddress, offerID)
	if err != nil {
		module, code := errors.Code(err)
		if module == roflmarket.ModuleName && code == 7 { // "offer not found"
			// Offer doesn't exist, don't return an error so that the response is cached.
			return &Offer{}, nil
		}
	}
	return offer, nil
}

func (rc *UniversalRuntimeApiLite) RoflMarketOffers(ctx context.Context, round uint64, providerAddress sdkTypes.Address) ([]*Offer, error) {
	return rc.sdkClient.ROFLMarket.Offers(ctx, round, providerAddress)
}

func (rc *UniversalRuntimeApiLite) RoflMarketInstance(ctx context.Context, round uint64, providerAddress sdkTypes.Address, instanceID InstanceID) (*Instance, error) {
	instance, err := rc.sdkClient.ROFLMarket.Instance(ctx, round, providerAddress, instanceID)
	if err != nil {
		module, code := errors.Code(err)
		if module == roflmarket.ModuleName && code == 8 { // "instance not found"
			// Instance doesn't exist, don't return an error so that the response is cached.
			return &Instance{}, nil
		}
		return nil, err
	}
	return instance, nil
}

func (rc *UniversalRuntimeApiLite) RoflMarketInstances(ctx context.Context, round uint64, providerAddress sdkTypes.Address) ([]*Instance, error) {
	return rc.sdkClient.ROFLMarket.Instances(ctx, round, providerAddress)
}

func (rc *UniversalRuntimeApiLite) RoflMarketInstanceCommands(ctx context.Context, round uint64, providerAddress sdkTypes.Address, instanceID InstanceID) ([]*QueuedCommand, error) {
	return rc.sdkClient.ROFLMarket.InstanceCommands(ctx, round, providerAddress, instanceID)
}
