package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"os"

	"github.com/jackc/pgx/v4"
	"github.com/oasisprotocol/oasis-core/go/common"
	"github.com/oasisprotocol/oasis-core/go/common/cbor"
	commonGrpc "github.com/oasisprotocol/oasis-core/go/common/grpc"
	"github.com/oasisprotocol/oasis-core/go/roothash/api/block"
	sdkClient "github.com/oasisprotocol/oasis-sdk/client-sdk/go/client"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/crypto/signature"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/modules/core"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

func closeOrLog(c io.Closer) {
	if err := c.Close(); err != nil {
		fmt.Printf("close: %v", err)
	}
}

type BlockTransactionSignerData struct {
	Index   int
	Address string
	Nonce   int
}

type BlockTransactionData struct {
	Index                   int
	Hash                    string
	SignerData              []*BlockTransactionSignerData
	RelatedAccountAddresses []string
}

type BlockData struct {
	Hash            string
	GasUsed         int64
	Size            int
	TransactionData []*BlockTransactionData
}

func downloadRound(ctx context.Context, rtClient sdkClient.RuntimeClient, round int64) (*block.Block, []*sdkClient.TransactionWithResults, error) {
	b, err := rtClient.GetBlock(ctx, uint64(round))
	if err != nil {
		return nil, nil, fmt.Errorf("get block: %w", err)
	}
	txrs, err := rtClient.GetTransactionsWithResults(ctx, uint64(round))
	if err != nil {
		return nil, nil, fmt.Errorf("get transactions with results: %w", err)
	}
	return b, txrs, nil
}

func extractRound(sigContext signature.Context, b *block.Block, txrs []*sdkClient.TransactionWithResults) (*BlockData, error) {
	var blockData BlockData
	blockData.Hash = b.Header.EncodedHash().String()
	blockData.TransactionData = make([]*BlockTransactionData, 0, len(txrs))
	for i, txr := range txrs {
		// fmt.Printf("%#v\n", txr)
		var blockTransactionData BlockTransactionData
		blockTransactionData.Index = i
		blockTransactionData.Hash = txr.Tx.Hash().Hex()
		blockTransactionData.RelatedAccountAddresses = make([]string, 0, 2)
		tx, err := txr.Tx.Verify(sigContext)
		if err != nil {
			err = fmt.Errorf("tx %d verify: %w", i, err)
			fmt.Println(err)
			tx = nil
		}
		if tx != nil {
			blockTransactionData.SignerData = make([]*BlockTransactionSignerData, 0, len(tx.AuthInfo.SignerInfo))
			for j, si := range tx.AuthInfo.SignerInfo {
				var blockTransactionSignerData BlockTransactionSignerData
				blockTransactionSignerData.Index = j
				addr, err1 := si.AddressSpec.Address()
				if err1 != nil {
					return nil, fmt.Errorf("tx %d signer %d derive address: %w", i, j, err1)
				}
				addrText, err1 := addr.MarshalText()
				if err1 != nil {
					return nil, fmt.Errorf("tx %d signer %d address marshal text: %w", i, j, err1)
				}
				addrTextString := string(addrText)
				blockTransactionSignerData.Address = addrTextString
				blockTransactionSignerData.Nonce = int(si.Nonce)
				blockTransactionData.SignerData = append(blockTransactionData.SignerData, &blockTransactionSignerData)
				blockTransactionData.RelatedAccountAddresses = append(blockTransactionData.RelatedAccountAddresses, addrTextString)
			}
		}
		blockData.TransactionData = append(blockData.TransactionData, &blockTransactionData)
		var txGasUsed int64
		foundGasUsedEvent := false
		for j, event := range txr.Events {
			// fmt.Printf("%#v\n", event)
			coreEvents, err1 := core.DecodeEvent(event)
			if err1 != nil {
				return nil, fmt.Errorf("tx %d event %d decode: %w", i, j, err1)
			}
			for k, coreEvent := range coreEvents {
				coreEventCast, ok := coreEvent.(*core.Event)
				if !ok {
					return nil, fmt.Errorf("tx %d event %d decoded event %d could not cast to core.Event", i, j, k)
				}
				if coreEventCast.GasUsed != nil {
					if foundGasUsedEvent {
						return nil, fmt.Errorf("tx %d multiple gas used events", i)
					}
					foundGasUsedEvent = true
					txGasUsed = int64(coreEventCast.GasUsed.Amount)
				}
			}
		}
		if !foundGasUsedEvent {
			if (txr.Result.IsSuccess() || txr.Result.IsUnknown()) && tx != nil {
				// Treat as if it used all the gas.
				txGasUsed = int64(tx.AuthInfo.Fee.Gas)
			} else {
				// Inaccurate: Treat as not using any gas.
			}
		}
		// fmt.Printf("gas used: %d\n", txGasUsed)
		blockData.GasUsed += txGasUsed
		// Inaccurate: Re-serialize signed tx to estimate original size.
		txSize := len(cbor.Marshal(txr.Tx))
		// fmt.Printf("tx size: %d\n", txSize)
		blockData.Size += txSize
	}
	return &blockData, nil
}

