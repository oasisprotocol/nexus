package modules

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
	ethCommon "github.com/ethereum/go-ethereum/common"
	"github.com/oasisprotocol/oasis-core/go/runtime/client/api"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/config"
	"github.com/stretchr/testify/require"

	"github.com/oasisprotocol/oasis-indexer/analyzer/evmabi"
	"github.com/oasisprotocol/oasis-indexer/cmd/common"
	"github.com/oasisprotocol/oasis-indexer/storage/oasis"
)

func TestEVMDownloadTokenERC20(t *testing.T) {
	// TODO: Would be nice to have an offline test.
	ctx := context.Background()
	cf, err := oasis.NewClientFactory(ctx, config.DefaultNetworks.All["mainnet"], true)
	require.NoError(t, err)
	source, err := cf.Runtime(config.DefaultNetworks.All["mainnet"].ParaTimes.All["emerald"].ID)
	require.NoError(t, err)
	// Wormhole bridged USDT on Emerald mainnet.
	tokenEthAddr, err := hex.DecodeString("dC19A122e268128B5eE20366299fc7b5b199C8e3")
	require.NoError(t, err)
	data, err := evmDownloadTokenERC20(ctx, common.Logger(), source, api.RoundLatest, tokenEthAddr)
	require.NoError(t, err)
	t.Logf("data %#v", data)
}

func TestEVMFailDeterministicUnoccupied(t *testing.T) {
	// TODO: Would be nice to have an offline test.
	ctx := context.Background()
	cf, err := oasis.NewClientFactory(ctx, config.DefaultNetworks.All["mainnet"], true)
	require.NoError(t, err)
	source, err := cf.Runtime(config.DefaultNetworks.All["mainnet"].ParaTimes.All["emerald"].ID)
	require.NoError(t, err)
	// An address at which no smart contract exists.
	tokenEthAddr, err := hex.DecodeString("5555555555555555555555555555555555555555")
	require.NoError(t, err)
	var name string
	err = evmCallWithABI(ctx, source, api.RoundLatest, tokenEthAddr, evmabi.ERC20, &name, "name")
	require.Error(t, err)
	fmt.Printf("getting ERC-20 name from unoccupied address should fail: %+v\n", err)
	require.True(t, errors.Is(err, EVMDeterministicError{}))
}

func TestEVMFailDeterministicOutOfGas(t *testing.T) {
	// TODO: Would be nice to have an offline test.
	ctx := context.Background()
	cf, err := oasis.NewClientFactory(ctx, config.DefaultNetworks.All["mainnet"], true)
	require.NoError(t, err)
	source, err := cf.Runtime(config.DefaultNetworks.All["mainnet"].ParaTimes.All["emerald"].ID)
	require.NoError(t, err)
	// Wormhole bridged USDT on Emerald mainnet.
	tokenEthAddr, err := hex.DecodeString("dC19A122e268128B5eE20366299fc7b5b199C8e3")
	require.NoError(t, err)
	var name string
	gasPrice := []byte{1}
	// Use very low gas to cause out of gas condition.
	gasLimit := uint64(10)
	caller := ethCommon.Address{1}.Bytes()
	value := []byte{0}
	err = evmCallWithABICustom(ctx, source, api.RoundLatest, gasPrice, gasLimit, caller, tokenEthAddr, value, evmabi.ERC20, &name, "name")
	require.Error(t, err)
	fmt.Printf("query that runs out of gas should fail: %+v\n", err)
	require.True(t, errors.Is(err, EVMDeterministicError{}))
}

func TestEVMFailDeterministicUnsupportedMethod(t *testing.T) {
	// TODO: Would be nice to have an offline test.
	ctx := context.Background()
	cf, err := oasis.NewClientFactory(ctx, config.DefaultNetworks.All["mainnet"], true)
	require.NoError(t, err)
	source, err := cf.Runtime(config.DefaultNetworks.All["mainnet"].ParaTimes.All["emerald"].ID)
	require.NoError(t, err)
	// Wormhole bridged USDT on Emerald mainnet.
	tokenEthAddr, err := hex.DecodeString("dC19A122e268128B5eE20366299fc7b5b199C8e3")
	require.NoError(t, err)
	fakeABI, err := abi.JSON(strings.NewReader(`[{
	  "constant": true,
	  "inputs": [],
	  "name": "bike",
	  "outputs": [
	    {
	      "name": "",
	      "type": "string"
	    }
	  ],
	  "payable": false,
	  "stateMutability": "view",
	  "type": "function"
	}]`))
	require.NoError(t, err)
	var name string
	err = evmCallWithABI(ctx, source, api.RoundLatest, tokenEthAddr, &fakeABI, &name, "bike")
	require.Error(t, err)
	fmt.Printf("querying an unsupported method should fail: %+v\n", err)
	require.True(t, errors.Is(err, EVMDeterministicError{}))
}
