package evm

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
	runtimeClient "github.com/oasisprotocol/oasis-core/go/runtime/client/api"
	sdkConfig "github.com/oasisprotocol/oasis-sdk/client-sdk/go/config"
	"github.com/stretchr/testify/require"

	"github.com/oasisprotocol/nexus/analyzer/evmabi"
	cmdCommon "github.com/oasisprotocol/nexus/cmd/common"
	"github.com/oasisprotocol/nexus/common"
	"github.com/oasisprotocol/nexus/config"
	"github.com/oasisprotocol/nexus/storage/oasis"
)

var (
	ChainName          = common.ChainNameMainnet
	CurrentArchiveName = config.DefaultChains[ChainName].CurrentRecord().ArchiveName
	// TODO: Would be nice to have an offline test.
	PublicSourceConfig = &config.SourceConfig{
		ChainName: ChainName,
		Nodes: map[string]*config.ArchiveConfig{
			CurrentArchiveName: {
				DefaultNode: &config.NodeConfig{
					RPC: sdkConfig.DefaultNetworks.All[string(ChainName)].RPC,
				},
			},
		},
		FastStartup: false,
	}
)

func TestERC165(t *testing.T) {
	ctx := context.Background()
	source, err := oasis.NewRuntimeClient(ctx, PublicSourceConfig, common.RuntimeEmerald)
	require.NoError(t, err)
	// AI ROSE on Emerald mainnet.
	tokenEthAddr, err := hex.DecodeString("0f4c5A429166608f9225E094F7E66B0bF68a53B9")
	require.NoError(t, err)
	supportsERC165, err := detectERC165(ctx, cmdCommon.Logger(), source, runtimeClient.RoundLatest, tokenEthAddr)
	require.NoError(t, err)
	require.True(t, supportsERC165)
	supportsERC721, err := detectInterface(ctx, cmdCommon.Logger(), source, runtimeClient.RoundLatest, tokenEthAddr, ERC165InterfaceID)
	require.NoError(t, err)
	require.True(t, supportsERC721)
}

func TestEVMDownloadTokenERC20(t *testing.T) {
	ctx := context.Background()
	source, err := oasis.NewRuntimeClient(ctx, PublicSourceConfig, common.RuntimeEmerald)
	require.NoError(t, err)
	// Wormhole bridged USDT on Emerald mainnet.
	tokenEthAddr, err := hex.DecodeString("dC19A122e268128B5eE20366299fc7b5b199C8e3")
	require.NoError(t, err)
	data, err := evmDownloadTokenERC20(ctx, cmdCommon.Logger(), source, runtimeClient.RoundLatest, tokenEthAddr)
	require.NoError(t, err)
	t.Logf("data %#v", data)
}

func TestEVMDownloadTokenBalanceERC20(t *testing.T) {
	ctx := context.Background()
	source, err := oasis.NewRuntimeClient(ctx, PublicSourceConfig, common.RuntimeEmerald)
	require.NoError(t, err)
	// Wormhole bridged USDT on Emerald mainnet.
	tokenEthAddr, err := hex.DecodeString("dC19A122e268128B5eE20366299fc7b5b199C8e3")
	require.NoError(t, err)
	// An address that possesses no USDT.
	accountEthAddr, err := hex.DecodeString("5555555555555555555555555555555555555555")
	require.NoError(t, err)
	balanceData, err := evmDownloadTokenBalanceERC20(ctx, cmdCommon.Logger(), source, runtimeClient.RoundLatest, tokenEthAddr, accountEthAddr)
	require.NoError(t, err)
	t.Logf("balance %#v", balanceData)
}

func TestEVMFailDeterministicUnoccupied(t *testing.T) {
	ctx := context.Background()
	source, err := oasis.NewRuntimeClient(ctx, PublicSourceConfig, common.RuntimeEmerald)
	require.NoError(t, err)
	// An address at which no smart contract exists.
	tokenEthAddr, err := hex.DecodeString("5555555555555555555555555555555555555555")
	require.NoError(t, err)
	var name string
	err = evmCallWithABI(ctx, source, runtimeClient.RoundLatest, tokenEthAddr, evmabi.ERC20, &name, "name")
	require.Error(t, err)
	fmt.Printf("getting ERC-20 name from unoccupied address should fail: %+v\n", err)
	require.True(t, errors.Is(err, EVMDeterministicError{}))
}

func TestEVMFailDeterministicOutOfGas(t *testing.T) {
	ctx := context.Background()
	source, err := oasis.NewRuntimeClient(ctx, PublicSourceConfig, common.RuntimeEmerald)
	require.NoError(t, err)
	// Wormhole bridged USDT on Emerald mainnet.
	tokenEthAddr, err := hex.DecodeString("dC19A122e268128B5eE20366299fc7b5b199C8e3")
	require.NoError(t, err)
	var name string
	// Use very low gas to cause out of gas condition.
	gasLimit := uint64(10)
	err = evmCallWithABICustom(ctx, source, runtimeClient.RoundLatest, DefaultGasPrice, gasLimit, DefaultCaller, tokenEthAddr, DefaultValue, evmabi.ERC20, &name, "name")
	require.Error(t, err)
	fmt.Printf("query that runs out of gas should fail: %+v\n", err)
	require.True(t, errors.Is(err, EVMDeterministicError{}))
}

func TestEVMFailDeterministicUnsupportedMethod(t *testing.T) {
	ctx := context.Background()
	source, err := oasis.NewRuntimeClient(ctx, PublicSourceConfig, common.RuntimeEmerald)
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
	err = evmCallWithABI(ctx, source, runtimeClient.RoundLatest, tokenEthAddr, &fakeABI, &name, "bike")
	require.Error(t, err)
	fmt.Printf("querying an unsupported method should fail: %+v\n", err)
	require.True(t, errors.Is(err, EVMDeterministicError{}))
}
