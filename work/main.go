package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"os"

	"github.com/jackc/pgx/v4"
	"github.com/oasisprotocol/oasis-core/go/common"
	"github.com/oasisprotocol/oasis-core/go/common/cbor"
	commonGrpc "github.com/oasisprotocol/oasis-core/go/common/grpc"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/client"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/crypto/signature"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/modules/core"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

func mainFallible(ctx context.Context) error {
	conn, err := commonGrpc.Dial("grpc.oasis.dev:443", grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{})))
	if err != nil {
		return err
	}
	var rtid common.Namespace
	chainAlias := "mainnet_emerald"
	if err = rtid.UnmarshalHex("000000000000000000000000000000000000000000000000e2eaa99fc008f87f"); err != nil {
		return err
	}
	mainnetEmeraldContext := signature.DeriveChainContext(rtid, "b11b369e0da5bb230b220127f5e7b242d385ef8c6f54906243f30af63c815535")
	rc := client.New(conn, rtid)
	round := uint64(2679238)
	var gasUsed int64
	var size int
	// Inaccurate: Ignore unparseable transactions.
	txrs, err := rc.GetTransactionsWithResults(ctx, round)
	if err != nil {
		return fmt.Errorf("get transactions with results round %d: %w", round, err)
	}
	for i, txr := range txrs {
		fmt.Printf("%#v\n", txr)
		var txGasUsed int64
		foundGasUsedEvent := false
		for j, event := range txr.Events {
			fmt.Printf("%#v\n", event)
			coreEvents, err1 := core.DecodeEvent(event)
			if err1 != nil {
				return fmt.Errorf("decode tx %d event %d: %w", i, j, err1)
			}
			for k, coreEvent := range coreEvents {
				coreEventCast, ok := coreEvent.(*core.Event)
				if !ok {
					return fmt.Errorf("could not cast tx %d event %d decoded event %d to core.Event", i, j, k)
				}
				if coreEventCast.GasUsed != nil {
					if foundGasUsedEvent {
						return fmt.Errorf("multiple gas used events in tx %d", i)
					}
					foundGasUsedEvent = true
					txGasUsed = int64(coreEventCast.GasUsed.Amount)
				}
			}
		}
		if !foundGasUsedEvent {
			if txr.Result.IsSuccess() || txr.Result.IsUnknown() {
				// Treat as if it used all the gas.
				tx, err1 := txr.Tx.Verify(mainnetEmeraldContext)
				if err1 != nil {
					// Should not be allowed to have a successful transaction without verification passing.
					return fmt.Errorf("verify tx %d: %w", i, err1)
				}
				txGasUsed = int64(tx.AuthInfo.Fee.Gas)
			} else {
				// Inaccurate: Treat as not using any gas.
			}
		}
		fmt.Printf("gas used: %d\n", txGasUsed)
		gasUsed += txGasUsed
		// Inaccurate: Re-serialize signed tx to estimate original size.
		txSize := len(cbor.Marshal(txr.Tx))
		fmt.Printf("tx size: %d\n", txSize)
		size += txSize
	}
	dbConn, err := pgx.Connect(ctx, "postgres://postgres:a@172.17.0.2/explorer")
	if err != nil {
		return err
	}
	rows, err := dbConn.Query(ctx, "INSERT INTO block_extra (chain_alias, round, gas_used, size) VALUES ($1, $2, $3, $4)", chainAlias, round, gasUsed, size)
	if err != nil {
		return err
	}
	rows.Close()
	return nil
}

func main() {
	if err := mainFallible(context.Background()); err != nil {
		fmt.Printf("%v\n", err)
		os.Exit(1)
	}
}
