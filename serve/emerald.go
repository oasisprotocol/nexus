package main

import (
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
	ocCommon "github.com/oasisprotocol/oasis-core/go/common"
	sdkClient "github.com/oasisprotocol/oasis-sdk/client-sdk/go/client"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/crypto/signature"
	"google.golang.org/grpc"

	"oasis-explorer-backend/common"
)

func makeEmeraldRouter(dbPool *pgxpool.Pool, conn *grpc.ClientConn) *chi.Mux {
	chainAlias := "mainnet_emerald"
	var rtid ocCommon.Namespace
	if err := rtid.UnmarshalHex("000000000000000000000000000000000000000000000000e2eaa99fc008f87f"); err != nil {
		panic(err)
	}
	rtClient := sdkClient.New(conn, rtid)
	sigContext := signature.DeriveChainContext(rtid, "b11b369e0da5bb230b220127f5e7b242d385ef8c6f54906243f30af63c815535")

	r := chi.NewRouter()
	r.Get("/home/latest_blocks", fallible(func(w http.ResponseWriter, r *http.Request) error {
		blockRows := make([]*BlockRow, 0, latestBlocksCount)
		if err := dbPool.BeginFunc(r.Context(), func(dbTx pgx.Tx) error {
			var batch pgx.Batch
			batch.Queue("SELECT height, b_hash, gas_used, size FROM block_extra WHERE chain_alias = $1 ORDER BY height DESC LIMIT $2", chainAlias, latestBlocksCount)
			batchResults := dbTx.SendBatch(r.Context(), &batch)
			defer common.CloseOrLog(batchResults)
			blockExtraRows, err := batchResults.Query()
			if err != nil {
				return err
			}
			defer blockExtraRows.Close()
			for blockExtraRows.Next() {
				var br BlockRow
				if err = blockExtraRows.Scan(&br.Height, &br.Hash, &br.GasUsed, &br.SizeBytes); err != nil {
					return err
				}
				blockRows = append(blockRows, &br)
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
				return fmt.Errorf("rtClient.GetTransactionsWithResults height %d: %w", br.Height, err)
			}
			br.Timestamp = int64(b.Header.Timestamp)
			br.NumTransactions = len(txrs)
		}
		if err := respondCacheableJson(w, blockRows, 6); err != nil {
			return fmt.Errorf("writing response: %w", err)
		}
		return nil
	}))
	r.Get("/home/latest_transactions", fallible(func(w http.ResponseWriter, r *http.Request) error {
		transactionRows := make([]*TransactionRow, 0, latestTransactionsCount)
		if err := dbPool.BeginFunc(r.Context(), func(dbTx pgx.Tx) error {
			var batch pgx.Batch
			batch.Queue("SELECT te.height, te.tx_index, te.tx_hash, te.eth_hash, ts.addr FROM transaction_extra te LEFT JOIN transaction_signer ts ON ts.chain_alias = te.chain_alias AND ts.height = te.height AND ts.tx_index = te.tx_index WHERE te.chain_alias=$1 AND ts.signer_index = 0 ORDER BY te.height DESC, te.tx_index DESC LIMIT $2", chainAlias, latestTransactionsCount)
			batchResults := dbTx.SendBatch(r.Context(), &batch)
			defer common.CloseOrLog(batchResults)
			transactionExtraRows, err := batchResults.Query()
			if err != nil {
				return err
			}
			defer transactionExtraRows.Close()
			for transactionExtraRows.Next() {
				var tr TransactionRow
				if err = transactionExtraRows.Scan(&tr.Height, &tr.Index, &tr.Hash, &tr.EthHash, &tr.From); err != nil {
					return err
				}
				transactionRows = append(transactionRows, &tr)
			}
			transactionExtraRows.Close()
			return nil
		}); err != nil {
			return err
		}
		txsByBlock := map[int64][]*TransactionRow{}
		for _, tr := range transactionRows {
			txsByBlock[tr.Height] = append(txsByBlock[tr.Height], tr)
		}
		for height, trs := range txsByBlock {
			b, err := rtClient.GetBlock(r.Context(), uint64(height))
			if err != nil {
				return fmt.Errorf("rtClient.GetBlock height %d: %w", height, err)
			}
			txrs, err := rtClient.GetTransactionsWithResults(r.Context(), uint64(height))
			if err != nil {
				return fmt.Errorf("rtClient.GetTransactionsWithResults height %d: %w", height, err)
			}
			for _, tr := range trs {
				tr.Timestamp = int64(b.Header.Timestamp)
				txr := txrs[tr.Index]
				tx, err1 := common.VerifyUtx(sigContext, &txr.Tx)
				if err1 != nil {
					err1 = fmt.Errorf("height %d tx %d verify: %w", tr.Height, tr.Index, err1)
					fmt.Println(err1)
					tx = nil
				}
				if tx != nil {
					feeAmountBI := tx.AuthInfo.Fee.Amount.Amount.ToBigInt()
					if feeAmountBI.IsInt64() {
						tr.FeeAmount = feeAmountBI.Int64()
					} else {
						err1 = fmt.Errorf("height %d tx %d fee amount %v exceeds int64", tr.Height, tr.Index, tx.AuthInfo.Fee.Amount)
						fmt.Println(err1)
					}
					// todo: is gas used desired?
					tr.FeeGas = int64(tx.AuthInfo.Fee.Gas)
					tr.Method = tx.Call.Method
					// todo: To, Amount
				}
			}
		}
		if err := respondCacheableJson(w, transactionRows, 6); err != nil {
			return fmt.Errorf("writing response: %w", err)
		}
		return nil
	}))
	return r
}
