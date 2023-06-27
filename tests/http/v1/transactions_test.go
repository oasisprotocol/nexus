package v1

import (
	"fmt"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/oasisprotocol/nexus/common"
	storage "github.com/oasisprotocol/nexus/storage/client"
	"github.com/oasisprotocol/nexus/tests"
)

func makeTestBlocks(t *testing.T) []storage.Block {
	blocks := []storage.Block{
		{
			Hash:   "0a29ac21fa69bb9e43e5cb25d10826ff3946f1ce977e82f99a2614206a50765c",
			Height: tests.GenesisHeight,
		},
		{
			Hash:   "21d4e61a4318d66a2b5cb0e6869604fd5e5d1fcf3d0778d9b229046f2f1ff490",
			Height: tests.GenesisHeight + 1,
		},
		{
			Hash:   "339e93ba66b7e9268cb1e447a970df77fd37d05bf856c60ae853aa8dbf840483",
			Height: tests.GenesisHeight + 2,
		},
		{
			Hash:   "a65b5685e02375f7f155f7835d81024b8ce90430b7c98d114081bb1104b4c9ed",
			Height: tests.GenesisHeight + 3,
		},
		{
			Hash:   "703fdac6dc2508c28415e593a932492ee59befbe26ce648235f8d1b3325c5756",
			Height: tests.GenesisHeight + 4,
		},
		{
			Hash:   "0e570ffb7db0ab62a21367a709047a942959f00fe5e0c5d00a13e10058c8ee36",
			Height: tests.GenesisHeight + 5,
		},
	}
	timestamps := []string{
		"2022-04-11T09:30:00Z",
		"2022-04-11T11:00:08Z",
		"2022-04-11T11:00:27Z",
		"2022-04-11T11:00:33Z",
		"2022-04-11T11:00:39Z",
		"2022-04-11T11:00:45Z",
	}
	require.Equal(t, len(blocks), len(timestamps))

	for i := 0; i < len(blocks); i++ {
		timestamp, err := time.Parse(time.RFC3339, timestamps[i])
		require.Nil(t, err)
		blocks[i].Timestamp = timestamp
	}
	return blocks
}

func makeTestTransactions() []storage.Transaction {
	transactions := []storage.Transaction{
		{
			Block:   8048959,
			Hash:    "c58a618242396f2f5c7fa7b9c110c02e23e1d5c132085e72755605d938251ce0",
			Nonce:   4209,
			Fee:     common.NewBigInt(0),
			Method:  "registry.RegisterNode",
			Success: true,
		},
		{
			Block:   8048959,
			Hash:    "79f70f2d318043529b485ce171880bf16bd3fe0f59caf673336ab70c7f65e938",
			Nonce:   13420,
			Fee:     common.NewBigInt(0),
			Method:  "registry.RegisterNode",
			Success: true,
		},
		{
			Block:   8048959,
			Hash:    "35f5f7b4f906c1ea2e57bb9989205ef14daab012fe675c0bf4d93be23bd7473a",
			Nonce:   5719,
			Fee:     common.NewBigInt(0),
			Method:  "registry.RegisterNode",
			Success: true,
		},
		{
			Block:   8048959,
			Hash:    "fe36a8bf7e18e75bb9652d58a0a1b902fd8d5753a8542b6b69e1ae383fe7e64f",
			Nonce:   2415,
			Fee:     common.NewBigInt(0),
			Method:  "registry.RegisterNode",
			Success: true,
		},
		{
			Block:   8048959,
			Hash:    "ad3cc19d7155084eb689b80b4f45d0e32af752049d8f85316965e476b7732264",
			Nonce:   13420,
			Fee:     common.NewBigInt(0),
			Method:  "registry.RegisterNode",
			Success: true,
		},
	}

	// Get a consistent ordering.
	sort.Slice(transactions, func(i, j int) bool {
		return transactions[i].Hash < transactions[j].Hash
	})
	return transactions
}

func TestListBlocks(t *testing.T) {
	tests.SkipUnlessE2E(t)

	tests.Init()

	testBlocks := makeTestBlocks(t)
	endHeight := tests.GenesisHeight + int64(len(testBlocks)-1)
	<-tests.After(endHeight)

	var list storage.BlockList
	err := tests.GetFrom(fmt.Sprintf("/consensus/blocks?from=%d&to=%d", tests.GenesisHeight, endHeight), &list)
	require.Nil(t, err)
	require.Equal(t, len(testBlocks), len(list.Blocks))

	for i, block := range list.Blocks {
		require.Equal(t, testBlocks[i], block)
	}
}

func TestGetBlock(t *testing.T) {
	tests.SkipUnlessE2E(t)

	tests.Init()

	testBlocks := makeTestBlocks(t)
	endHeight := tests.GenesisHeight + int64(len(testBlocks)-1)
	<-tests.After(endHeight)

	var block storage.Block
	err := tests.GetFrom(fmt.Sprintf("/consensus/blocks/%d", endHeight), &block)
	require.Nil(t, err)
	require.Equal(t, testBlocks[len(testBlocks)-1], block)
}

func TestListTransactions(t *testing.T) {
	tests.SkipUnlessE2E(t)

	tests.Init()

	testTransactions := makeTestTransactions()
	endHeight := tests.GenesisHeight + int64(len(testTransactions)-1)
	<-tests.After(endHeight)

	var list storage.TransactionList
	err := tests.GetFrom(fmt.Sprintf("/consensus/transactions?block=%d", testTransactions[0].Block), &list)
	require.Nil(t, err)
	require.Equal(t, len(testTransactions), len(list.Transactions))

	// Get a consistent ordering.
	sort.Slice(list.Transactions, func(i, j int) bool {
		return list.Transactions[i].Hash < list.Transactions[j].Hash
	})
	for i, transaction := range list.Transactions {
		require.NotNil(t, transaction.Body)
		transaction.Body = nil
		require.Equal(t, testTransactions[i], transaction)
	}
}

func TestGetTransaction(t *testing.T) {
	tests.SkipUnlessE2E(t)

	tests.Init()

	testTransactions := makeTestTransactions()
	endHeight := tests.GenesisHeight + int64(len(testTransactions)-1)
	<-tests.After(endHeight)

	var transaction storage.Transaction
	err := tests.GetFrom(fmt.Sprintf("/consensus/transactions/%s", testTransactions[0].Hash), &transaction)
	require.Nil(t, err)
	require.NotNil(t, transaction.Body)
	transaction.Body = nil
	require.Equal(t, testTransactions[0], transaction)
}
