package main

import (
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
	indexerV1 "github.com/oasisprotocol/oasis-indexer/api/v1"
	sdkClient "github.com/oasisprotocol/oasis-sdk/client-sdk/go/client"

	"oasis-explorer-backend/common"
)

func makeEmeraldRouter(dbPool *pgxpool.Pool, rtClient sdkClient.RuntimeClient) *chi.Mux {
	r := chi.NewRouter()
	r.Get("/home/latest_blocks", fallible(func(w http.ResponseWriter, r *http.Request) error {
		blockRows := make([]*BlockRow, 0, 10)
		if err := dbPool.BeginFunc(r.Context(), func(dbTx pgx.Tx) error {
			var batch pgx.Batch
			batch.Queue("SELECT height, b_hash, gas_used, size FROM block_extra WHERE chain_alias='mainnet_emerald' ORDER BY height DESC LIMIT 10")
			batchResults := dbTx.SendBatch(r.Context(), &batch)
			defer common.CloseOrLog(batchResults)
			blockExtraRows, err := batchResults.Query()
			if err != nil {
				return err
			}
			defer blockExtraRows.Close()
			for blockExtraRows.Next() {
				var blockRow BlockRow
				if err = blockExtraRows.Scan(&blockRow.Height, &blockRow.Hash, &blockRow.GasUsed, &blockRow.SizeBytes); err != nil {
					return err
				}
				blockRows = append(blockRows, &blockRow)
			}
			blockExtraRows.Close()
			return nil
		}); err != nil {
			return err
		}
		for _, br := range blockRows {
			b, err := rtClient.GetBlock(r.Context(), uint64(br.Height))
			if err != nil {
				return fmt.Errorf("rtClient.GetBlock height %d: %w", br.Height, err)
			}
			txrs, err := rtClient.GetTransactionsWithResults(r.Context(), uint64(br.Height))
			if err != nil {
				return fmt.Errorf("rtClient.GetTransactionsWithResults heighg %d: %w", br.Height, err)
			}
			br.Timestamp = int64(b.Header.Timestamp)
			br.NumTransactions = len(txrs)
		}
		var blockList indexerV1.BlockList
		if err := getOkReadJson(INDEXER_ENDPOINT+"/emerald/blocks?limit=10", &blockList); err != nil {
			return fmt.Errorf("indexer emerald blocks: %w", err)
		}
		blockRows := make([]BlockRow, len(blockList.Blocks))
		for i := range blockList.Blocks {
			blockRows[i].Height = blockList.Blocks[i].Height
			blockRows[i].Hash = blockList.Blocks[i].Hash
			blockRows[i].Timestamp = blockList.Blocks[i].Timestamp.Unix()
		}
		for _ = range blockRows {
			// TODO: other fields
		}
		if err := respondCacheableJson(w, blockRows, 6); err != nil {
			return fmt.Errorf("writing response: %w", err)
		}
		return nil
	}))
	r.Get("/home/latest_transactions", fallible(func(w http.ResponseWriter, r *http.Request) error {
		var transactionList indexerV1.TransactionList
		if err := getOkReadJson(INDEXER_ENDPOINT+"/emerald/transactions?limit=10", &transactionList); err != nil {
			return fmt.Errorf("indexer emerald transactions: %w", err)
		}
		transactionRows := make([]TransactionRow, len(transactionList.Transactions))
		for i := range transactionList.Transactions {
			transactionRows[i].Height = transactionList.Transactions[i].Height
			transactionRows[i].Hash = transactionList.Transactions[i].Hash
			transactionRows[i].From = transactionList.Transactions[i].Sender
			transactionRows[i].FeeAmount = int64(transactionList.Transactions[i].Fee)
			transactionRows[i].Method = transactionList.Transactions[i].Method
		}
		for _ = range transactionRows {
			// TODO: other fields
		}
		if err := respondCacheableJson(w, transactionRows, 6); err != nil {
			return fmt.Errorf("writing response: %w", err)
		}
		return nil
	}))
	return r
}
