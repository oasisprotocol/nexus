package main

import (
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	indexerV1 "github.com/oasisprotocol/oasis-indexer/api/v1"
)

func makeEmeraldRouter() *chi.Mux {
	r := chi.NewRouter()
	r.Get("/home/latest_blocks", fallible(func(w http.ResponseWriter, r *http.Request) error {
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
