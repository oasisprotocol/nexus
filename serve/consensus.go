package main

import (
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	indexerV1 "github.com/oasisprotocol/oasis-indexer/api/v1"
)

func makeConsensusRouter() *chi.Mux {
	r := chi.NewRouter()
	r.Get("/home/latest_blocks", fallible(func(w http.ResponseWriter, r *http.Request) error {
		var status indexerV1.Status
		if err := getOkReadJson(INDEXER_ENDPOINT+"/", &status); err != nil {
			return err
		}
		var blockList indexerV1.BlockList
		if err := getOkReadJson(INDEXER_ENDPOINT+fmt.Sprintf("/consensus/blocks?from=%d&limit=10", status.LatestBlock-9), &blockList); err != nil {
			return err
		}
		blockRows := make([]BlockRow, len(blockList.Blocks))
		for i := range blockList.Blocks {
			blockRows[i].Height = blockList.Blocks[i].Height
			blockRows[i].Hash = blockList.Blocks[i].Hash
			blockRows[i].Timestamp = blockList.Blocks[i].Timestamp.Unix()
		}
		for i := range blockRows {
			var transactionList indexerV1.TransactionList
			if err := getOkReadJson(INDEXER_ENDPOINT+fmt.Sprintf("/consensus/transactions?block=%d", blockRows[i].Height), &transactionList); err != nil {
				return err
			}
			blockRows[i].NumTransactions = len(transactionList.Transactions)
			// TODO: block size
			// TODO: total gas
		}
		if err := respondCacheableJson(w, blockRows, 6); err != nil {
			return fmt.Errorf("writing response: %w", err)
		}
		return nil
	}))
	r.Get("/home/latest_transactions", fallible(func(w http.ResponseWriter, r *http.Request) error {
		var transactionList indexerV1.TransactionList
		if err := getOkReadJson(INDEXER_ENDPOINT+"/consensus/transactions?limit=10", &transactionList); err != nil {
			return err
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
