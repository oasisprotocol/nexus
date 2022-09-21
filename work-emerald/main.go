package main

import (
	"context"
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"os"

	"github.com/jackc/pgx/v4"
	ocCommon "github.com/oasisprotocol/oasis-core/go/common"
	"github.com/oasisprotocol/oasis-core/go/common/cbor"
	"github.com/oasisprotocol/oasis-core/go/common/crypto/address"
	ocGrpc "github.com/oasisprotocol/oasis-core/go/common/grpc"
	"github.com/oasisprotocol/oasis-core/go/roothash/api/block"
	sdkClient "github.com/oasisprotocol/oasis-sdk/client-sdk/go/client"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/crypto/signature"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/modules/accounts"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/modules/consensusaccounts"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/modules/core"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/modules/evm"
	sdkTypes "github.com/oasisprotocol/oasis-sdk/client-sdk/go/types"
	"golang.org/x/crypto/sha3"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"oasis-explorer-backend/common"
)

var TopicErc20Transfer []byte = keccak256([]byte("Transfer(address,address,uint256)"))
var TopicErc20Approval []byte = keccak256([]byte("Approval(address,address,uint256)"))
var EthAddrZeroRaw []byte = make([]byte, 20)

type BlockTransactionSignerData struct {
	Index   int
	Address string
	Nonce   int
}

type BlockTransactionData struct {
	Index                   int
	Hash                    string
	EthHash                 *string
	SignerData              []*BlockTransactionSignerData
	RelatedAccountAddresses map[string]bool
}

type AddressPreimageData struct {
	ContextIdentifier string
	ContextVersion    int
	Data              []byte
}

type BlockData struct {
	Hash             string
	NumTransactions  int
	GasUsed          int64
	Size             int
	TransactionData  []*BlockTransactionData
	AddressPreimages map[string]*AddressPreimageData
}

func keccak256(data []byte) []byte {
	h := sha3.NewLegacyKeccak256()
	h.Write(data)
	return h.Sum(nil)
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
	// todo: non-transaction events
	return b, txrs, nil
}

func extractAddressPreimage(as *sdkTypes.AddressSpec) (*AddressPreimageData, error) {
	// Adapted from oasis-sdk/client-sdk/go/types/transaction.go.
	var (
		ctx  address.Context
		data []byte
	)
	switch {
	case as.Signature != nil:
		spec := as.Signature
		switch {
		case spec.Ed25519 != nil:
			ctx = sdkTypes.AddressV0Ed25519Context
			data, _ = spec.Ed25519.MarshalBinary()
		case spec.Secp256k1Eth != nil:
			ctx = sdkTypes.AddressV0Secp256k1EthContext
			// Use a scheme such that we can compute Secp256k1 addresses from Ethereum
			// addresses as this makes things more interoperable.
			untaggedPk, _ := spec.Secp256k1Eth.MarshalBinaryUncompressedUntagged()
			data = keccak256(untaggedPk)[32-20:]
		case spec.Sr25519 != nil:
			ctx = sdkTypes.AddressV0Sr25519Context
			data, _ = spec.Sr25519.MarshalBinary()
		default:
			panic("address: unsupported public key type")
		}
	case as.Multisig != nil:
		config := as.Multisig
		ctx = sdkTypes.AddressV0MultisigContext
		data = cbor.Marshal(config)
	default:
		return nil, fmt.Errorf("malformed AddressSpec")
	}
	return &AddressPreimageData{
		ContextIdentifier: ctx.Identifier,
		ContextVersion:    int(ctx.Version),
		Data:              data,
	}, nil
}

func visitAddressSpec(addressPreimages map[string]*AddressPreimageData, as *sdkTypes.AddressSpec) (string, error) {
	addrAbstract, err := as.Address()
	if err != nil {
		return "", fmt.Errorf("derive adddress: %w", err)
	}
	addrBytes, err := addrAbstract.MarshalText()
	if err != nil {
		return "", fmt.Errorf("address marshal text: %w", err)
	}
	addr := string(addrBytes)

	if _, ok := addressPreimages[addr]; !ok {
		preimageData, err1 := extractAddressPreimage(as)
		if err1 != nil {
			return "", fmt.Errorf("extract address preimage: %w", err1)
		}
		addressPreimages[addr] = preimageData
	}

	return addr, nil
}

