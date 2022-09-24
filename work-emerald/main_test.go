package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"testing"

	ocCommon "github.com/oasisprotocol/oasis-core/go/common"
	ocGrpc "github.com/oasisprotocol/oasis-core/go/common/grpc"
	sdkClient "github.com/oasisprotocol/oasis-sdk/client-sdk/go/client"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/crypto/signature"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

func TestEvmEvent(t *testing.T) {
	ctx := context.Background()
	conn, err := ocGrpc.Dial("grpc.oasis.dev:443", grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{})))
	require.NoError(t, err)
	var rtid ocCommon.Namespace
	err = rtid.UnmarshalHex("000000000000000000000000000000000000000000000000e2eaa99fc008f87f")
	require.NoError(t, err)
	rtClient := sdkClient.New(conn, rtid)
	sigContext := signature.DeriveChainContext(rtid, "b11b369e0da5bb230b220127f5e7b242d385ef8c6f54906243f30af63c815535")
	// https://explorer.emerald.oasis.dev/tx/0x835c6db89994782b86a6b86aa6282adeac603907aa25fad5b7148ce321e295fd/token-transfers
	b, txrs, err := downloadRound(ctx, rtClient, 2934042)
	require.NoError(t, err)
	blockData, err := extractRound(sigContext, b, txrs)
	require.NoError(t, err)
	blockDataPretty, err := json.MarshalIndent(blockData, "", "  ")
	require.NoError(t, err)
	t.Log(string(blockDataPretty))
}

func TestEvmTransfer(t *testing.T) {
	ctx := context.Background()
	conn, err := ocGrpc.Dial("grpc.oasis.dev:443", grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{})))
	require.NoError(t, err)
	var rtid ocCommon.Namespace
	err = rtid.UnmarshalHex("000000000000000000000000000000000000000000000000e2eaa99fc008f87f")
	require.NoError(t, err)
	rtClient := sdkClient.New(conn, rtid)
	sigContext := signature.DeriveChainContext(rtid, "b11b369e0da5bb230b220127f5e7b242d385ef8c6f54906243f30af63c815535")
	// https://explorer.emerald.oasis.dev/tx/0x67fa1a2318a415d026508037dc0ab2eae75f6aab9cd2e6a14c0645868ac4e52c/internal-transactions
	b, txrs, err := downloadRound(ctx, rtClient, 2990282)
	require.NoError(t, err)
	blockData, err := extractRound(sigContext, b, txrs)
	require.NoError(t, err)
	blockDataPretty, err := json.MarshalIndent(blockData, "", "  ")
	require.NoError(t, err)
	t.Log(string(blockDataPretty))
}
