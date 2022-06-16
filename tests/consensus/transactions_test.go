package consensus

import (
	"fmt"
	"sort"
	"testing"

	v1 "github.com/oasislabs/oasis-indexer/api/v1"
	"github.com/oasislabs/oasis-indexer/tests"
	"github.com/stretchr/testify/require"
)

type TestTransaction struct {
	Height  int64
	Hash    string
	Nonce   uint64
	Fee     uint64
	Method  string
	Success bool
}

func makeTestTransactions(t *testing.T) []TestTransaction {
	heights := []int64{8048959, 8048959, 8048959, 8048959, 8048959}
	hashes := []string{
		"c58a618242396f2f5c7fa7b9c110c02e23e1d5c132085e72755605d938251ce0",
		"79f70f2d318043529b485ce171880bf16bd3fe0f59caf673336ab70c7f65e938",
		"35f5f7b4f906c1ea2e57bb9989205ef14daab012fe675c0bf4d93be23bd7473a",
		"fe36a8bf7e18e75bb9652d58a0a1b902fd8d5753a8542b6b69e1ae383fe7e64f",
		"ad3cc19d7155084eb689b80b4f45d0e32af752049d8f85316965e476b7732264",
	}
	nonces := []uint64{4209, 13420, 5719, 2415, 13420}
	fees := []uint64{0, 0, 0, 0, 0}
	methods := []string{
		"registry.RegisterNode",
		"registry.RegisterNode",
		"registry.RegisterNode",
		"registry.RegisterNode",
		"registry.RegisterNode",
	}
	successes := []bool{true, true, true, true, true}
	require.Equal(t, len(heights), len(hashes))
	require.Equal(t, len(hashes), len(nonces))
	require.Equal(t, len(nonces), len(fees))
	require.Equal(t, len(fees), len(methods))
	require.Equal(t, len(methods), len(successes))

	transactions := make([]TestTransaction, 0, len(hashes))
	for i := 0; i < len(hashes); i++ {
		transactions = append(transactions, TestTransaction{
			Height:  heights[i],
			Hash:    hashes[i],
			Nonce:   nonces[i],
			Fee:     fees[i],
			Method:  methods[i],
			Success: successes[i],
		})
	}

	// Get a consistent ordering.
	sort.Slice(transactions, func(i, j int) bool {
		return transactions[i].Hash < transactions[j].Hash
	})
	return transactions
}

func TestListTransactions(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping testing in short mode")
	}

	tests.Init()

	testTransactions := makeTestTransactions(t)
	endHeight := tests.GenesisHeight + int64(len(testTransactions)-1)
	<-tests.After(endHeight)

	var list v1.TransactionList
	tests.GetFrom(fmt.Sprintf("/consensus/transactions?block=%d", 8048959), &list)
	require.Equal(t, len(testTransactions), len(list.Transactions))

	// Get a consistent ordering.
	sort.Slice(list.Transactions, func(i, j int) bool {
		return list.Transactions[i].Hash < list.Transactions[j].Hash
	})
	for i, transaction := range list.Transactions {
		require.Equal(t, testTransactions[i].Height, transaction.Height)
		require.Equal(t, testTransactions[i].Hash, transaction.Hash)
		require.Equal(t, testTransactions[i].Nonce, transaction.Nonce)
		require.Equal(t, testTransactions[i].Fee, transaction.Fee)
		require.Equal(t, testTransactions[i].Method, transaction.Method)
		require.Equal(t, testTransactions[i].Success, transaction.Success)
	}
}

func TestGetTransaction(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping testing in short mode")
	}

	tests.Init()

	testTransactions := makeTestTransactions(t)
	endHeight := tests.GenesisHeight + int64(len(testTransactions)-1)
	<-tests.After(endHeight)

	var transaction v1.Transaction
	txHash := "d048495891e86f7588e7cbcfcc706aefd20da55de3bd2a8a53b153c5575c8b73"
	tests.GetFrom(fmt.Sprintf("/consensus/transactions/%s", txHash), &transaction)
	require.Equal(t, int64(8048960), transaction.Height)
	require.Equal(t, txHash, transaction.Hash)
	require.Equal(t, uint64(0), transaction.Nonce)
	require.Equal(t, uint64(0), transaction.Fee)
	require.Equal(t, "registry.RegisterNode", transaction.Method)
	require.Equal(t, true, transaction.Success)
}