func visitEthereumAddress(addressPreimages map[string]*AddressPreimageData, addrRaw []byte) (string, error) {
	preimageData := &AddressPreimageData{
		ContextIdentifier: sdkTypes.AddressV0Secp256k1EthContext.Identifier,
		ContextVersion:    int(sdkTypes.AddressV0Secp256k1EthContext.Version),
		Data:              addrRaw,
	}
	addrAbstract := (sdkTypes.Address)(address.NewAddress(sdkTypes.AddressV0Secp256k1EthContext, addrRaw))
	addrBytes, err := addrAbstract.MarshalText()
	if err != nil {
		return "", fmt.Errorf("address marshal text: %w", err)
	}
	addr := string(addrBytes)
	addressPreimages[addr] = preimageData
	return addr, nil
}

func visitRelatedAddressAbstract(relatedAddresses map[string]bool, addrAbstract sdkTypes.Address) (string, error) {
	addrBytes, err := addrAbstract.MarshalText()
	if err != nil {
		return "", fmt.Errorf("address marshal text: %w", err)
	}
	addr := string(addrBytes)

	relatedAddresses[addr] = true

	return addr, nil
}

func extractRound(sigContext signature.Context, b *block.Block, txrs []*sdkClient.TransactionWithResults) (*BlockData, error) {
	var blockData BlockData
	blockData.Hash = b.Header.EncodedHash().String()
	blockData.NumTransactions = len(txrs)
	blockData.TransactionData = make([]*BlockTransactionData, 0, len(txrs))
	blockData.AddressPreimages = map[string]*AddressPreimageData{}
	for i, txr := range txrs {
		// fmt.Printf("%#v\n", txr)
		var blockTransactionData BlockTransactionData
		blockTransactionData.Index = i
		blockTransactionData.Hash = txr.Tx.Hash().Hex()
		if len(txr.Tx.AuthProofs) == 1 && txr.Tx.AuthProofs[0].Module == "evm.ethereum.v0" {
			ethHash := hex.EncodeToString(keccak256(txr.Tx.Body))
			blockTransactionData.EthHash = &ethHash
		}
		blockTransactionData.RelatedAccountAddresses = map[string]bool{}
		tx, err := common.VerifyUtx(sigContext, &txr.Tx)
		if err != nil {
			err = fmt.Errorf("tx %d: %w", i, err)
			fmt.Println(err)
			tx = nil
		}
		if tx != nil {
			blockTransactionData.SignerData = make([]*BlockTransactionSignerData, 0, len(tx.AuthInfo.SignerInfo))
			for j, si := range tx.AuthInfo.SignerInfo {
				var blockTransactionSignerData BlockTransactionSignerData
				blockTransactionSignerData.Index = j
				// todo: combine this into visitRelatedAddressSpec
				addr, err1 := visitAddressSpec(blockData.AddressPreimages, &si.AddressSpec)
				if err1 != nil {
					return nil, fmt.Errorf("tx %d signer %d visit address spec: %w", i, j, err1)
				}
				blockTransactionSignerData.Address = addr
				blockTransactionSignerData.Nonce = int(si.Nonce)
				blockTransactionData.SignerData = append(blockTransactionData.SignerData, &blockTransactionSignerData)
				blockTransactionData.RelatedAccountAddresses[addr] = true
			}
		}
		var txGasUsed int64
		foundGasUsedEvent := false
		for j, event := range txr.Events {
			// fmt.Printf("%#v\n", event)
			// core
			coreEvents, err1 := core.DecodeEvent(event)
			if err1 != nil {
				return nil, fmt.Errorf("tx %d event %d decode core: %w", i, j, err1)
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
			// accounts
			accountEvents, err1 := accounts.DecodeEvent(event)
			if err1 != nil {
				return nil, fmt.Errorf("tx %d event %d decode accounts: %w", i, j, err1)
			}
			for k, accountEvent := range accountEvents {
				accountEventCast, ok := accountEvent.(*accounts.Event)
				if !ok {
					return nil, fmt.Errorf("tx %d event %d decoded event %d could not cast to accounts.Event", i, j, k)
				}
				if accountEventCast.Transfer != nil {
					if _, err1 = visitRelatedAddressAbstract(blockTransactionData.RelatedAccountAddresses, accountEventCast.Transfer.From); err1 != nil {
						return nil, fmt.Errorf("tx %d event %d decoded event %d from: %w", i, j, k, err1)
					}
					if _, err1 = visitRelatedAddressAbstract(blockTransactionData.RelatedAccountAddresses, accountEventCast.Transfer.To); err1 != nil {
						return nil, fmt.Errorf("tx %d event %d decoded event %d to: %w", i, j, k, err1)
					}
				}
				if accountEventCast.Burn != nil {
					if _, err1 = visitRelatedAddressAbstract(blockTransactionData.RelatedAccountAddresses, accountEventCast.Burn.Owner); err1 != nil {
						return nil, fmt.Errorf("tx %d event %d decoded event %d owner: %w", i, j, k, err1)
					}
				}
				if accountEventCast.Mint != nil {
					if _, err1 = visitRelatedAddressAbstract(blockTransactionData.RelatedAccountAddresses, accountEventCast.Mint.Owner); err1 != nil {
						return nil, fmt.Errorf("tx %d event %d decoded event %d owner: %w", i, j, k, err1)
					}
				}
			}
			// consensus accounts
			consensusaccountsEvents, err1 := consensusaccounts.DecodeEvent(event)
			if err1 != nil {
				return nil, fmt.Errorf("tx %d event %d decode consensusaccounts: %w", i, j, err1)
			}
			for k, consensusaccountsEvent := range consensusaccountsEvents {
				consensusaccountsEventCast, ok := consensusaccountsEvent.(*consensusaccounts.Event)
				if !ok {
					return nil, fmt.Errorf("tx %d event %d decoded event %d could not cast to consensusaccounts.Event", i, j, k)
				}
				if consensusaccountsEventCast.Deposit != nil {
					// .From is from another chain, so exclude?
					if _, err1 = visitRelatedAddressAbstract(blockTransactionData.RelatedAccountAddresses, consensusaccountsEventCast.Deposit.To); err1 != nil {
						return nil, fmt.Errorf("tx %d event %d decoded event %d from: %w", i, j, k, err1)
					}
				}
				if consensusaccountsEventCast.Withdraw != nil {
					if _, err1 = visitRelatedAddressAbstract(blockTransactionData.RelatedAccountAddresses, consensusaccountsEventCast.Withdraw.From); err1 != nil {
						return nil, fmt.Errorf("tx %d event %d decoded event %d from: %w", i, j, k, err1)
					}
					// .To is from another chain, so exclude?
				}
			}
			// evm
			evmEvents, err1 := evm.DecodeEvent(event)
			if err1 != nil {
				return nil, fmt.Errorf("tx %d event %d decode evm: %w", i, j, err1)
			}
			for k, evmEvent := range evmEvents {
				evmEventCast, ok := evmEvent.(*evm.Event)
				if !ok {
					return nil, fmt.Errorf("tx %d event %d decoded event %d could not cast to evm.Event", i, j, k)
				}
				if len(evmEventCast.Topics) >= 1 {
					// todo: can you really switch on byte slice?
					switch evmEventCast.Topics[0] {
					case TopicErc20Transfer:
						if len(evmEventCast.Topics) == 3 {
							fromAddrRaw := evmEventCast.Topics[1][32-20:]
							// todo: do what I mean
							if fromAddrRaw != EthAddrZeroRaw {
								// todo: combine these into visitRelatedEthereumAddress
								fromAddr, err2 := visitEthereumAddress(blockData.AddressPreimages, fromAddrRaw)
								if err2 != nil {
									return nil, fmt.Errorf("tx %d event %d decoded event %d from: %w", i, j, k, err2)
								}
								blockTransactionData.RelatedAccountAddresses[fromAddr] = true
							}
							// todo: to
						}
					}
				}
				fmt.Printf("event\naddress %s\ndata %s\n", hex.EncodeToString(evmEventCast.Address), hex.EncodeToString(evmEventCast.Data))
				for _, topic := range evmEventCast.Topics {
					fmt.Printf("topic %s\n", hex.EncodeToString(topic))
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
		blockData.TransactionData = append(blockData.TransactionData, &blockTransactionData)
		blockData.GasUsed += txGasUsed
		// Inaccurate: Re-serialize signed tx to estimate original size.
		txSize := len(cbor.Marshal(txr.Tx))
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
		for addr := range transactionData.RelatedAccountAddresses {
			batch.Queue("INSERT INTO related_transaction (chain_alias, account_address, tx_height, tx_index) VALUES ($1, $2, $3, $4)", chainAlias, addr, round, transactionData.Index)
		}
		batch.Queue("INSERT INTO transaction_extra (chain_alias, height, tx_index, tx_hash, eth_hash) VALUES ($1, $2, $3, $4, $5)", chainAlias, round, transactionData.Index, transactionData.Hash, transactionData.EthHash)
	}
	for addr, preimageData := range blockData.AddressPreimages {
		batch.Queue("INSERT INTO address_preimage (address, context_identifier, context_version, addr_data) VALUES ($1, $2, $3, $4) ON CONFLICT DO NOTHING", addr, preimageData.ContextIdentifier, preimageData.ContextVersion, preimageData.Data)
	}
	batch.Queue("INSERT INTO block_extra (chain_alias, height, b_hash, num_transactions, gas_used, size) VALUES ($1, $2, $3, $4, $5, $6)", chainAlias, round, blockData.Hash, blockData.NumTransactions, blockData.GasUsed, blockData.Size)
	batch.Queue("UPDATE progress SET first_unscanned_height = $1 WHERE chain_alias = $2", round+1, chainAlias)
	batchResults := dbTx.SendBatch(ctx, &batch)
	defer common.CloseOrLog(batchResults)
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

func scanLoop(ctx context.Context, dbConn *pgx.Conn, chainAlias string, rtClient sdkClient.RuntimeClient, sigContext signature.Context) error {
	var firstUnscannedHeight int64
	if err := dbConn.QueryRow(ctx, "SELECT first_unscanned_height FROM progress WHERE chain_alias = $1", chainAlias).Scan(&firstUnscannedHeight); err != nil {
		return err
	}
	for round := firstUnscannedHeight; ; round++ {
		if err := scanRound(ctx, dbConn, chainAlias, rtClient, sigContext, round); err != nil {
			return fmt.Errorf("round %d: %w", round, err)
		}
	}
}

func mainFallible(ctx context.Context) error {
	dbConn, err := pgx.Connect(ctx, "postgres://postgres:a@172.17.0.2/explorer")
	if err != nil {
		return err
	}
	conn, err := ocGrpc.Dial("grpc.oasis.dev:443", grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{})))
	if err != nil {
		return err
	}
	chainAlias := "mainnet_emerald"
	var rtid ocCommon.Namespace
	if err = rtid.UnmarshalHex("000000000000000000000000000000000000000000000000e2eaa99fc008f87f"); err != nil {
		return err
	}
	rtClient := sdkClient.New(conn, rtid)
	sigContext := signature.DeriveChainContext(rtid, "b11b369e0da5bb230b220127f5e7b242d385ef8c6f54906243f30af63c815535")
	if err = scanLoop(ctx, dbConn, chainAlias, rtClient, sigContext); err != nil {
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
