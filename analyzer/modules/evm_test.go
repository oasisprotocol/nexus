package modules

import (
	"context"
	"encoding/hex"
	"testing"

	"github.com/oasisprotocol/oasis-core/go/runtime/client/api"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/config"
	"github.com/stretchr/testify/require"

	"github.com/oasisprotocol/oasis-indexer/cmd/common"
	"github.com/oasisprotocol/oasis-indexer/storage/oasis"
)

func TestEVMTokenDataERC20(t *testing.T) {
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