func saveRound(ctx context.Context, dbTx pgx.Tx, chainAlias string, round int64, blockData *BlockData) error {
	var batch pgx.Batch
	for _, transactionData := range blockData.TransactionData {
		for _, signerData := range transactionData.SignerData {
			batch.Queue("INSERT INTO transaction_signer (chain_alias, height, tx_index, signer_index, addr, nonce) VALUES ($1, $2, $3, $4, $5, $6)", chainAlias, round, transactionData.Index, signerData.Index, signerData.Address, signerData.Nonce)
		}
		for _, addr := range transactionData.RelatedAccountAddresses {
			batch.Queue("INSERT INTO related_transaction (chain_alias, account_address, tx_height, tx_index) VALUES ($1, $2, $3, $4)", chainAlias, addr, round, transactionData.Index)
		}
		batch.Queue("INSERT INTO transaction_extra (chain_alias, height, tx_index, tx_hash) VALUES ($1, $2, $3, $4)", chainAlias, round, transactionData.Index, transactionData.Hash)
	}
	batch.Queue("INSERT INTO block_extra (chain_alias, height, b_hash, gas_used, size) VALUES ($1, $2, $3, $4, $5)", chainAlias, round, blockData.Hash, blockData.GasUsed, blockData.Size)
	batch.Queue("UPDATE progress SET first_unscanned_height = $1 WHERE chain_alias = $2", round+1, chainAlias)
	batchResults := dbTx.SendBatch(ctx, &batch)
	defer closeOrLog(batchResults)
	for i := 0; i < batch.Len(); i++ {
		if _, err := batchResults.Exec(); err != nil {
			// We lose info about what query went wrong ):.
			return err
		}
	}
	return nil
}

func scanRound(ctx context.Context, dbConn *pgx.Conn, chainAlias string, rtClient sdkClient.RuntimeClient, sigContext signature.Context, round int64) error {
	fmt.Printf("scanning round %d\n", round)
	b, txrs, err := downloadRound(ctx, rtClient, round)
	if err != nil {
		return fmt.Errorf("download: %w", err)
	}
	blockData, err := extractRound(sigContext, b, txrs)
	if err != nil {
		return fmt.Errorf("extract: %w", err)
	}
	if err = dbConn.BeginFunc(ctx, func(tx pgx.Tx) error {
		if err1 := saveRound(ctx, tx, chainAlias, round, blockData); err1 != nil {
			return fmt.Errorf("save: %w", err1)
		}
		return nil
	}); err != nil {
		return err
	}
	return nil
}

func scanLoop(ctx context.Context, dbConn *pgx.Conn, chainAlias string, rc sdkClient.RuntimeClient, sigContext signature.Context) error {
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
	rc := sdkClient.New(conn, rtid)
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
