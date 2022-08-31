package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"os"

	"github.com/oasisprotocol/oasis-core/go/common"
	commonGrpc "github.com/oasisprotocol/oasis-core/go/common/grpc"
	runtimeClient "github.com/oasisprotocol/oasis-core/go/runtime/client/api"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

func mainFallible(ctx context.Context) error {
	conn, err := commonGrpc.Dial("grpc.oasis.dev:443", grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{})))
	if err != nil {
		return err
	}
	rt := runtimeClient.NewRuntimeClient(conn)
	var rtid common.Namespace
	if err = rtid.UnmarshalHex("000000000000000000000000000000000000000000000000e2eaa99fc008f87f"); err != nil {
		return err
	}
	txrs, err := rt.GetTransactionsWithResults(ctx, &runtimeClient.GetTransactionsRequest{
		RuntimeID: rtid,
		Round:     2679238,
	})
	if err != nil {
		return err
	}
	for _, txr := range txrs {
		fmt.Printf("%#v\n", txr)
		for _, event := range txr.Events {
			fmt.Printf("%#v\n", event)
		}
	}
	return nil
}

func main() {
	if err := mainFallible(context.Background()); err != nil {
		fmt.Printf("%v\n", err)
		os.Exit(1)
	}
}
