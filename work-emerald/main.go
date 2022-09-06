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

func scanRound(ctx context.Context, dbConn *pgx.Conn, chainAlias string, rtClient client.RuntimeClient, sigContext signature.Context, round int64) error {
	fmt.Printf("scanning round %d\n", round)
	var gasUsed int64
	var size int
	// Inaccurate: Ignore unparseable transactions.
	txrs, err := rtClient.GetTransactionsWithResults(ctx, uint64(round))
	if err != nil {
		return fmt.Errorf("get transactions with results: %w", err)
	}
	for i, txr := range txrs {
		// fmt.Printf("%#v\n", txr)
		var txGasUsed int64
		foundGasUsedEvent := false
		for j, event := range txr.Events {
			// fmt.Printf("%#v\n", event)
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
				tx, err1 := txr.Tx.Verify(sigContext)
				if err1 != nil {
					// Should not be allowed to have a successful transaction without verification passing.
					return fmt.Errorf("verify tx %d: %w", i, err1)
				}
				txGasUsed = int64(tx.AuthInfo.Fee.Gas)
			} else {
				// Inaccurate: Treat as not using any gas.
			}
		}
		// fmt.Printf("gas used: %d\n", txGasUsed)
		gasUsed += txGasUsed
		// Inaccurate: Re-serialize signed tx to estimate original size.
		txSize := len(cbor.Marshal(txr.Tx))
		// fmt.Printf("tx size: %d\n", txSize)
		size += txSize
	}
	if err = dbConn.BeginTxFunc(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead}, func(tx pgx.Tx) error {
		if _, err1 := dbConn.Exec(ctx, "INSERT INTO block_extra (chain_alias, height, gas_used, size) VALUES ($1, $2, $3, $4)", chainAlias, round, gasUsed, size); err1 != nil {
			return err1
		}
		if _, err1 := dbConn.Exec(ctx, "UPDATE progress SET first_unscanned_height = $1 WHERE chain_alias = $2", round+1, chainAlias); err1 != nil {
			return err1
		}
		return nil
	}); err != nil {
		return err
	}
	return nil
}

func scanLoop(ctx context.Context, dbConn *pgx.Conn, chainAlias string, rc client.RuntimeClient, sigContext signature.Context) error {
	var firstUnscannedHeight int64
	if err := dbConn.QueryRow(ctx, "SELECT first_unscanned_height FROM progress WHERE chain_alias = $1", chainAlias).Scan(&firstUnscannedHeight); err != nil {
		return err
	}
	for round := firstUnscannedHeight; ; round++ {
		if err := scanRound(ctx, dbConn, chainAlias, rc, sigContext, round); err != nil {
			return fmt.Errorf("round %d: %w", round, err)
		}
	}
}

func mainFallible(ctx context.Context) error {
	dbConn, err := pgx.Connect(ctx, "postgres://postgres:a@172.17.0.2/explorer")
	if err != nil {
		return err
	}
	conn, err := commonGrpc.Dial("grpc.oasis.dev:443", grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{})))
	if err != nil {
		return err
	}
	chainAlias := "mainnet_emerald"
	var rtid common.Namespace
	if err = rtid.UnmarshalHex("000000000000000000000000000000000000000000000000e2eaa99fc008f87f"); err != nil {
		return err
	}
	rc := client.New(conn, rtid)
	sigContext := signature.DeriveChainContext(rtid, "b11b369e0da5bb230b220127f5e7b242d385ef8c6f54906243f30af63c815535")
	if err = scanLoop(ctx, dbConn, chainAlias, rc, sigContext); err != nil {
		return err
	}
	return nil
}

func main() {
	if err := mainFallible(context.Background()); err != nil {
		fmt.Printf("%v\n", err)
		os.Exit(1)
	}
}
